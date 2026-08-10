// Package store is the retrieval engine: a read-only knowledge base opened from
// a baked SQLite snapshot, plus a writer used by the ingester to build one. It
// keeps the import graph light (velocity orm for SQLite, no heavy driver
// leaves) and runs hybrid retrieval: FTS5 keyword ranking merged with in-process
// cosine similarity over stored embeddings.
package store

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/velocitykode/velocity/orm"

	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
)

// rrfK is the reciprocal-rank-fusion constant (k0). Larger values flatten the
// contribution of high ranks, blending keyword and vector lists more evenly.
const rrfK = 60

// defaultLimit is used when a caller passes limit <= 0.
const defaultLimit = 5

// Store is a read-only handle to a baked knowledge-base snapshot.
//
// All entries are loaded into memory at Open so cosine similarity and full-card
// fetches are pure in-process work; the SQLite handle stays open only to serve
// FTS5 MATCH queries. Raw SQL runs through orm.Manager.Raw, which binds to the
// manager's own connection (the temp-file snapshot opened here), not the global
// orm Default() manager that orm.NewRawQuery would target.
type Store struct {
	emb      embed.Embedder
	db       *orm.Manager
	tmpDir   string
	manifest kb.Manifest
	entries  []kb.Entry    // ordered by id, decoded once at Open
	byID     map[int64]int // entry id -> index into entries
}

// Open opens a read-only knowledge base from the bytes of a baked snapshot. The
// embedder is used only to vectorise queries at search time; pass embed.Noop()
// for keyword-only retrieval. The snapshot bytes are written to a temp file
// (the sqlite driver opens a path, not bytes) that Close removes.
func Open(ctx context.Context, snapshot []byte, emb embed.Embedder) (*Store, error) {
	if len(snapshot) == 0 {
		return nil, fmt.Errorf("store: empty snapshot")
	}

	// Place the snapshot inside a dedicated temp directory rather than the
	// shared temp root: the sqlite driver tightens the permissions of the
	// file's parent directory at connect time, and we may not own the system
	// temp root. A private subdirectory is ours to chmod and remove.
	tmpDir, err := os.MkdirTemp("", "kb-snapshot-*")
	if err != nil {
		return nil, fmt.Errorf("store: create snapshot temp dir: %w", err)
	}
	tmpPath := filepath.Join(tmpDir, "snapshot.db")
	if err := os.WriteFile(tmpPath, snapshot, 0o600); err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("store: write snapshot temp file: %w", err)
	}

	db, err := orm.NewManagerWithContext(ctx, orm.ManagerConfig{
		Driver:   snapshotDriver,
		Database: tmpPath,
	})
	if err != nil {
		_ = os.RemoveAll(tmpDir)
		return nil, fmt.Errorf("store: open snapshot: %w", err)
	}

	s := &Store{
		emb:    emb,
		db:     db,
		tmpDir: tmpDir,
		byID:   make(map[int64]int),
	}

	if err := s.load(ctx); err != nil {
		_ = db.Shutdown(ctx)
		_ = os.RemoveAll(tmpDir)
		return nil, err
	}
	return s, nil
}

// load reads every entry and the manifest into memory. A failure here means the
// bytes were not a valid snapshot built by this package.
func (s *Store) load(ctx context.Context) error {
	rows, err := s.db.Raw(ctx, `SELECT id, kind, title, body, signature, package, ref, version,
deprecated, use_instead, tags, embedding FROM entries ORDER BY id`)
	if err != nil {
		return fmt.Errorf("store: not a valid snapshot: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			e          kb.Entry
			kind       string
			deprecated int
			tags       string
			emb        []byte
		)
		if err := rows.Scan(&e.ID, &kind, &e.Title, &e.Body, &e.Signature, &e.Package,
			&e.Ref, &e.Version, &deprecated, &e.UseInstead, &tags, &emb); err != nil {
			return fmt.Errorf("store: scan entry: %w", err)
		}
		e.Kind = kb.Kind(kind)
		e.Deprecated = deprecated != 0
		e.Tags = splitTags(tags)
		e.Embedding = decodeEmbedding(emb)
		s.byID[e.ID] = len(s.entries)
		s.entries = append(s.entries, e)
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("store: read entries: %w", err)
	}

	return s.loadManifest(ctx)
}

func (s *Store) loadManifest(ctx context.Context) error {
	rows, err := s.db.Raw(ctx, `SELECT json FROM manifest LIMIT 1`)
	if err != nil {
		return fmt.Errorf("store: read manifest: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return fmt.Errorf("store: scan manifest: %w", err)
		}
		if err := json.Unmarshal([]byte(raw), &s.manifest); err != nil {
			return fmt.Errorf("store: decode manifest: %w", err)
		}
	}
	return rows.Err()
}

// Close releases the underlying database handle and any temp file.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	var firstErr error
	if s.db != nil {
		if err := s.db.Shutdown(context.Background()); err != nil {
			firstErr = err
		}
		s.db = nil
	}
	if s.tmpDir != "" {
		if err := os.RemoveAll(s.tmpDir); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
		s.tmpDir = ""
	}
	return firstErr
}

// Search runs hybrid retrieval for an intent query: FTS5 keyword ranking fused
// with cosine similarity over embeddings (reciprocal-rank fusion). When the
// embedder is unavailable it returns keyword-only results. A zero kind searches
// all kinds; limit <= 0 uses a sensible default.
func (s *Store) Search(ctx context.Context, query string, kind kb.Kind, limit int) ([]kb.Result, error) {
	if limit <= 0 {
		limit = defaultLimit
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, nil
	}

	// Candidate pool is a few times the requested limit so fusion has room to
	// reorder before truncation.
	pool := limit * 4
	if pool < 20 {
		pool = 20
	}

	keywordIDs, err := s.keywordSearch(ctx, query, kind, pool)
	if err != nil {
		return nil, err
	}
	vectorIDs := s.vectorSearch(ctx, query, kind, pool)

	fused := fuse(keywordIDs, vectorIDs)
	if len(fused) > limit {
		fused = fused[:limit]
	}

	results := make([]kb.Result, 0, len(fused))
	for _, fr := range fused {
		idx, ok := s.byID[fr.id]
		if !ok {
			continue
		}
		results = append(results, kb.Result{Entry: s.entries[idx], Score: fr.score})
	}
	return results, nil
}

// keywordSearch runs the FTS5 MATCH and returns matching entry ids ordered best
// first (lowest bm25 score is most relevant).
func (s *Store) keywordSearch(ctx context.Context, query string, kind kb.Kind, n int) ([]int64, error) {
	match := buildMatch(query)
	if match == "" {
		return nil, nil
	}

	var (
		sql  string
		args []any
	)
	if kind == "" {
		sql = `SELECT entries_fts.rowid FROM entries_fts
WHERE entries_fts MATCH ? ORDER BY bm25(entries_fts) LIMIT ?`
		args = []any{match, n}
	} else {
		sql = `SELECT entries_fts.rowid FROM entries_fts
JOIN entries e ON e.id = entries_fts.rowid
WHERE entries_fts MATCH ? AND e.kind = ? ORDER BY bm25(entries_fts) LIMIT ?`
		args = []any{match, string(kind), n}
	}

	rows, err := s.db.Raw(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("store: keyword search: %w", err)
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan keyword hit: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: keyword search rows: %w", err)
	}
	return ids, nil
}

// vectorSearch embeds the query and ranks in-memory entries by cosine
// similarity. It returns nil (keyword-only) when the embedder is unavailable or
// any embedding error occurs, so a missing backend degrades gracefully.
func (s *Store) vectorSearch(ctx context.Context, query string, kind kb.Kind, n int) []int64 {
	if s.emb == nil {
		return nil
	}
	vecs, err := s.emb.Embed(ctx, []string{query})
	if err != nil || len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil
	}
	qv := vecs[0]

	type scored struct {
		id  int64
		sim float64
	}
	ranked := make([]scored, 0, len(s.entries))
	for _, e := range s.entries {
		if kind != "" && e.Kind != kind {
			continue
		}
		if len(e.Embedding) == 0 {
			continue
		}
		sim := cosine(qv, e.Embedding)
		if sim <= 0 {
			// No semantic overlap (includes a zero query vector). Skipping
			// keeps a signal-free query from surfacing arbitrary entries.
			continue
		}
		ranked = append(ranked, scored{id: e.ID, sim: sim})
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		return ranked[i].sim > ranked[j].sim
	})
	if len(ranked) > n {
		ranked = ranked[:n]
	}

	ids := make([]int64, len(ranked))
	for i, r := range ranked {
		ids[i] = r.id
	}
	return ids
}

// fusedResult pairs an entry id with its fused score.
type fusedResult struct {
	id    int64
	score float64
}

// fuse merges ranked id lists with reciprocal-rank fusion: each list contributes
// 1/(rrfK + rank) per id (rank is 0-based). Results are sorted by descending
// score with id as a stable tiebreak.
func fuse(lists ...[]int64) []fusedResult {
	scores := make(map[int64]float64)
	for _, list := range lists {
		for rank, id := range list {
			scores[id] += 1.0 / float64(rrfK+rank)
		}
	}

	out := make([]fusedResult, 0, len(scores))
	for id, sc := range scores {
		out = append(out, fusedResult{id: id, score: sc})
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].score != out[j].score {
			return out[i].score > out[j].score
		}
		return out[i].id < out[j].id
	})
	return out
}

// buildMatch turns a free-text query into a safe FTS5 MATCH string. Each term is
// double-quoted (escaping embedded quotes by doubling) so FTS5 operators in the
// raw query are treated as literal text, then given a trailing prefix wildcard
// and joined with OR. OR-with-prefix maximises recall for loose intent queries
// (a partial term overlap still surfaces); bm25 ranks entries matching more
// terms first, so precision is preserved at the top of the list.
func buildMatch(query string) string {
	fields := strings.Fields(query)
	terms := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.ReplaceAll(f, `"`, `""`)
		terms = append(terms, `"`+f+`"*`)
	}
	return strings.Join(terms, " OR ")
}

// Symbol returns exact symbol entries matching a name (anti-hallucination
// lookup). Match is case-insensitive and may cover qualified names such as
// "package.Title".
func (s *Store) Symbol(ctx context.Context, name string) ([]kb.Entry, error) {
	_ = ctx
	target := strings.ToLower(strings.TrimSpace(name))
	if target == "" {
		return nil, nil
	}

	var out []kb.Entry
	for _, e := range s.entries {
		if e.Kind != kb.KindSymbol {
			continue
		}
		title := strings.ToLower(e.Title)
		qualified := title
		if e.Package != "" {
			qualified = strings.ToLower(e.Package) + "." + title
		}
		// Methods are stored title-qualified as "Type.Method"; also match the
		// bare method segment so a lookup of "Dispatch" finds "Foo.Dispatch".
		method := title
		if idx := strings.LastIndex(title, "."); idx >= 0 {
			method = title[idx+1:]
		}
		if title == target || qualified == target || method == target {
			out = append(out, e)
		}
	}
	return out, nil
}

// Guards returns curated rule entries for a topic or package (the guarding
// surface). An empty topic returns all rules, highest-signal first (lowest id,
// which the ingester orders by priority). A non-empty topic matches
// case-insensitively against package, tags, or title.
func (s *Store) Guards(ctx context.Context, topic string) ([]kb.Entry, error) {
	_ = ctx
	needle := strings.ToLower(strings.TrimSpace(topic))

	var out []kb.Entry
	for _, e := range s.entries {
		if e.Kind != kb.KindRule {
			continue
		}
		if needle == "" || guardMatches(e, needle) {
			out = append(out, e)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].ID < out[j].ID
	})
	return out, nil
}

// guardMatches reports whether a rule entry is relevant to the needle. The
// needle is split into terms; a rule matches when ANY term appears (case
// insensitive substring) anywhere in its package, title, tags, or body. Body is
// included so a guard surfaces on vocabulary that lives in its explanation, and
// OR-over-terms keeps multi-word topics from over-constraining the few rules.
func guardMatches(e kb.Entry, needle string) bool {
	var haystack strings.Builder
	haystack.WriteString(strings.ToLower(e.Package))
	haystack.WriteByte(' ')
	haystack.WriteString(strings.ToLower(e.Title))
	haystack.WriteByte(' ')
	haystack.WriteString(strings.ToLower(e.Body))
	for _, t := range e.Tags {
		haystack.WriteByte(' ')
		haystack.WriteString(strings.ToLower(t))
	}
	hay := haystack.String()

	for _, term := range strings.Fields(needle) {
		if strings.Contains(hay, term) {
			return true
		}
	}
	return false
}

// Manifest reports what the open snapshot covers.
func (s *Store) Manifest(ctx context.Context) (kb.Manifest, error) {
	_ = ctx
	return s.manifest, nil
}

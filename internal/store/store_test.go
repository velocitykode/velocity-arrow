package store

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
)

func readFile(path string) ([]byte, error) { return os.ReadFile(path) }

// fakeEmbedder is a deterministic, network-free Embedder for tests. It maps a
// fixed vocabulary of tokens to one-hot dimensions so cosine similarity between
// a query and an entry reflects token overlap.
type fakeEmbedder struct {
	vocab map[string]int
	dim   int
	fail  bool // when true, Embed reports ErrUnavailable
}

func newFakeEmbedder(tokens ...string) *fakeEmbedder {
	v := make(map[string]int, len(tokens))
	for i, t := range tokens {
		v[t] = i
	}
	return &fakeEmbedder{vocab: v, dim: len(tokens)}
}

func (f *fakeEmbedder) Dimensions() int { return f.dim }

func (f *fakeEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if f.fail {
		return nil, embed.ErrUnavailable
	}
	out := make([][]float32, len(texts))
	for i, t := range texts {
		out[i] = f.vectorize(t)
	}
	return out, nil
}

func (f *fakeEmbedder) vectorize(text string) []float32 {
	vec := make([]float32, f.dim)
	for _, tok := range splitWords(text) {
		if idx, ok := f.vocab[tok]; ok {
			vec[idx] += 1
		}
	}
	return vec
}

// splitWords lowercases and splits on spaces, mirroring how the fake builds
// vectors from both queries and entry bodies in tests.
func splitWords(s string) []string {
	var out []string
	cur := ""
	for _, r := range s {
		if r == ' ' {
			if cur != "" {
				out = append(out, cur)
				cur = ""
			}
			continue
		}
		cur += lower(string(r))
	}
	if cur != "" {
		out = append(out, cur)
	}
	return out
}

func lower(s string) string {
	b := []rune(s)
	for i, r := range b {
		if r >= 'A' && r <= 'Z' {
			b[i] = r + ('a' - 'A')
		}
	}
	return string(b)
}

// sampleEntries returns a handful of entries across kinds, some with embeddings
// (built from the fake vocabulary) and some without.
func sampleEntries(f *fakeEmbedder) []kb.Entry {
	return []kb.Entry{
		{
			ID: 1, Kind: kb.KindSymbol, Title: "Hash", Package: "crypto",
			Body: "hash a password securely", Signature: "func Hash(s string) string",
			Ref: "crypto/hash.go:10", Version: "0.62.2",
			Tags: []string{"password", "security"}, Embedding: f.vectorize("hash password security"),
		},
		{
			ID: 2, Kind: kb.KindHelper, Title: "Check", Package: "crypto",
			Body: "verify a password against a hash", Ref: "crypto/hash.go:20",
			Version: "0.62.2", Tags: []string{"password"},
			Embedding: f.vectorize("verify password hash"),
		},
		{
			ID: 3, Kind: kb.KindRule, Title: "Never log secrets", Package: "log",
			Body: "do not write secrets to the log", Ref: "docs/security",
			Version: "0.62.2", Tags: []string{"security", "logging"},
			// no embedding on purpose
		},
		{
			ID: 4, Kind: kb.KindRule, Title: "Use constant-time compare", Package: "crypto",
			Body: "compare secrets with constant time equality", Ref: "docs/crypto",
			Version: "0.62.2", Tags: []string{"timing", "security"},
		},
		{
			ID: 5, Kind: kb.KindConcept, Title: "Pipeline", Package: "pipeline",
			Body: "compose stages over a payload", Ref: "pipeline/doc.go:1",
			Version: "0.62.2", Embedding: f.vectorize("pipeline stages compose"),
		},
	}
}

// buildSnapshot writes a real snapshot via the Writer and returns its bytes.
func buildSnapshot(t *testing.T, entries []kb.Entry, m kb.Manifest) []byte {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "snap.db")

	w, err := Create(ctx, path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	for _, e := range entries {
		if err := w.Insert(ctx, e); err != nil {
			t.Fatalf("Insert(%d): %v", e.ID, err)
		}
	}
	if err := w.Finalize(ctx, m); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("writer Close: %v", err)
	}

	data, err := readFile(path)
	if err != nil {
		t.Fatalf("read snapshot: %v", err)
	}
	return data
}

func sampleManifest() kb.Manifest {
	return kb.Manifest{
		VelocityVersion: "0.62.2",
		BuiltAt:         "2026-06-14T00:00:00Z",
		Packages:        []string{"crypto", "log", "pipeline"},
		Counts:          map[kb.Kind]int{kb.KindSymbol: 1, kb.KindHelper: 1, kb.KindRule: 2, kb.KindConcept: 1},
		Total:           5,
	}
}

func openSample(t *testing.T, emb embed.Embedder) (*Store, *fakeEmbedder) {
	t.Helper()
	f := newFakeEmbedder("hash", "password", "security", "verify", "pipeline", "stages", "compose")
	data := buildSnapshot(t, sampleEntries(f), sampleManifest())
	if emb == nil {
		emb = f
	}
	s, err := Open(context.Background(), data, emb)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, f
}

func TestSearchHybrid(t *testing.T) {
	s, _ := openSample(t, nil)
	ctx := context.Background()

	tests := []struct {
		name      string
		query     string
		kind      kb.Kind
		limit     int
		wantFirst int64
		wantKind  kb.Kind // when set, every result must match
		minLen    int
	}{
		{name: "password intent", query: "hash password", kind: "", limit: 5, wantFirst: 1, minLen: 1},
		{name: "kind filter helper", query: "password", kind: kb.KindHelper, limit: 5, wantKind: kb.KindHelper, minLen: 1},
		{name: "default limit applied", query: "password", kind: "", limit: 0, minLen: 1},
		{name: "concept", query: "pipeline stages", kind: "", limit: 3, wantFirst: 5, minLen: 1},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			res, err := s.Search(ctx, tc.query, tc.kind, tc.limit)
			if err != nil {
				t.Fatalf("Search: %v", err)
			}
			if len(res) < tc.minLen {
				t.Fatalf("want >=%d results, got %d", tc.minLen, len(res))
			}
			if tc.wantFirst != 0 && res[0].ID != tc.wantFirst {
				t.Fatalf("want first id %d, got %d (%+v)", tc.wantFirst, res[0].ID, res)
			}
			for _, r := range res {
				if tc.wantKind != "" && r.Kind != tc.wantKind {
					t.Fatalf("kind filter leaked: %s", r.Kind)
				}
				if r.Score <= 0 {
					t.Fatalf("expected positive fused score, got %v", r.Score)
				}
			}
		})
	}
}

func TestSearchKeywordOnly(t *testing.T) {
	// embed.Noop always reports ErrUnavailable, so retrieval is keyword-only.
	s, _ := openSample(t, embed.Noop())
	res, err := s.Search(context.Background(), "password", "", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("keyword-only search returned nothing")
	}
	found := false
	for _, r := range res {
		if r.ID == 1 || r.ID == 2 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected a password entry, got %+v", res)
	}
}

func TestSearchFailingEmbedderFallsBack(t *testing.T) {
	f := newFakeEmbedder("password")
	f.fail = true
	data := buildSnapshot(t, sampleEntries(newFakeEmbedder("hash", "password")), sampleManifest())
	s, err := Open(context.Background(), data, f)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	res, err := s.Search(context.Background(), "password", "", 5)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("expected keyword fallback results")
	}
}

func TestSearchEdgeCases(t *testing.T) {
	s, _ := openSample(t, nil)
	ctx := context.Background()

	if res, err := s.Search(ctx, "   ", "", 5); err != nil || res != nil {
		t.Fatalf("blank query: want nil/nil, got %v / %v", res, err)
	}
	// FTS5 operator characters must not break the query.
	if _, err := s.Search(ctx, `password AND OR NOT "x*" (y)`, "", 5); err != nil {
		t.Fatalf("operator-laden query errored: %v", err)
	}
	// A term with no matches anywhere yields zero results, no error.
	res, err := s.Search(ctx, "zzzznotacorpus", "", 5)
	if err != nil {
		t.Fatalf("miss query errored: %v", err)
	}
	if len(res) != 0 {
		t.Fatalf("expected no results, got %d", len(res))
	}
}

func TestSymbol(t *testing.T) {
	s, _ := openSample(t, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		input   string
		wantIDs []int64
	}{
		{name: "bare title case-insensitive", input: "hash", wantIDs: []int64{1}},
		{name: "qualified name", input: "crypto.Hash", wantIDs: []int64{1}},
		{name: "no match", input: "Nonexistent", wantIDs: nil},
		{name: "rule is not a symbol", input: "Never log secrets", wantIDs: nil},
		{name: "blank", input: "  ", wantIDs: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Symbol(ctx, tc.input)
			if err != nil {
				t.Fatalf("Symbol: %v", err)
			}
			gotIDs := ids(got)
			if !equalIDs(gotIDs, tc.wantIDs) {
				t.Fatalf("want %v, got %v", tc.wantIDs, gotIDs)
			}
		})
	}
}

func TestGuards(t *testing.T) {
	s, _ := openSample(t, nil)
	ctx := context.Background()

	tests := []struct {
		name    string
		topic   string
		wantIDs []int64
	}{
		{name: "all rules ordered by id", topic: "", wantIDs: []int64{3, 4}},
		{name: "by package", topic: "crypto", wantIDs: []int64{4}},
		{name: "by tag", topic: "logging", wantIDs: []int64{3}},
		{name: "by title substring", topic: "constant-time", wantIDs: []int64{4}},
		{name: "case insensitive", topic: "SECURITY", wantIDs: []int64{3, 4}},
		{name: "miss", topic: "nope", wantIDs: nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := s.Guards(ctx, tc.topic)
			if err != nil {
				t.Fatalf("Guards: %v", err)
			}
			if !equalIDs(ids(got), tc.wantIDs) {
				t.Fatalf("want %v, got %v", tc.wantIDs, ids(got))
			}
			for _, e := range got {
				if e.Kind != kb.KindRule {
					t.Fatalf("non-rule leaked: %s", e.Kind)
				}
			}
		})
	}
}

func TestManifestRoundTrip(t *testing.T) {
	s, _ := openSample(t, nil)
	got, err := s.Manifest(context.Background())
	if err != nil {
		t.Fatalf("Manifest: %v", err)
	}
	want := sampleManifest()
	if got.VelocityVersion != want.VelocityVersion || got.BuiltAt != want.BuiltAt || got.Total != want.Total {
		t.Fatalf("manifest scalars mismatch: %+v", got)
	}
	if len(got.Packages) != len(want.Packages) {
		t.Fatalf("packages mismatch: %+v", got.Packages)
	}
	for k, v := range want.Counts {
		if got.Counts[k] != v {
			t.Fatalf("count[%s] = %d, want %d", k, got.Counts[k], v)
		}
	}
}

func TestEntryRoundTrip(t *testing.T) {
	// Verify embedding + tags + flags survive write/read.
	f := newFakeEmbedder("a", "b", "c")
	in := kb.Entry{
		ID: 7, Kind: kb.KindSymbol, Title: "Old", Package: "legacy",
		Body: "x", Signature: "func Old()", Ref: "r", Version: "v",
		Deprecated: true, UseInstead: "New", Tags: []string{"one", "two"},
		Embedding: f.vectorize("a b c"),
	}
	data := buildSnapshot(t, []kb.Entry{in}, sampleManifest())
	s, err := Open(context.Background(), data, embed.Noop())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()

	got := s.entries[0]
	if got.ID != in.ID || got.Kind != in.Kind || got.Title != in.Title ||
		got.Package != in.Package || got.Signature != in.Signature ||
		!got.Deprecated || got.UseInstead != "New" {
		t.Fatalf("scalar round-trip mismatch: %+v", got)
	}
	if !equalStrings(got.Tags, in.Tags) {
		t.Fatalf("tags round-trip: %v", got.Tags)
	}
	if !equalFloats(got.Embedding, in.Embedding) {
		t.Fatalf("embedding round-trip: %v vs %v", got.Embedding, in.Embedding)
	}
}

func TestInsertAutoID(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "auto.db")
	w, err := Create(ctx, path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	// ID == 0 lets SQLite assign the rowid.
	if err := w.Insert(ctx, kb.Entry{Kind: kb.KindRule, Title: "auto", Body: "b"}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := w.Finalize(ctx, sampleManifest()); err != nil {
		t.Fatalf("Finalize: %v", err)
	}
	_ = w.Close()

	data, err := readFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	s, err := Open(ctx, data, embed.Noop())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	if len(s.entries) != 1 || s.entries[0].ID == 0 {
		t.Fatalf("auto id not assigned: %+v", s.entries)
	}
}

func TestOpenInvalidSnapshot(t *testing.T) {
	if _, err := Open(context.Background(), nil, embed.Noop()); err == nil {
		t.Fatal("expected error for empty snapshot")
	}
	if _, err := Open(context.Background(), []byte("not a sqlite db"), embed.Noop()); err == nil {
		t.Fatal("expected error for non-sqlite bytes")
	}
}

func TestCloseIdempotent(t *testing.T) {
	s, _ := openSample(t, nil)
	if err := s.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}
	var nilStore *Store
	if err := nilStore.Close(); err != nil {
		t.Fatalf("nil Close: %v", err)
	}
}

func TestWriterClosedErrors(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "w.db")
	w, err := Create(ctx, path)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := w.Insert(ctx, kb.Entry{ID: 1}); err == nil {
		t.Fatal("Insert after Close should error")
	}
	if err := w.Finalize(ctx, sampleManifest()); err == nil {
		t.Fatal("Finalize after Close should error")
	}
	if err := w.Close(); err != nil {
		t.Fatalf("idempotent writer Close: %v", err)
	}
}

func TestCodecHelpers(t *testing.T) {
	// Embedding codec round-trips and rejects malformed lengths.
	cases := [][]float32{nil, {}, {1.5}, {0, -1, 3.14159, 1e9}}
	for _, c := range cases {
		got := decodeEmbedding(encodeEmbedding(c))
		if len(c) == 0 {
			if got != nil {
				t.Fatalf("empty embedding decoded to %v", got)
			}
			continue
		}
		if !equalFloats(got, c) {
			t.Fatalf("embedding round-trip %v -> %v", c, got)
		}
	}
	if decodeEmbedding([]byte{1, 2, 3}) != nil {
		t.Fatal("non-multiple-of-4 should decode to nil")
	}

	// Tags codec.
	if splitTags("") != nil {
		t.Fatal("empty tags should be nil")
	}
	if joinTags([]string{"a", "b"}) != "a,b" {
		t.Fatal("joinTags")
	}

	// Cosine guards.
	if cosine(nil, nil) != 0 || cosine([]float32{1}, []float32{1, 2}) != 0 ||
		cosine([]float32{0, 0}, []float32{1, 1}) != 0 {
		t.Fatal("cosine guard failed")
	}
	if got := cosine([]float32{1, 0}, []float32{1, 0}); got < 0.999 {
		t.Fatalf("identical vectors cosine = %v", got)
	}
}

func TestBuildMatch(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{in: "", want: ""},
		{in: "hash password", want: `"hash"* OR "password"*`},
		{in: `a"b`, want: `"a""b"*`},
		{in: "  spaced   out ", want: `"spaced"* OR "out"*`},
	}
	for _, tc := range tests {
		if got := buildMatch(tc.in); got != tc.want {
			t.Fatalf("buildMatch(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// --- small test utilities ---

func ids(es []kb.Entry) []int64 {
	if len(es) == 0 {
		return nil
	}
	out := make([]int64, len(es))
	for i, e := range es {
		out[i] = e.ID
	}
	return out
}

func equalIDs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func equalFloats(a, b []float32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

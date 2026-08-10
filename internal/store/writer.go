package store

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/velocitykode/velocity/orm"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

// Writer builds a baked knowledge-base snapshot at a path. The ingester inserts
// entries, then Finalize builds the FTS5 index and writes the manifest row.
//
// Raw SQL runs through orm.Manager.Exec/Raw, which execute against the manager's
// own driver (the connection opened by Create). This is the orm seam for binding
// a raw statement to a specific manager: the package-level orm.NewRawQuery binds
// to the global Default() manager, which is not what we open here, so it is
// deliberately avoided.
type Writer struct {
	db   *orm.Manager
	path string
}

// Create creates (or truncates) a snapshot database at path and returns a Writer
// with the schema applied. path must be an absolute or slash-containing path so
// the sqlite driver opens it verbatim rather than relocating it under database/.
func Create(ctx context.Context, path string) (*Writer, error) {
	if err := truncate(path); err != nil {
		return nil, fmt.Errorf("store: prepare snapshot file: %w", err)
	}

	db, err := orm.NewManagerWithContext(ctx, orm.ManagerConfig{
		Driver:   snapshotDriver,
		Database: path,
	})
	if err != nil {
		return nil, fmt.Errorf("store: open snapshot for write: %w", err)
	}

	w := &Writer{db: db, path: path}
	if err := w.applySchema(ctx); err != nil {
		_ = db.Shutdown(ctx)
		return nil, err
	}
	return w, nil
}

func (w *Writer) applySchema(ctx context.Context) error {
	for _, stmt := range schemaStatements {
		if _, err := w.db.Exec(ctx, stmt); err != nil {
			return fmt.Errorf("store: apply schema: %w", err)
		}
	}
	return nil
}

// schemaStatements is the snapshot schema. entries_fts is an external-content
// FTS5 index over entries; it is populated by a rebuild in Finalize rather than
// per-insert triggers so a bulk build stays cheap.
var schemaStatements = []string{
	`CREATE TABLE entries(
  id INTEGER PRIMARY KEY, kind TEXT, title TEXT, body TEXT, signature TEXT,
  package TEXT, ref TEXT, version TEXT, deprecated INTEGER, use_instead TEXT,
  tags TEXT,
  embedding BLOB
);`,
	`CREATE VIRTUAL TABLE entries_fts USING fts5(title, body, tags, content='entries', content_rowid='id');`,
	`CREATE TABLE manifest(json TEXT);`,
}

// Insert writes one entry, including its embedding blob when present.
func (w *Writer) Insert(ctx context.Context, e kb.Entry) error {
	if w.db == nil {
		return fmt.Errorf("store: writer is closed")
	}

	deprecated := 0
	if e.Deprecated {
		deprecated = 1
	}

	const q = `INSERT INTO entries
(id, kind, title, body, signature, package, ref, version, deprecated, use_instead, tags, embedding)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	args := []any{
		e.ID,
		string(e.Kind),
		e.Title,
		e.Body,
		e.Signature,
		e.Package,
		e.Ref,
		e.Version,
		deprecated,
		e.UseInstead,
		joinTags(e.Tags),
		encodeEmbedding(e.Embedding),
	}

	if e.ID == 0 {
		// Let SQLite assign the rowid when the caller leaves ID zero.
		const qAuto = `INSERT INTO entries
(kind, title, body, signature, package, ref, version, deprecated, use_instead, tags, embedding)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
		if _, err := w.db.Exec(ctx, qAuto, args[1:]...); err != nil {
			return fmt.Errorf("store: insert entry: %w", err)
		}
		return nil
	}

	if _, err := w.db.Exec(ctx, q, args...); err != nil {
		return fmt.Errorf("store: insert entry: %w", err)
	}
	return nil
}

// Finalize builds the FTS5 index and stores the manifest. Call once after all
// inserts, before Close.
func (w *Writer) Finalize(ctx context.Context, m kb.Manifest) error {
	if w.db == nil {
		return fmt.Errorf("store: writer is closed")
	}

	if _, err := w.db.Exec(ctx, `INSERT INTO entries_fts(entries_fts) VALUES('rebuild');`); err != nil {
		return fmt.Errorf("store: rebuild fts index: %w", err)
	}

	blob, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("store: marshal manifest: %w", err)
	}
	if _, err := w.db.Exec(ctx, `INSERT INTO manifest(json) VALUES (?)`, string(blob)); err != nil {
		return fmt.Errorf("store: write manifest: %w", err)
	}
	return nil
}

// Close flushes and closes the database.
func (w *Writer) Close() error {
	if w.db == nil {
		return nil
	}
	err := w.db.Shutdown(context.Background())
	w.db = nil
	return err
}

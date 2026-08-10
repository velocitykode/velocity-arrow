// Command ingest builds the baked knowledge-base snapshot that arrow embeds: it
// parses exact symbols from the velocity source tree, loads curated guard rules
// and recipes, embeds each entry, and writes internal/kb/data/velocity-kb.db.
// Run it when velocity bumps or the rules change, then commit the regenerated
// snapshot.
//
// Usage:
//
//	go run ./cmd/ingest -velocity ~/code/velocity -version v0.72.0
//
// Embeddings require a configured backend (see internal/embed). With none, the
// snapshot is built keyword-only and the server still serves FTS5 results.
package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/velocitykode/velocity-arrow/internal/corpus"
	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
)

const embedBatch = 64

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "ingest: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	defaultRoot := os.Getenv("VELOCITY_SRC")
	if defaultRoot == "" {
		if home, err := os.UserHomeDir(); err == nil {
			defaultRoot = filepath.Join(home, "code", "velocity")
		}
	}
	root := flag.String("velocity", defaultRoot, "path to the velocity source tree")
	version := flag.String("version", "dev", "velocity version stamp for entries and manifest")
	out := flag.String("out", filepath.Join("internal", "kb", "data", "velocity-kb.db"), "output snapshot path")
	flag.Parse()

	ctx := context.Background()

	rulesDir, err := fs.Sub(kb.RulesFS, "rules")
	if err != nil {
		return fmt.Errorf("open rules: %w", err)
	}

	var entries []kb.Entry

	syms, err := corpus.Symbols(ctx, *root, *version)
	if err != nil {
		return fmt.Errorf("symbols: %w", err)
	}
	entries = append(entries, syms...)

	curated, err := corpus.Markdown(rulesDir, *version)
	if err != nil {
		return fmt.Errorf("markdown: %w", err)
	}
	entries = append(entries, curated...)

	if len(entries) == 0 {
		return fmt.Errorf("no entries gathered; check -velocity path %q", *root)
	}

	embedEntries(ctx, embed.New(), entries)

	w, err := store.Create(ctx, *out)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	defer w.Close()

	manifest := kb.Manifest{
		VelocityVersion: *version,
		BuiltAt:         time.Now().UTC().Format(time.RFC3339),
		Counts:          map[kb.Kind]int{},
	}
	seenPkg := map[string]bool{}
	for i := range entries {
		if err := w.Insert(ctx, entries[i]); err != nil {
			return fmt.Errorf("insert %q: %w", entries[i].Title, err)
		}
		manifest.Counts[entries[i].Kind]++
		manifest.Total++
		if p := entries[i].Package; p != "" && !seenPkg[p] {
			seenPkg[p] = true
			manifest.Packages = append(manifest.Packages, p)
		}
	}

	if err := w.Finalize(ctx, manifest); err != nil {
		return fmt.Errorf("finalize: %w", err)
	}

	fmt.Fprintf(os.Stderr, "ingest: wrote %d entries to %s (velocity %s)\n", manifest.Total, *out, *version)
	return nil
}

// embedEntries fills Embedding on each entry in batches. When no backend is
// available it logs and leaves embeddings nil, yielding a keyword-only snapshot.
func embedEntries(ctx context.Context, emb embed.Embedder, entries []kb.Entry) {
	for start := 0; start < len(entries); start += embedBatch {
		end := min(start+embedBatch, len(entries))
		texts := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			texts = append(texts, embedText(entries[i]))
		}
		vecs, err := emb.Embed(ctx, texts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "ingest: embeddings unavailable (%v); building keyword-only\n", err)
			return
		}
		for i, v := range vecs {
			entries[start+i].Embedding = v
		}
	}
}

// embedText is the text vectorised for an entry: headline plus body.
func embedText(e kb.Entry) string {
	if e.Body == "" {
		return e.Title
	}
	return e.Title + "\n" + e.Body
}

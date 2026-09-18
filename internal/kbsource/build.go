package kbsource

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/velocitykode/velocity-arrow/internal/corpus"
	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
)

// BuildInput names the sources of one snapshot.
type BuildInput struct {
	VelocityDir string    // framework source tree to parse
	Version     string    // stamp written into every entry and the manifest
	DocsRoot    string    // published docs content tree; "" skips doc pages
	BuiltAt     time.Time // manifest timestamp; zero = now
	Out         string    // snapshot path to write
}

const embedBatch = 64

// Build writes a snapshot from the framework source, the curated rules and,
// when given, the docs tree. It is the single implementation behind both the
// release publisher (cmd/ingest) and the on-demand local build.
func Build(ctx context.Context, in BuildInput) error {
	rulesDir, err := fs.Sub(kb.RulesFS, "rules")
	if err != nil {
		return fmt.Errorf("open rules: %w", err)
	}

	var entries []kb.Entry

	syms, err := corpus.Symbols(ctx, in.VelocityDir, in.Version)
	if err != nil {
		return fmt.Errorf("symbols: %w", err)
	}
	entries = append(entries, syms...)

	curated, err := corpus.Markdown(rulesDir, in.Version)
	if err != nil {
		return fmt.Errorf("markdown: %w", err)
	}
	entries = append(entries, curated...)

	if in.DocsRoot != "" {
		pages, derr := corpus.Docs(os.DirFS(in.DocsRoot), in.Version)
		if derr != nil {
			return fmt.Errorf("docs: %w", derr)
		}
		if len(pages) == 0 {
			return fmt.Errorf("docs: no pages found under %q; check the path", in.DocsRoot)
		}
		entries = append(entries, pages...)
	}

	if len(entries) == 0 {
		return fmt.Errorf("no entries gathered; check velocity path %q", in.VelocityDir)
	}

	embedEntries(ctx, embed.New(), entries)

	w, err := store.Create(ctx, in.Out)
	if err != nil {
		return fmt.Errorf("create snapshot: %w", err)
	}
	defer w.Close()

	builtAt := in.BuiltAt
	if builtAt.IsZero() {
		builtAt = time.Now()
	}
	manifest := kb.Manifest{
		VelocityVersion: in.Version,
		BuiltAt:         builtAt.UTC().Format(time.RFC3339),
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
	return nil
}

// embedEntries fills Embedding on each entry in batches. When no backend is
// available it leaves embeddings nil, yielding a keyword-only snapshot.
func embedEntries(ctx context.Context, emb embed.Embedder, entries []kb.Entry) {
	for start := 0; start < len(entries); start += embedBatch {
		end := min(start+embedBatch, len(entries))
		texts := make([]string, 0, end-start)
		for i := start; i < end; i++ {
			texts = append(texts, embedText(entries[i]))
		}
		vecs, err := emb.Embed(ctx, texts)
		if err != nil {
			return
		}
		for i, v := range vecs {
			entries[start+i].Embedding = v
		}
	}
}

func embedText(e kb.Entry) string {
	if e.Body == "" {
		return e.Title
	}
	return e.Title + "\n" + e.Body
}

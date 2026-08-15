package corpus

import (
	"fmt"
	"io/fs"
	"path"
	"sort"
	"strings"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

// Docs loads published documentation pages from a Hugo content tree (the
// vel.build site's content/docs directory) as KindDoc entries: one entry per
// page, Ref carrying the site path ("/docs/core/async") so a consumer can
// deep-link the page, Tags carrying the section for retrieval.
//
// Pages without a front-matter title and pages marked draft are skipped; Hugo
// shortcode lines ({{< ... >}} / {{% ... %}}) are stripped from bodies since
// they are render directives, not content.
func Docs(fsys fs.FS, version string) ([]kb.Entry, error) {
	var paths []string
	err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(p), ".md") {
			paths = append(paths, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("corpus: walking docs tree: %w", err)
	}
	sort.Strings(paths)

	entries := make([]kb.Entry, 0, len(paths))
	for _, p := range paths {
		raw, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return nil, fmt.Errorf("corpus: reading %s: %w", p, rerr)
		}
		entry, ok, perr := parseDocPage(string(raw), p, version)
		if perr != nil {
			return nil, perr
		}
		if ok {
			entries = append(entries, entry)
		}
	}
	return entries, nil
}

// parseDocPage turns one Hugo page into a KindDoc entry. ok is false for pages
// that should not be indexed (no title, draft, or empty body after cleanup).
func parseDocPage(content, p, version string) (kb.Entry, bool, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	front, body, err := splitFrontMatter(content, p)
	if err != nil {
		return kb.Entry{}, false, err
	}
	meta := parseFrontMatter(front)

	if meta["title"] == "" || parseBool(meta["draft"]) {
		return kb.Entry{}, false, nil
	}

	body = stripShortcodes(body)
	if body == "" {
		return kb.Entry{}, false, nil
	}

	// Description leads the body so a page whose vocabulary lives mostly in
	// code blocks still matches conceptual queries.
	if desc := meta["description"]; desc != "" {
		body = desc + "\n\n" + body
	}

	ref, section := docSitePath(p)
	return kb.Entry{
		Kind:    kb.KindDoc,
		Title:   meta["title"],
		Body:    body,
		Ref:     ref,
		Version: version,
		Tags:    []string{"docs", section},
	}, true, nil
}

// docSitePath maps a content-relative file path to its published site path and
// section: "core/async.md" -> ("/docs/core/async", "core"), an _index.md maps
// to its directory ("advanced/_index.md" -> "/docs/advanced").
func docSitePath(p string) (ref, section string) {
	p = strings.TrimSuffix(p, ".md")
	if path.Base(p) == "_index" {
		p = path.Dir(p)
	}
	if p == "." || p == "" {
		return "/docs", "docs"
	}
	section = p
	if i := strings.Index(p, "/"); i >= 0 {
		section = p[:i]
	}
	return "/docs/" + p, section
}

// stripShortcodes drops lines that are purely Hugo shortcode delimiters and
// trims the result. Inline prose around a shortcode is preserved.
func stripShortcodes(body string) string {
	lines := strings.Split(body, "\n")
	kept := lines[:0]
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "{{<") || strings.HasPrefix(t, "{{%") {
			continue
		}
		kept = append(kept, ln)
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

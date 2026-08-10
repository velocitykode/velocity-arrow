package corpus

import (
	"fmt"
	"io/fs"
	"sort"
	"strings"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

// Markdown implements the documented contract: load one entry per *.md file in
// fsys, parsing a "---"-fenced front-matter block followed by an answer body.
func Markdown(fsys fs.FS, version string) ([]kb.Entry, error) {
	var paths []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".md") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("corpus: walking markdown tree: %w", err)
	}
	// Deterministic order regardless of filesystem iteration.
	sort.Strings(paths)

	entries := make([]kb.Entry, 0, len(paths))
	for _, path := range paths {
		raw, rerr := fs.ReadFile(fsys, path)
		if rerr != nil {
			return nil, fmt.Errorf("corpus: reading %s: %w", path, rerr)
		}
		entry, perr := parseMarkdown(string(raw), path, version)
		if perr != nil {
			return nil, perr
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

// parseMarkdown turns one file's contents into a kb.Entry.
func parseMarkdown(content, path, version string) (kb.Entry, error) {
	// Normalise line endings so CRLF files parse identically.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")

	front, body, err := splitFrontMatter(content, path)
	if err != nil {
		return kb.Entry{}, err
	}

	meta := parseFrontMatter(front)

	title := meta["title"]
	if title == "" {
		return kb.Entry{}, fmt.Errorf("corpus: %s: front-matter is missing a title", path)
	}

	entry := kb.Entry{
		Kind:       resolveKind(meta["kind"]),
		Title:      title,
		Body:       strings.TrimSpace(body),
		Package:    meta["package"],
		Tags:       parseTags(meta["tags"]),
		UseInstead: meta["use_instead"],
		Deprecated: parseBool(meta["deprecated"]),
		Ref:        path,
		Version:    version,
	}
	return entry, nil
}

// splitFrontMatter separates the "---"-fenced front matter from the body. The
// opening fence may be preceded only by blank lines.
func splitFrontMatter(content, path string) (front, body string, err error) {
	lines := strings.Split(content, "\n")

	// Find the opening fence, allowing leading blank lines.
	start := -1
	for i, ln := range lines {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.TrimSpace(ln) == "---" {
			start = i
		}
		break
	}
	if start == -1 {
		return "", "", fmt.Errorf("corpus: %s: no front-matter fence (expected a leading '---' line)", path)
	}

	// Find the closing fence.
	end := -1
	for i := start + 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			end = i
			break
		}
	}
	if end == -1 {
		return "", "", fmt.Errorf("corpus: %s: unterminated front-matter (missing closing '---')", path)
	}

	front = strings.Join(lines[start+1:end], "\n")
	if end+1 < len(lines) {
		body = strings.Join(lines[end+1:], "\n")
	}
	return front, body, nil
}

// parseFrontMatter reads "key: value" lines into a map. Unknown keys are kept
// but ignored by callers; blank lines and comment-style lines are skipped.
func parseFrontMatter(front string) map[string]string {
	meta := make(map[string]string)
	for _, ln := range strings.Split(front, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		idx := strings.Index(ln, ":")
		if idx < 0 {
			continue
		}
		key := strings.TrimSpace(ln[:idx])
		val := strings.TrimSpace(ln[idx+1:])
		val = trimQuotes(val)
		if key == "" {
			continue
		}
		meta[strings.ToLower(key)] = val
	}
	return meta
}

// resolveKind maps a front-matter kind to a valid kb.Kind, defaulting to
// KindRule for empty or unrecognised values.
func resolveKind(raw string) kb.Kind {
	k := kb.Kind(strings.ToLower(strings.TrimSpace(raw)))
	for _, valid := range kb.Kinds {
		if k == valid {
			return k
		}
	}
	return kb.KindRule
}

// parseTags accepts both "[a, b, c]" and "a, b, c" forms.
func parseTags(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	raw = strings.TrimPrefix(raw, "[")
	raw = strings.TrimSuffix(raw, "]")

	var tags []string
	for _, part := range strings.Split(raw, ",") {
		t := trimQuotes(strings.TrimSpace(part))
		if t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// parseBool interprets the truthy front-matter forms.
func parseBool(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "yes", "1":
		return true
	default:
		return false
	}
}

// trimQuotes removes a single matching pair of surrounding quotes.
func trimQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '"' && s[len(s)-1] == '"') || (s[0] == '\'' && s[len(s)-1] == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

package docs

import (
	"embed"
	"path/filepath"
	"strings"
)

//go:embed content
var content embed.FS

// DocEntry represents a single documentation page.
type DocEntry struct {
	Path    string
	Title   string
	Content string
}

// AllDocs returns all embedded documentation entries.
func AllDocs() []DocEntry {
	var entries []DocEntry
	readDir("content", &entries)
	return entries
}

func readDir(dir string, entries *[]DocEntry) {
	dirEntries, err := content.ReadDir(dir)
	if err != nil {
		return
	}

	for _, entry := range dirEntries {
		path := filepath.Join(dir, entry.Name())
		if entry.IsDir() {
			readDir(path, entries)
			continue
		}

		if !strings.HasSuffix(entry.Name(), ".md") {
			continue
		}

		data, err := content.ReadFile(path)
		if err != nil {
			continue
		}

		title := titleFromPath(path)
		*entries = append(*entries, DocEntry{
			Path:    path,
			Title:   title,
			Content: string(data),
		})
	}
}

func titleFromPath(path string) string {
	// Convert path like "content/orm/queries.md" to "ORM - Queries"
	path = strings.TrimPrefix(path, "content/")
	path = strings.TrimSuffix(path, ".md")

	parts := strings.Split(path, "/")
	for i, part := range parts {
		parts[i] = strings.Title(strings.ReplaceAll(part, "-", " "))
	}

	return strings.Join(parts, " - ")
}

package docs

import "testing"

func TestAllDocs(t *testing.T) {
	entries := AllDocs()
	if len(entries) == 0 {
		t.Fatal("expected at least one embedded doc entry")
	}

	for _, entry := range entries {
		if entry.Path == "" {
			t.Error("entry has empty path")
		}
		if entry.Title == "" {
			t.Error("entry has empty title")
		}
		if entry.Content == "" {
			t.Errorf("entry %q has empty content", entry.Path)
		}
	}
}

func TestAllDocs_ContainsORM(t *testing.T) {
	entries := AllDocs()
	found := false
	for _, entry := range entries {
		if entry.Title == "Orm - Overview" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find ORM overview doc")
	}
}

func TestAllDocs_ContainsGettingStarted(t *testing.T) {
	entries := AllDocs()
	found := false
	for _, entry := range entries {
		if entry.Title == "Getting Started" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find getting-started doc")
	}
}

func TestTitleFromPath(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"content/getting-started.md", "Getting Started"},
		{"content/orm/overview.md", "Orm - Overview"},
		{"content/orm/queries.md", "Orm - Queries"},
	}

	for _, tt := range tests {
		got := titleFromPath(tt.path)
		if got != tt.want {
			t.Errorf("titleFromPath(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

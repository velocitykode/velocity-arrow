package corpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestSymbolsHonoursBuildConstraints guards snapshot reproducibility: a symbol
// defined once per platform must be extracted once, from the linux file, so
// two ingests of the same tree are byte-identical and the knowledge base
// describes one platform rather than whichever file go/doc met first.
func TestSymbolsHonoursBuildConstraints(t *testing.T) {
	root := writeFixture(t)
	dir := filepath.Join(root, "widget")

	suffixed := "package widget\n\n// Hello is the windows variant and must not win.\nfunc Hello(subject string) string { return \"\" }\n"
	if err := os.WriteFile(filepath.Join(dir, "widget_windows.go"), []byte(suffixed), 0o644); err != nil {
		t.Fatalf("write suffixed fixture: %v", err)
	}
	tagged := "//go:build darwin\n\npackage widget\n\n// OnlyDarwin exists on one platform and is outside the snapshot.\nfunc OnlyDarwin() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "widget_tagged.go"), []byte(tagged), 0o644); err != nil {
		t.Fatalf("write tagged fixture: %v", err)
	}

	entries, err := Symbols(context.Background(), root, "vtest")
	if err != nil {
		t.Fatalf("Symbols: %v", err)
	}

	var hello int
	for _, e := range entries {
		switch e.Title {
		case "Hello":
			hello++
			if !strings.HasPrefix(e.Ref, "widget/widget.go:") {
				t.Errorf("Hello extracted from %q, want the linux file widget/widget.go", e.Ref)
			}
			if strings.Contains(e.Body, "windows") {
				t.Errorf("Hello body came from the windows file: %q", e.Body)
			}
		case "OnlyDarwin":
			t.Errorf("OnlyDarwin extracted; //go:build darwin must be excluded")
		}
	}
	if hello != 1 {
		t.Fatalf("Hello extracted %d times, want exactly 1", hello)
	}
}

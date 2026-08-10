package corpus

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

const fixturePkg = `// Package widget is a fixture.
package widget

// Greeter greets people by name.
type Greeter struct {
	Name string
}

// Hello returns a greeting for the given subject.
func Hello(subject string) string {
	return "hi " + subject
}

// Greet emits a greeting using the receiver's name.
func (g Greeter) Greet() string {
	return "hi from " + g.Name
}

// internalHelper is unexported and must not appear in the output.
func internalHelper() bool { return true }

type private struct{ x int }
`

func writeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "widget")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "widget.go"), []byte(fixturePkg), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	return root
}

func entriesByTitle(entries []kb.Entry) map[string]kb.Entry {
	m := make(map[string]kb.Entry, len(entries))
	for _, e := range entries {
		m[e.Title] = e
	}
	return m
}

func TestSymbols(t *testing.T) {
	root := writeFixture(t)

	entries, err := Symbols(context.Background(), root, "v1.2.3")
	if err != nil {
		t.Fatalf("Symbols: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected entries, got none")
	}

	byTitle := entriesByTitle(entries)

	tests := []struct {
		name        string
		title       string
		wantPresent bool
		wantSigSub  string
		wantPkg     string
		wantBodySub string
	}{
		{name: "exported func", title: "Hello", wantPresent: true, wantSigSub: "func Hello(subject string) string", wantPkg: "widget", wantBodySub: "returns a greeting"},
		{name: "exported type", title: "Greeter", wantPresent: true, wantSigSub: "type Greeter struct", wantPkg: "widget", wantBodySub: "greets people"},
		{name: "method dotted title", title: "Greeter.Greet", wantPresent: true, wantSigSub: "func (g Greeter) Greet() string", wantPkg: "widget"},
		{name: "unexported func excluded", title: "internalHelper", wantPresent: false},
		{name: "unexported type excluded", title: "private", wantPresent: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, ok := byTitle[tc.title]
			if ok != tc.wantPresent {
				t.Fatalf("presence of %q = %v, want %v", tc.title, ok, tc.wantPresent)
			}
			if !tc.wantPresent {
				return
			}
			if e.Kind != kb.KindSymbol {
				t.Errorf("kind = %q, want %q", e.Kind, kb.KindSymbol)
			}
			if tc.wantSigSub != "" && !strings.Contains(e.Signature, tc.wantSigSub) {
				t.Errorf("signature = %q, want substring %q", e.Signature, tc.wantSigSub)
			}
			if strings.Contains(e.Signature, "return ") {
				t.Errorf("signature should be body-stripped, got %q", e.Signature)
			}
			if tc.wantPkg != "" && e.Package != tc.wantPkg {
				t.Errorf("package = %q, want %q", e.Package, tc.wantPkg)
			}
			if tc.wantBodySub != "" && !strings.Contains(e.Body, tc.wantBodySub) {
				t.Errorf("body = %q, want substring %q", e.Body, tc.wantBodySub)
			}
			if e.Version != "v1.2.3" {
				t.Errorf("version = %q, want v1.2.3", e.Version)
			}
			// Ref must be "relpath:line", relative (no temp-dir prefix).
			if !strings.HasPrefix(e.Ref, "widget/widget.go:") {
				t.Errorf("ref = %q, want prefix widget/widget.go:", e.Ref)
			}
			if filepath.IsAbs(strings.SplitN(e.Ref, ":", 2)[0]) {
				t.Errorf("ref path should be relative, got %q", e.Ref)
			}
		})
	}
}

func TestSymbolsSkipsTestFiles(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "widget")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "widget.go"), []byte(fixturePkg), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	testFile := "package widget\n\n// FromTest must not be extracted.\nfunc FromTest() {}\n"
	if err := os.WriteFile(filepath.Join(dir, "extra_test.go"), []byte(testFile), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	entries, err := Symbols(context.Background(), root, "v0")
	if err != nil {
		t.Fatalf("Symbols: %v", err)
	}
	if _, ok := entriesByTitle(entries)["FromTest"]; ok {
		t.Error("symbol from _test.go file should be excluded")
	}
}

func TestSymbolsContextCancelled(t *testing.T) {
	root := writeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Symbols(ctx, root, "v0"); err == nil {
		t.Error("expected error from cancelled context")
	}
}

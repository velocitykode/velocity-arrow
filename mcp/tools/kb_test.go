package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-mcp/content"
	"github.com/velocitykode/velocity-mcp/server"
)

// openKBStore opens the embedded knowledge-base snapshot the shipped binary
// serves, so these tests exercise the real corpus rather than a fixture.
func openKBStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), kb.SnapshotDB, embed.New())
	if err != nil {
		t.Fatalf("opening knowledge-base snapshot: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestKBHandlers(t *testing.T) {
	s := openKBStore(t)

	tests := []struct {
		name         string
		handler      func(context.Context, *server.Request) (*server.Response, error)
		args         map[string]any
		wantContains []string
	}{
		{
			name:         "kb-search finds the hashing surface",
			handler:      NewKBSearchHandler(s),
			args:         map[string]any{"query": "hash a password"},
			wantContains: []string{"[symbol]", "Hasher", "auth"},
		},
		{
			name:         "kb-search honors the kind filter",
			handler:      NewKBSearchHandler(s),
			args:         map[string]any{"query": "password", "kind": string(kb.KindSymbol), "limit": 3},
			wantContains: []string{"[symbol]"},
		},
		{
			name:         "kb-search miss stays explicit about the boundary",
			handler:      NewKBSearchHandler(s),
			args:         map[string]any{"query": "zzqqxx-not-a-framework-thing"},
			wantContains: []string{"Absence here does NOT mean"},
		},
		{
			name:         "kb-symbol returns the exact signature",
			handler:      NewKBSymbolHandler(s),
			args:         map[string]any{"name": "Hasher"},
			wantContains: []string{"type Hasher interface", "ref: auth/hasher.go"},
		},
		{
			name:         "kb-symbol miss stays explicit about the boundary",
			handler:      NewKBSymbolHandler(s),
			args:         map[string]any{"name": "NoSuchSymbolInVelocity"},
			wantContains: []string{"No such symbol in the knowledge base"},
		},
		{
			name:         "kb-guard returns curated rules for a topic",
			handler:      NewKBGuardHandler(s),
			args:         map[string]any{"topic": "logging"},
			wantContains: []string{"[rule]", "STDOUT"},
		},
		{
			name:         "kb-guard with no topic returns the highest-signal guards",
			handler:      NewKBGuardHandler(s),
			args:         map[string]any{},
			wantContains: []string{"[rule]"},
		},
		{
			name:         "kb-guard unknown topic reports no guards",
			handler:      NewKBGuardHandler(s),
			args:         map[string]any{"topic": "zzqqxx"},
			wantContains: []string{"No guards recorded"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler(context.Background(), makeRequest(tt.args))
			if err != nil {
				t.Fatalf("handler error: %v", err)
			}
			if result.IsError() {
				t.Fatalf("tool error: %s", result.Contents()[0].(*content.Text).String())
			}

			text := result.Contents()[0].(*content.Text).String()
			for _, want := range tt.wantContains {
				if !strings.Contains(text, want) {
					t.Errorf("response missing %q\ngot:\n%s", want, text)
				}
			}
		})
	}
}

func TestKBManifestResource(t *testing.T) {
	s := openKBStore(t)
	res := NewKBManifestResource(s)

	if res.URI() != KBManifestURI {
		t.Errorf("URI = %q, want %q", res.URI(), KBManifestURI)
	}
	if res.Name() != "kb-manifest" {
		t.Errorf("Name = %q, want %q", res.Name(), "kb-manifest")
	}
	if res.MimeType() != "application/json" {
		t.Errorf("MimeType = %q, want %q", res.MimeType(), "application/json")
	}

	result, err := res.Read(context.Background(), makeRequest(nil))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if result.IsError() {
		t.Fatalf("resource error: %s", result.Contents()[0].(*content.Text).String())
	}

	var m kb.Manifest
	raw := result.Contents()[0].(*content.Text).String()
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("decoding manifest %q: %v", raw, err)
	}
	if m.Total == 0 {
		t.Error("manifest reports zero entries")
	}
	if m.VelocityVersion == "" {
		t.Error("manifest carries no velocity version stamp")
	}
	if m.Counts[kb.KindSymbol] == 0 {
		t.Error("manifest reports no symbol entries")
	}
}

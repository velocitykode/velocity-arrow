package mcp

import (
	"context"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/internal/embed"
	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-arrow/mcp/tools"
	"github.com/velocitykode/velocity-mcp/schema"
	"github.com/velocitykode/velocity-mcp/server"
)

// openKB opens the embedded knowledge-base snapshot the server serves.
func openKB(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Open(context.Background(), kb.SnapshotDB, embed.New())
	if err != nil {
		t.Fatalf("opening knowledge-base snapshot: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func TestToolDefinitions_NamesAndSchemas(t *testing.T) {
	tests := []struct {
		name       string
		tool       *server.ToolBuilder
		wantParams []string // expected parameter names
		required   []string // expected required params
	}{
		{
			name:       "velocity_app_info",
			tool:       appInfoTool(),
			wantParams: nil, // no params
		},
		{
			name:       "velocity_db_schema",
			tool:       dbSchemaTool(),
			wantParams: []string{"detail", "filter", "database"},
		},
		{
			name:       "velocity_db_query",
			tool:       dbQueryTool(false),
			wantParams: []string{"query", "database"},
			required:   []string{"query"},
		},
		{
			name:       "velocity_routes",
			tool:       routesTool(),
			wantParams: nil,
		},
		{
			name:       "velocity_search_docs",
			tool:       searchDocsTool(),
			wantParams: []string{"queries", "packages", "token_limit"},
			required:   []string{"queries"},
		},
		{
			name:       "velocity_last_error",
			tool:       lastErrorTool(),
			wantParams: nil,
		},
		{
			name:       "velocity_log_entries",
			tool:       logEntriesTool(),
			wantParams: []string{"level", "pattern", "limit", "date"},
		},
		{
			name:       "velocity_config",
			tool:       configTool(),
			wantParams: []string{"key"},
		},
		{
			name:       "velocity_kb_search",
			tool:       kbSearchTool(),
			wantParams: []string{"query", "kind", "limit"},
			required:   []string{"query"},
		},
		{
			name:       "velocity_kb_symbol",
			tool:       kbSymbolTool(),
			wantParams: []string{"name"},
			required:   []string{"name"},
		},
		{
			name:       "velocity_kb_guard",
			tool:       kbGuardTool(),
			wantParams: []string{"topic"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.tool.Name() != tt.name {
				t.Errorf("tool name = %q, want %q", tt.tool.Name(), tt.name)
			}

			if tt.tool.Description() == "" {
				t.Error("tool description is empty")
			}

			// Verify parameters exist in schema
			obj := schema.NewObject()
			tt.tool.Schema(obj)
			schemaMap := obj.ToMap()
			props, _ := schemaMap["properties"].(map[string]any)

			for _, param := range tt.wantParams {
				if len(props) == 0 {
					t.Errorf("expected param %q but schema has no properties", param)
					continue
				}
				if _, ok := props[param]; !ok {
					t.Errorf("missing expected parameter %q in schema", param)
				}
			}

			// Verify required fields
			if len(tt.required) > 0 {
				requiredList, _ := schemaMap["required"].([]string)
				for _, req := range tt.required {
					found := false
					for _, r := range requiredList {
						if r == req {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("param %q should be required", req)
					}
				}
			}
		})
	}
}

func TestRegisterTools_AllRegistered(t *testing.T) {
	s := newServer(false, openKB(t))

	// Verify all project tools plus the knowledge-base tools are registered
	expectedNames := []string{
		"velocity_app_info",
		"velocity_db_schema",
		"velocity_db_query",
		"velocity_routes",
		"velocity_search_docs",
		"velocity_last_error",
		"velocity_log_entries",
		"velocity_config",
		"velocity_kb_search",
		"velocity_kb_symbol",
		"velocity_kb_guard",
	}

	registered := s.Tools()
	if len(registered) != len(expectedNames) {
		t.Fatalf("registered tools = %d, want %d", len(registered), len(expectedNames))
	}
	for i, name := range expectedNames {
		if registered[i].Name() != name {
			t.Errorf("tools[%d] = %q, want %q", i, registered[i].Name(), name)
		}
	}
}

func TestNewServer_KnowledgeBaseSurface(t *testing.T) {
	s := newServer(false, openKB(t))

	resources := s.Resources()
	if len(resources) != 1 {
		t.Fatalf("registered resources = %d, want 1", len(resources))
	}
	if got := resources[0].URI(); got != tools.KBManifestURI {
		t.Errorf("resource URI = %q, want %q", got, tools.KBManifestURI)
	}

	for _, want := range []string{"velocity_kb_guard", "velocity_kb_symbol", "velocity_kb_search", "kb://manifest"} {
		if !strings.Contains(instructions, want) {
			t.Errorf("server instructions do not mention %q", want)
		}
	}
}

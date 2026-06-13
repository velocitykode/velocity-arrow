package mcp

import (
	"testing"

	"github.com/velocitykode/velocity-mcp/schema"
	"github.com/velocitykode/velocity-mcp/server"
)

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
			wantParams: []string{"summary", "filter", "database"},
		},
		{
			name:       "velocity_db_query",
			tool:       dbQueryTool(),
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
			wantParams: []string{"entries"},
		},
		{
			name:       "velocity_config",
			tool:       configTool(),
			wantParams: []string{"key"},
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

func TestRegisterTools_AllEightRegistered(t *testing.T) {
	s := newServer()

	// Verify all 8 tools are registered
	expectedNames := []string{
		"velocity_app_info",
		"velocity_db_schema",
		"velocity_db_query",
		"velocity_routes",
		"velocity_search_docs",
		"velocity_last_error",
		"velocity_log_entries",
		"velocity_config",
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

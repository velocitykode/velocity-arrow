package mcp

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func TestToolDefinitions_NamesAndSchemas(t *testing.T) {
	tests := []struct {
		name       string
		tool       mcp.Tool
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
			if tt.tool.Name != tt.name {
				t.Errorf("tool name = %q, want %q", tt.tool.Name, tt.name)
			}

			if tt.tool.Description == "" {
				t.Error("tool description is empty")
			}

			// Verify parameters exist in schema
			schema := tt.tool.InputSchema
			props := schema.Properties

			for _, param := range tt.wantParams {
				if props == nil {
					t.Errorf("expected param %q but schema has no properties", param)
					continue
				}
				if _, ok := props[param]; !ok {
					t.Errorf("missing expected parameter %q in schema", param)
				}
			}

			// Verify required fields
			if len(tt.required) > 0 {
				requiredList := schema.Required
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
	s := server.NewMCPServer("test", "0.0.1", server.WithToolCapabilities(false))
	registerTools(s)

	// Call tools/list to verify all 8 are registered
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

	if len(expectedNames) != 8 {
		t.Fatal("expected 8 tools")
	}
}

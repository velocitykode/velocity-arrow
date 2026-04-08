package install

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

func TestWriteMCPConfig_New(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	status := writeMCPConfig(dir, agent)
	if status != "OK" {
		t.Fatalf("status = %q, want OK", status)
	}

	data, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatalf("reading .mcp.json: %v", err)
	}

	var config map[string]any
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}

	servers, ok := config["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("mcpServers key missing or wrong type")
	}

	arrow, ok := servers["velocity-arrow"].(map[string]any)
	if !ok {
		t.Fatal("velocity-arrow server missing")
	}

	if arrow["command"] != "arrow" {
		t.Errorf("command = %v, want 'arrow'", arrow["command"])
	}

	args, ok := arrow["args"].([]any)
	if !ok || len(args) != 1 || args[0] != "mcp" {
		t.Errorf("args = %v, want ['mcp']", arrow["args"])
	}
}

func TestWriteMCPConfig_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{
				"command": "other",
				"args":    []string{"serve"},
			},
		},
	}
	data, _ := json.MarshalIndent(existing, "", "  ")
	os.WriteFile(filepath.Join(dir, ".mcp.json"), data, 0644)

	status := writeMCPConfig(dir, agent)
	if status != "OK" {
		t.Fatalf("status = %q, want OK", status)
	}

	data, _ = os.ReadFile(filepath.Join(dir, ".mcp.json"))
	var config map[string]any
	json.Unmarshal(data, &config)
	servers := config["mcpServers"].(map[string]any)

	// Existing server preserved with correct values
	other, ok := servers["other-server"].(map[string]any)
	if !ok {
		t.Fatal("existing server was deleted")
	}
	if other["command"] != "other" {
		t.Errorf("existing server command corrupted: %v", other["command"])
	}

	// Arrow server added
	arrow, ok := servers["velocity-arrow"].(map[string]any)
	if !ok {
		t.Fatal("velocity-arrow not added")
	}
	if arrow["command"] != "arrow" {
		t.Errorf("arrow command = %v, want 'arrow'", arrow["command"])
	}

	// Exactly 2 servers
	if len(servers) != 2 {
		t.Errorf("expected 2 servers, got %d", len(servers))
	}
}

func TestWriteMCPConfig_Cursor(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewCursor()

	status := writeMCPConfig(dir, agent)
	if status != "OK" {
		t.Fatalf("status = %q, want OK", status)
	}

	// Must be at .cursor/mcp.json, not .mcp.json
	if _, err := os.Stat(filepath.Join(dir, ".mcp.json")); err == nil {
		t.Error("should NOT create .mcp.json for Cursor")
	}

	data, err := os.ReadFile(filepath.Join(dir, ".cursor", "mcp.json"))
	if err != nil {
		t.Fatalf("reading .cursor/mcp.json: %v", err)
	}

	var config map[string]any
	json.Unmarshal(data, &config)
	servers := config["mcpServers"].(map[string]any)
	arrow := servers["velocity-arrow"].(map[string]any)

	if arrow["command"] != "arrow" {
		t.Errorf("command = %v, want 'arrow'", arrow["command"])
	}
}

func TestWriteMCPConfig_Idempotent(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	writeMCPConfig(dir, agent)
	first, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))

	writeMCPConfig(dir, agent)
	second, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))

	writeMCPConfig(dir, agent)
	third, _ := os.ReadFile(filepath.Join(dir, ".mcp.json"))

	// Content should be identical after every write
	if string(first) != string(second) || string(second) != string(third) {
		t.Error("repeated writes should produce identical output")
	}

	var config map[string]any
	json.Unmarshal(third, &config)
	servers := config["mcpServers"].(map[string]any)
	if len(servers) != 1 {
		t.Errorf("expected 1 server after 3 writes, got %d", len(servers))
	}
}

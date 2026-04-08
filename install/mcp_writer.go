package install

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

// writeMCPConfig registers the Arrow MCP server in the agent's config file.
// Returns a status string.
func writeMCPConfig(dir string, agent agents.MCPAgent) string {
	path := filepath.Join(dir, agent.MCPConfigPath())
	configKey := agent.MCPConfigKey()

	// Read existing config or start fresh
	config := make(map[string]any)

	existing, err := os.ReadFile(path)
	if err == nil {
		if err := json.Unmarshal(existing, &config); err != nil {
			// If existing file is invalid JSON, start fresh
			config = make(map[string]any)
		}
	}

	// Ensure the servers section exists
	servers, ok := config[configKey].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}

	// Register velocity-arrow MCP server
	servers["velocity-arrow"] = map[string]any{
		"command": "arrow",
		"args":    []string{"mcp"},
	}

	config[configKey] = servers

	// Ensure parent directory exists
	if parentDir := filepath.Dir(agent.MCPConfigPath()); parentDir != "." {
		if err := os.MkdirAll(filepath.Join(dir, parentDir), 0755); err != nil {
			return fmt.Sprintf("FAILED (%v)", err)
		}
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Sprintf("FAILED (%v)", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Sprintf("FAILED (%v)", err)
	}

	return "OK"
}

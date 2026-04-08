package agents

import (
	"os"
	"path/filepath"
	"runtime"
)

// Cursor implements the Agent interface for the Cursor editor.
type Cursor struct{}

func NewCursor() *Cursor {
	return &Cursor{}
}

func (c *Cursor) Name() string {
	return "Cursor"
}

func (c *Cursor) DetectOnSystem() bool {
	switch runtime.GOOS {
	case "darwin":
		_, err := os.Stat("/Applications/Cursor.app")
		return err == nil
	case "linux":
		_, err := os.Stat("/opt/cursor")
		return err == nil
	default:
		return false
	}
}

func (c *Cursor) DetectInProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".cursor"))
	return err == nil
}

func (c *Cursor) GuidelinesPath() string {
	return "AGENTS.md"
}

func (c *Cursor) GuidelinesTag() string {
	return "velocity-arrow-guidelines"
}

func (c *Cursor) SkillsDir() string {
	return filepath.Join(".cursor", "skills")
}

func (c *Cursor) MCPConfigPath() string {
	return filepath.Join(".cursor", "mcp.json")
}

func (c *Cursor) MCPConfigKey() string {
	return "mcpServers"
}

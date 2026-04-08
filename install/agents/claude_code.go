package agents

import (
	"os"
	"os/exec"
	"path/filepath"
)

// ClaudeCode implements the Agent interface for Claude Code.
type ClaudeCode struct{}

func NewClaudeCode() *ClaudeCode {
	return &ClaudeCode{}
}

func (c *ClaudeCode) Name() string {
	return "Claude Code"
}

func (c *ClaudeCode) DetectOnSystem() bool {
	_, err := exec.LookPath("claude")
	return err == nil
}

func (c *ClaudeCode) DetectInProject(dir string) bool {
	// Check for .claude/ directory or CLAUDE.md
	if _, err := os.Stat(filepath.Join(dir, ".claude")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(dir, "CLAUDE.md")); err == nil {
		return true
	}
	return false
}

func (c *ClaudeCode) GuidelinesPath() string {
	return "CLAUDE.md"
}

func (c *ClaudeCode) GuidelinesTag() string {
	return "velocity-arrow-guidelines"
}

func (c *ClaudeCode) SkillsDir() string {
	return filepath.Join(".claude", "skills")
}

func (c *ClaudeCode) MCPConfigPath() string {
	return ".mcp.json"
}

func (c *ClaudeCode) MCPConfigKey() string {
	return "mcpServers"
}

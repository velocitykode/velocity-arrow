package agents

import (
	"os"
	"path/filepath"
	"testing"
)

// --- Claude Code ---

func TestClaudeCode_Name(t *testing.T) {
	a := NewClaudeCode()
	if a.Name() != "Claude Code" {
		t.Errorf("Name = %q, want Claude Code", a.Name())
	}
}

func TestClaudeCode_Paths(t *testing.T) {
	a := NewClaudeCode()
	if a.GuidelinesPath() != "CLAUDE.md" {
		t.Errorf("GuidelinesPath = %q", a.GuidelinesPath())
	}
	if a.GuidelinesTag() != "velocity-arrow-guidelines" {
		t.Errorf("GuidelinesTag = %q", a.GuidelinesTag())
	}
	if a.SkillsDir() != filepath.Join(".claude", "skills") {
		t.Errorf("SkillsDir = %q", a.SkillsDir())
	}
	if a.MCPConfigPath() != ".mcp.json" {
		t.Errorf("MCPConfigPath = %q", a.MCPConfigPath())
	}
	if a.MCPConfigKey() != "mcpServers" {
		t.Errorf("MCPConfigKey = %q", a.MCPConfigKey())
	}
}

func TestClaudeCode_DetectInProject(t *testing.T) {
	dir := t.TempDir()

	if NewClaudeCode().DetectInProject(dir) {
		t.Error("should not detect in empty dir")
	}

	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)
	if !NewClaudeCode().DetectInProject(dir) {
		t.Error("should detect .claude/ directory")
	}
}

func TestClaudeCode_DetectInProject_CLAUDE_MD(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "CLAUDE.md"), []byte("test"), 0644)
	if !NewClaudeCode().DetectInProject(dir) {
		t.Error("should detect CLAUDE.md file")
	}
}

func TestClaudeCode_DetectOnSystem(t *testing.T) {
	// Just verify it doesn't panic
	_ = NewClaudeCode().DetectOnSystem()
}

// --- Cursor ---

func TestCursor_Name(t *testing.T) {
	a := NewCursor()
	if a.Name() != "Cursor" {
		t.Errorf("Name = %q, want Cursor", a.Name())
	}
}

func TestCursor_Paths(t *testing.T) {
	a := NewCursor()
	if a.GuidelinesPath() != "AGENTS.md" {
		t.Errorf("GuidelinesPath = %q", a.GuidelinesPath())
	}
	if a.GuidelinesTag() != "velocity-arrow-guidelines" {
		t.Errorf("GuidelinesTag = %q", a.GuidelinesTag())
	}
	if a.SkillsDir() != filepath.Join(".cursor", "skills") {
		t.Errorf("SkillsDir = %q", a.SkillsDir())
	}
	if a.MCPConfigPath() != filepath.Join(".cursor", "mcp.json") {
		t.Errorf("MCPConfigPath = %q", a.MCPConfigPath())
	}
	if a.MCPConfigKey() != "mcpServers" {
		t.Errorf("MCPConfigKey = %q", a.MCPConfigKey())
	}
}

func TestCursor_DetectInProject(t *testing.T) {
	dir := t.TempDir()

	if NewCursor().DetectInProject(dir) {
		t.Error("should not detect in empty dir")
	}

	os.MkdirAll(filepath.Join(dir, ".cursor"), 0755)
	if !NewCursor().DetectInProject(dir) {
		t.Error("should detect .cursor/ directory")
	}
}

func TestCursor_DetectOnSystem(t *testing.T) {
	_ = NewCursor().DetectOnSystem()
}

// --- Codex ---

func TestCodex_Name(t *testing.T) {
	a := NewCodex()
	if a.Name() != "Codex" {
		t.Errorf("Name = %q, want Codex", a.Name())
	}
}

func TestCodex_Paths(t *testing.T) {
	a := NewCodex()
	if a.GuidelinesPath() != "AGENTS.md" {
		t.Errorf("GuidelinesPath = %q", a.GuidelinesPath())
	}
	if a.GuidelinesTag() != "velocity-arrow-guidelines" {
		t.Errorf("GuidelinesTag = %q", a.GuidelinesTag())
	}
}

func TestCodex_DetectInProject(t *testing.T) {
	dir := t.TempDir()

	if NewCodex().DetectInProject(dir) {
		t.Error("should not detect in empty dir")
	}

	os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("test"), 0644)
	if !NewCodex().DetectInProject(dir) {
		t.Error("should detect AGENTS.md file")
	}
}

func TestCodex_DetectOnSystem(t *testing.T) {
	_ = NewCodex().DetectOnSystem()
}

// --- Interface compliance ---

func TestClaudeCode_Interfaces(t *testing.T) {
	var _ Agent = NewClaudeCode()
	var _ GuidelinesAgent = NewClaudeCode()
	var _ SkillsAgent = NewClaudeCode()
	var _ MCPAgent = NewClaudeCode()
}

func TestCursor_Interfaces(t *testing.T) {
	var _ Agent = NewCursor()
	var _ GuidelinesAgent = NewCursor()
	var _ SkillsAgent = NewCursor()
	var _ MCPAgent = NewCursor()
}

func TestCodex_Interfaces(t *testing.T) {
	var _ Agent = NewCodex()
	var _ GuidelinesAgent = NewCodex()
}

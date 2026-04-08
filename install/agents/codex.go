package agents

import (
	"os"
	"os/exec"
	"path/filepath"
)

// Codex implements the Agent interface for OpenAI Codex.
type Codex struct{}

func NewCodex() *Codex {
	return &Codex{}
}

func (c *Codex) Name() string {
	return "Codex"
}

func (c *Codex) DetectOnSystem() bool {
	_, err := exec.LookPath("codex")
	return err == nil
}

func (c *Codex) DetectInProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "AGENTS.md"))
	return err == nil
}

func (c *Codex) GuidelinesPath() string {
	return "AGENTS.md"
}

func (c *Codex) GuidelinesTag() string {
	return "velocity-arrow-guidelines"
}

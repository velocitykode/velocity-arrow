package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

func TestParseProjectGoMod(t *testing.T) {
	dir := t.TempDir()
	gomod := `module myapp

go 1.26

require (
	github.com/velocitykode/velocity v0.20.3
	github.com/joho/godotenv v1.5.1
)
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	info, err := parseProjectGoMod(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if info.module != "myapp" {
		t.Errorf("module = %q, want myapp", info.module)
	}
	if info.goVersion != "1.26" {
		t.Errorf("goVersion = %q, want 1.26", info.goVersion)
	}
	if len(info.deps) != 2 {
		t.Fatalf("deps count = %d, want 2", len(info.deps))
	}
}

func TestIsVelocityProject(t *testing.T) {
	tests := []struct {
		name string
		deps []projectDep
		want bool
	}{
		{
			name: "has velocity",
			deps: []projectDep{{path: "github.com/velocitykode/velocity", version: "v0.20.3"}},
			want: true,
		},
		{
			name: "has velocity subpackage",
			deps: []projectDep{{path: "github.com/velocitykode/velocity/orm", version: "v0.20.3"}},
			want: true,
		},
		{
			name: "no velocity",
			deps: []projectDep{{path: "github.com/gin-gonic/gin", version: "v1.9.0"}},
			want: false,
		},
		{
			name: "empty deps",
			deps: nil,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			goMod := &projectGoMod{deps: tt.deps}
			got := isVelocityProject(goMod)
			if got != tt.want {
				t.Errorf("isVelocityProject = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasDep(t *testing.T) {
	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
			{path: "github.com/joho/godotenv", version: "v1.5.1"},
			{path: "github.com/velocitykode/velwatch-go", version: "v0.0.1"},
		},
	}

	tests := []struct {
		dep  string
		want bool
	}{
		{"github.com/velocitykode/velocity", true},
		{"github.com/joho/godotenv", true},
		{"github.com/velocitykode/velwatch-go", true},
		{"github.com/not/here", false},
	}

	for _, tt := range tests {
		t.Run(tt.dep, func(t *testing.T) {
			got := hasDep(goMod, tt.dep)
			if got != tt.want {
				t.Errorf("hasDep(%q) = %v, want %v", tt.dep, got, tt.want)
			}
		})
	}
}

func TestDetectAgents_ClaudeCode(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	agents := detectAgents(dir)

	found := false
	for _, a := range agents {
		if a.Name() == "Claude Code" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Claude Code to be detected via .claude/ directory")
	}
}

func TestDetectAgents_Cursor(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, ".cursor"), 0755)

	agents := detectAgents(dir)

	found := false
	for _, a := range agents {
		if a.Name() == "Cursor" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Cursor to be detected via .cursor/ directory")
	}
}

func TestDetectAgents_None(t *testing.T) {
	dir := t.TempDir()
	// No agent markers - detection depends on system install
	// We can't guarantee none are on the system, so just verify no panic
	_ = detectAgents(dir)
}

func TestRun_NotVelocityProject(t *testing.T) {
	dir := t.TempDir()
	gomod := `module notvelocity

go 1.26

require github.com/gin-gonic/gin v1.9.0
`
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0644)

	err := Run(dir)
	if err == nil {
		t.Fatal("expected error for non-velocity project")
	}
}

func TestRun_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	err := Run(dir)
	if err == nil {
		t.Fatal("expected error for missing go.mod")
	}
}

func TestRun_VelocityProject(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module testapp

go 1.26

require github.com/velocitykode/velocity v0.20.3
`), 0644)

	// Create .claude/ so Claude Code is detected
	os.MkdirAll(filepath.Join(dir, ".claude"), 0755)

	err := Run(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify CLAUDE.md was created with guidelines
	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatal("CLAUDE.md should be created")
	}
	content := string(data)
	if !strings.Contains(content, "<velocity-arrow-guidelines>") {
		t.Error("CLAUDE.md should contain guidelines tags")
	}
	if !strings.Contains(content, "testapp") {
		t.Error("CLAUDE.md should contain templated module name")
	}

	// Verify .mcp.json was created
	mcpData, err := os.ReadFile(filepath.Join(dir, ".mcp.json"))
	if err != nil {
		t.Fatal(".mcp.json should be created")
	}
	if !strings.Contains(string(mcpData), "velocity-arrow") {
		t.Error(".mcp.json should register velocity-arrow")
	}

	// Verify skills were installed
	_, err = os.Stat(filepath.Join(dir, ".claude", "skills", "framework-generate", "SKILL.md"))
	if err != nil {
		t.Error("framework-generate skill should be installed")
	}
}

func TestFormatAgentNames(t *testing.T) {
	agentList := []agents.Agent{
		agents.NewClaudeCode(),
		agents.NewCursor(),
	}
	result := formatAgentNames(agentList)
	if result != "Claude Code, Cursor" {
		t.Errorf("formatAgentNames = %q, want 'Claude Code, Cursor'", result)
	}
}

func TestFormatAgentNames_Single(t *testing.T) {
	agentList := []agents.Agent{agents.NewCodex()}
	result := formatAgentNames(agentList)
	if result != "Codex" {
		t.Errorf("formatAgentNames = %q, want 'Codex'", result)
	}
}

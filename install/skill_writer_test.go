package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

func TestWriteSkills_OK(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
		},
	}

	skills := matchSkills(goMod)
	status := writeSkills(dir, agent, skills)

	if status != "OK" {
		t.Fatalf("status = %q, want OK", status)
	}

	skillsDir := filepath.Join(dir, ".claude", "skills")

	// Verify exact skill directories created
	entries, err := os.ReadDir(skillsDir)
	if err != nil {
		t.Fatalf("reading skills dir: %v", err)
	}

	skillNames := make(map[string]bool)
	for _, entry := range entries {
		skillNames[entry.Name()] = true
	}

	if !skillNames["framework-generate"] {
		t.Error("missing framework-generate skill dir")
	}
	if !skillNames["framework-review"] {
		t.Error("missing framework-review skill dir")
	}

	// Verify SKILL.md has real content (not empty files)
	for _, name := range []string{"framework-generate", "framework-review"} {
		data, err := os.ReadFile(filepath.Join(skillsDir, name, "SKILL.md"))
		if err != nil {
			t.Errorf("reading %s/SKILL.md: %v", name, err)
			continue
		}
		content := string(data)
		if len(content) < 50 {
			t.Errorf("%s/SKILL.md is too short (%d bytes) - likely empty or corrupt", name, len(content))
		}
		// Verify frontmatter is present
		if !strings.Contains(content, "name:") {
			t.Errorf("%s/SKILL.md missing frontmatter 'name:' field", name)
		}
		if !strings.Contains(content, "description:") {
			t.Errorf("%s/SKILL.md missing frontmatter 'description:' field", name)
		}
		// Verify the skill name in frontmatter matches the directory
		if !strings.Contains(content, "name: "+name) {
			t.Errorf("%s/SKILL.md frontmatter name should match directory name", name)
		}
	}
}

func TestWriteSkills_Empty_CreatesNoFiles(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	status := writeSkills(dir, agent, nil)
	if status != "NOOP" {
		t.Errorf("status = %q, want NOOP", status)
	}

	// Verify no skills directory was created
	skillsDir := filepath.Join(dir, ".claude", "skills")
	_, err := os.Stat(skillsDir)
	if err == nil {
		entries, _ := os.ReadDir(skillsDir)
		if len(entries) > 0 {
			t.Errorf("NOOP should not create any skill dirs, found %d", len(entries))
		}
	}
}

func TestWriteSkills_CursorPath(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewCursor()

	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
		},
	}

	skills := matchSkills(goMod)
	status := writeSkills(dir, agent, skills)
	if status != "OK" {
		t.Fatalf("status = %q, want OK", status)
	}

	// Must be under .cursor/skills/, not .claude/skills/
	cursorSkillsDir := filepath.Join(dir, ".cursor", "skills")
	claudeSkillsDir := filepath.Join(dir, ".claude", "skills")

	if _, err := os.Stat(cursorSkillsDir); err != nil {
		t.Fatal("skills should be in .cursor/skills/")
	}
	if _, err := os.Stat(claudeSkillsDir); err == nil {
		t.Error("should NOT create .claude/skills/ for Cursor agent")
	}

	// Verify content was written
	data, err := os.ReadFile(filepath.Join(cursorSkillsDir, "framework-generate", "SKILL.md"))
	if err != nil {
		t.Fatalf("reading SKILL.md: %v", err)
	}
	if !strings.Contains(string(data), "framework-generate") {
		t.Error("SKILL.md content should reference framework-generate")
	}
}

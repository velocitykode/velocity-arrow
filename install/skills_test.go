package install

import (
	"strings"
	"testing"
)

func TestMatchSkills_WithVelocity(t *testing.T) {
	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
		},
	}

	skills := matchSkills(goMod)
	if len(skills) < 2 {
		t.Fatalf("expected at least 2 skills for velocity dep, got %d", len(skills))
	}

	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}

	if !names["framework-generate"] {
		t.Error("missing framework-generate skill")
	}
	if !names["framework-review"] {
		t.Error("missing framework-review skill")
	}
}

func TestMatchSkills_WithVelwatchGo(t *testing.T) {
	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
			{path: "github.com/velocitykode/velwatch-go", version: "v0.0.1"},
		},
	}

	skills := matchSkills(goMod)
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}

	if !names["velwatch-instrumentation"] {
		t.Error("missing velwatch-instrumentation skill")
	}
}

func TestMatchSkills_WithVelocityCLI(t *testing.T) {
	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/velocitykode/velocity", version: "v0.20.3"},
			{path: "github.com/velocitykode/velocity-cli", version: "v0.8.32"},
		},
	}

	skills := matchSkills(goMod)
	names := make(map[string]bool)
	for _, s := range skills {
		names[s.Name] = true
	}

	if !names["vel-generate"] {
		t.Error("missing vel-generate skill")
	}
}

func TestMatchSkills_NoDeps(t *testing.T) {
	goMod := &projectGoMod{
		deps: []projectDep{
			{path: "github.com/gin-gonic/gin", version: "v1.9.0"},
		},
	}

	skills := matchSkills(goMod)
	if len(skills) != 0 {
		t.Errorf("expected no skills for non-velocity dep, got %d", len(skills))
	}
}

func TestReadSkillFile(t *testing.T) {
	skill := Skill{Name: "framework-generate", Dir: "framework-generate"}
	content, err := readSkillFile(skill, "SKILL.md")
	if err != nil {
		t.Fatalf("readSkillFile error: %v", err)
	}
	if content == "" {
		t.Error("SKILL.md should not be empty")
	}
	if !strings.Contains(content, "framework-generate") {
		t.Error("SKILL.md should contain skill name")
	}
}

func TestReadSkillFile_NotFound(t *testing.T) {
	skill := Skill{Name: "framework-generate", Dir: "framework-generate"}
	_, err := readSkillFile(skill, "NONEXISTENT.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestCleanSkillName(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"Framework Generate", "framework-generate"},
		{"vel-generate", "vel-generate"},
		{"My Skill", "my-skill"},
	}
	for _, tt := range tests {
		got := cleanSkillName(tt.input)
		if got != tt.want {
			t.Errorf("cleanSkillName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

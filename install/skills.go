package install

import (
	"embed"
	"strings"
)

//go:embed skills
var skillsFS embed.FS

// Skill represents an embedded skill that can be installed.
type Skill struct {
	Name        string
	Description string
	Dir         string // directory name in skills/
}

// skillMapping maps go.mod dependencies to skills.
var skillMapping = []struct {
	dep    string
	skills []Skill
}{
	{
		dep: "github.com/velocitykode/velocity",
		skills: []Skill{
			{Name: "framework-generate", Description: "Generate Velocity framework code", Dir: "framework-generate"},
			{Name: "framework-review", Description: "Review Velocity framework code", Dir: "framework-review"},
		},
	},
	{
		dep: "github.com/velocitykode/velwatch-go",
		skills: []Skill{
			{Name: "velwatch-instrumentation", Description: "Velwatch observability instrumentation", Dir: "velwatch-instrumentation"},
		},
	},
	{
		dep: "github.com/velocitykode/velocity-cli",
		skills: []Skill{
			{Name: "vel-generate", Description: "Generate Velocity CLI commands", Dir: "vel-generate"},
		},
	},
}

// matchSkills returns skills matching the project's go.mod dependencies.
func matchSkills(goMod *projectGoMod) []Skill {
	var matched []Skill

	for _, mapping := range skillMapping {
		if hasDep(goMod, mapping.dep) {
			for _, skill := range mapping.skills {
				// Verify the skill directory exists in embedded FS
				if dirExists(skillsFS, "skills/"+skill.Dir) {
					matched = append(matched, skill)
				}
			}
		}
	}

	return matched
}

func dirExists(fs embed.FS, path string) bool {
	entries, err := fs.ReadDir(path)
	return err == nil && len(entries) > 0
}

// readSkillFile reads a file from an embedded skill.
func readSkillFile(skill Skill, filename string) (string, error) {
	path := "skills/" + skill.Dir + "/" + filename
	data, err := skillsFS.ReadFile(path)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

package install

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

// writeSkills copies matched skills to the agent's skills directory.
// Returns a status string.
func writeSkills(dir string, agent agents.SkillsAgent, skills []Skill) string {
	if len(skills) == 0 {
		return "NOOP"
	}

	skillsDir := filepath.Join(dir, agent.SkillsDir())

	// Ensure skills directory exists
	if err := os.MkdirAll(skillsDir, 0755); err != nil {
		return fmt.Sprintf("FAILED (%v)", err)
	}

	installed := 0
	for _, skill := range skills {
		skillDir := filepath.Join(skillsDir, skill.Name)
		if err := os.MkdirAll(skillDir, 0755); err != nil {
			continue
		}

		// Copy all files from the embedded skill directory
		srcDir := "skills/" + skill.Dir
		entries, err := skillsFS.ReadDir(srcDir)
		if err != nil {
			continue
		}

		if err := copySkillFiles(srcDir, skillDir, entries); err != nil {
			continue
		}

		installed++
	}

	if installed == len(skills) {
		return "OK"
	}
	return fmt.Sprintf("PARTIAL (%d/%d)", installed, len(skills))
}

func copySkillFiles(srcDir, destDir string, entries []fs.DirEntry) error {
	for _, entry := range entries {
		srcPath := srcDir + "/" + entry.Name()

		if entry.IsDir() {
			subDestDir := filepath.Join(destDir, entry.Name())
			if err := os.MkdirAll(subDestDir, 0755); err != nil {
				return err
			}
			subEntries, err := skillsFS.ReadDir(srcPath)
			if err != nil {
				return err
			}
			if err := copySkillFiles(srcPath, subDestDir, subEntries); err != nil {
				return err
			}
			continue
		}

		data, err := skillsFS.ReadFile(srcPath)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, entry.Name())
		if err := os.WriteFile(destPath, data, 0644); err != nil {
			return err
		}
	}
	return nil
}

// cleanSkillName normalizes a skill name for filesystem use.
func cleanSkillName(name string) string {
	return strings.ReplaceAll(strings.ToLower(name), " ", "-")
}

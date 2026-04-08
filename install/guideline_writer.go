package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

// writeGuidelines writes composed guidelines to the agent's guidelines file.
// Returns a status string: NEW, REPLACED, or FAILED.
func writeGuidelines(dir string, agent agents.GuidelinesAgent, content string) string {
	path := filepath.Join(dir, agent.GuidelinesPath())
	tag := agent.GuidelinesTag()

	wrapped := wrapInTags(tag, content)

	existing, err := os.ReadFile(path)
	if err != nil {
		// File doesn't exist - create new
		if err := os.WriteFile(path, []byte(wrapped+"\n"), 0644); err != nil {
			return fmt.Sprintf("FAILED (%v)", err)
		}
		return "NEW"
	}

	// File exists - replace or append tagged section
	existingStr := string(existing)
	openTag := fmt.Sprintf("<%s>", tag)
	closeTag := fmt.Sprintf("</%s>", tag)

	openIdx := strings.Index(existingStr, openTag)
	closeIdx := strings.Index(existingStr, closeTag)

	if openIdx >= 0 && closeIdx >= 0 {
		// Replace existing tagged section
		before := existingStr[:openIdx]
		after := existingStr[closeIdx+len(closeTag):]
		updated := before + wrapped + after

		// Normalize multiple blank lines
		updated = normalizeBlankLines(updated)

		if err := os.WriteFile(path, []byte(updated), 0644); err != nil {
			return fmt.Sprintf("FAILED (%v)", err)
		}
		return "REPLACED"
	}

	// No existing tags - append
	appendContent := "\n\n" + wrapped + "\n"
	if err := os.WriteFile(path, []byte(existingStr+appendContent), 0644); err != nil {
		return fmt.Sprintf("FAILED (%v)", err)
	}
	return "NEW"
}

func wrapInTags(tag, content string) string {
	return fmt.Sprintf("<%s>\n%s\n</%s>", tag, strings.TrimSpace(content), tag)
}

func normalizeBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	var result []string
	blankCount := 0

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount <= 2 {
				result = append(result, line)
			}
		} else {
			blankCount = 0
			result = append(result, line)
		}
	}

	return strings.Join(result, "\n")
}

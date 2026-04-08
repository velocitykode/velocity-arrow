package install

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/install/agents"
)

func TestWriteGuidelines_New(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	status := writeGuidelines(dir, agent, "# Test Guidelines\n\nSome content.")
	if status != "NEW" {
		t.Errorf("status = %q, want NEW", status)
	}

	data, err := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	if err != nil {
		t.Fatalf("reading file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "<velocity-arrow-guidelines>") {
		t.Error("missing opening tag")
	}
	if !strings.Contains(content, "</velocity-arrow-guidelines>") {
		t.Error("missing closing tag")
	}
	if !strings.Contains(content, "# Test Guidelines") {
		t.Error("missing guidelines content")
	}
}

func TestWriteGuidelines_Replace(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	// Write initial
	writeGuidelines(dir, agent, "# First version")

	// Replace
	status := writeGuidelines(dir, agent, "# Second version")
	if status != "REPLACED" {
		t.Errorf("status = %q, want REPLACED", status)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	if strings.Contains(content, "First version") {
		t.Error("old content should be replaced")
	}
	if !strings.Contains(content, "Second version") {
		t.Error("new content should be present")
	}

	// Should only have one pair of tags
	if strings.Count(content, "<velocity-arrow-guidelines>") != 1 {
		t.Error("should have exactly one opening tag")
	}
}

func TestWriteGuidelines_PreservesExisting(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	// Write existing content first
	path := filepath.Join(dir, "CLAUDE.md")
	os.WriteFile(path, []byte("# My Project\n\nExisting content.\n"), 0644)

	status := writeGuidelines(dir, agent, "# Arrow Guidelines")
	if status != "NEW" {
		t.Errorf("status = %q, want NEW (appended to existing)", status)
	}

	data, _ := os.ReadFile(path)
	content := string(data)

	if !strings.Contains(content, "# My Project") {
		t.Error("existing content should be preserved")
	}
	if !strings.Contains(content, "# Arrow Guidelines") {
		t.Error("new guidelines should be appended")
	}
}

func TestWriteGuidelines_Idempotent(t *testing.T) {
	dir := t.TempDir()
	agent := agents.NewClaudeCode()

	// Write same content three times
	writeGuidelines(dir, agent, "# Same Content")
	writeGuidelines(dir, agent, "# Same Content")
	writeGuidelines(dir, agent, "# Same Content")

	data, _ := os.ReadFile(filepath.Join(dir, "CLAUDE.md"))
	content := string(data)

	// Should only have one pair of tags
	if strings.Count(content, "<velocity-arrow-guidelines>") != 1 {
		t.Errorf("expected 1 opening tag, got %d", strings.Count(content, "<velocity-arrow-guidelines>"))
	}
}

func TestWrapInTags(t *testing.T) {
	result := wrapInTags("test-tag", "  content here  ")
	expected := "<test-tag>\ncontent here\n</test-tag>"
	if result != expected {
		t.Errorf("wrapInTags = %q, want %q", result, expected)
	}
}

func TestNormalizeBlankLines(t *testing.T) {
	input := "line1\n\n\n\n\nline2\n\nline3"
	result := normalizeBlankLines(input)

	// Should collapse 5 blank lines to max 2 (which means max 3 consecutive newlines)
	lines := strings.Split(result, "\n")
	blankRun := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankRun++
			if blankRun > 2 {
				t.Error("should not have more than 2 consecutive blank lines")
				break
			}
		} else {
			blankRun = 0
		}
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line2") {
		t.Error("content should be preserved")
	}
}

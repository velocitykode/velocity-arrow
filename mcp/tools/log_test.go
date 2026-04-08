package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseLogEntries(t *testing.T) {
	content := `[09:12:03] INFO: Server started on :4000
[09:12:15] INFO: GET /dashboard | status=200
[09:13:01] WARN: Slow query | duration=450ms
[09:14:05] ERROR: Failed to send email | error="connection refused"
  goroutine 1 [running]:
  main.sendEmail()
      /app/handlers/auth.go:87
[09:15:30] INFO: POST /register | status=201`

	entries := parseLogEntries(content)
	if len(entries) != 5 {
		t.Fatalf("entries count = %d, want 5", len(entries))
	}

	// Verify exact content of each entry
	if entries[0] != "[09:12:03] INFO: Server started on :4000" {
		t.Errorf("entry[0] = %q", entries[0])
	}
	if entries[1] != "[09:12:15] INFO: GET /dashboard | status=200" {
		t.Errorf("entry[1] = %q", entries[1])
	}

	// Entry[3] is the ERROR with stack trace - must include all continuation lines
	errorEntry := entries[3]
	if !strings.HasPrefix(errorEntry, "[09:14:05] ERROR: Failed to send email") {
		t.Errorf("error entry should start with timestamp and ERROR, got: %q", errorEntry)
	}
	if !strings.Contains(errorEntry, "goroutine 1 [running]:") {
		t.Error("error entry should include goroutine line from stack trace")
	}
	if !strings.Contains(errorEntry, "/app/handlers/auth.go:87") {
		t.Error("error entry should include file:line from stack trace")
	}

	// Last entry
	if entries[4] != "[09:15:30] INFO: POST /register | status=201" {
		t.Errorf("entry[4] = %q", entries[4])
	}
}

func TestParseLogEntries_Empty(t *testing.T) {
	entries := parseLogEntries("")
	if len(entries) != 0 {
		t.Errorf("expected no entries, got %d", len(entries))
	}
}

func TestParseLogEntries_SingleEntry(t *testing.T) {
	entries := parseLogEntries("[09:00:00] INFO: Only entry")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0] != "[09:00:00] INFO: Only entry" {
		t.Errorf("entry = %q", entries[0])
	}
}

func TestFindLastError_ReturnsLastNotFirst(t *testing.T) {
	content := `[09:12:03] INFO: Server started
[09:14:05] ERROR: First error | id=1
[09:15:30] INFO: Some info
[09:16:01] ERROR: Second error | id=2
[09:16:02] INFO: Shutdown`

	result := findLastError(content)
	if result != "[09:16:01] ERROR: Second error | id=2" {
		t.Errorf("should return LAST error, got: %q", result)
	}
}

func TestFindLastError_None(t *testing.T) {
	content := `[09:12:03] INFO: Server started
[09:12:15] INFO: All good`

	result := findLastError(content)
	if result != "" {
		t.Errorf("expected empty, got %q", result)
	}
}

func TestFindLastError_IncludesStackTrace(t *testing.T) {
	content := `[09:12:03] INFO: Server started
[09:14:05] ERROR: Panic occurred
  goroutine 1 [running]:
  main.handler()
      /app/main.go:42
[09:15:30] INFO: Recovered`

	result := findLastError(content)

	// Must start with the ERROR line
	if !strings.HasPrefix(result, "[09:14:05] ERROR: Panic occurred") {
		t.Errorf("should start with ERROR line, got: %q", result)
	}
	// Must include the full stack trace
	if !strings.Contains(result, "goroutine 1 [running]:") {
		t.Error("should include goroutine line")
	}
	if !strings.Contains(result, "main.handler()") {
		t.Error("should include function name")
	}
	if !strings.Contains(result, "/app/main.go:42") {
		t.Error("should include file:line")
	}
	// Must NOT include the INFO line after
	if strings.Contains(result, "Recovered") {
		t.Error("should not include subsequent log entry")
	}
}

func TestFindLatestLogFile_FindsTodayFirst(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "storage", "logs")
	os.MkdirAll(logDir, 0755)

	// Create an older log and a newer one
	os.WriteFile(filepath.Join(logDir, "velocity-2026-01-01.log"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(logDir, "velocity-2026-04-08.log"), []byte("new"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	logFile, err := findLatestLogFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should return a path that ends with the latest log file
	if !strings.HasSuffix(logFile, "velocity-2026-04-08.log") {
		t.Errorf("expected latest log file, got %q", logFile)
	}
}

func TestFindLatestLogFile_FallsBackToSingleLog(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "storage", "logs")
	os.MkdirAll(logDir, 0755)

	os.WriteFile(filepath.Join(logDir, "velocity.log"), []byte("single"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	logFile, err := findLatestLogFile()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(logFile, "velocity.log") {
		t.Errorf("should fall back to velocity.log, got %q", logFile)
	}
}

func TestFindLatestLogFile_NoLogs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "storage", "logs"), 0755)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	_, err := findLatestLogFile()
	if err == nil {
		t.Fatal("expected error for empty logs dir")
	}
	if !strings.Contains(err.Error(), "no log files") {
		t.Errorf("error should mention 'no log files', got: %v", err)
	}
}

func TestIsLogEntryStart(t *testing.T) {
	tests := []struct {
		line  string
		start bool
	}{
		{"[09:12:03] INFO: test", true},
		{"2026-04-08 09:12:03 INFO test", true},
		{"  goroutine 1 [running]:", false},
		{"  main.handler()", false},
		{"", false},
		{"short", false},
	}

	for _, tt := range tests {
		got := isLogEntryStart(tt.line)
		if got != tt.start {
			t.Errorf("isLogEntryStart(%q) = %v, want %v", tt.line, got, tt.start)
		}
	}
}

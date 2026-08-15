package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-mcp/content"
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

func TestFilterLogEntries_Level(t *testing.T) {
	entries := []string{
		"[09:12:03] DEBUG: Query executed | sql=SELECT 1",
		"[09:12:04] INFO: Server started",
		"[09:13:01] WARN: Slow query | duration=450ms",
		"[09:14:05] ERROR: Connection refused",
		"no recognizable level here at all",
	}

	tests := []struct {
		name     string
		minLevel int
		want     int
	}{
		{"debug keeps all", logLevelRanks["DEBUG"], 5},
		{"info drops debug", logLevelRanks["INFO"], 4},
		{"warning drops debug and info", logLevelRanks["WARNING"], 3},
		{"error keeps errors only", logLevelRanks["ERROR"], 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterLogEntries(entries, tt.minLevel, "")
			if len(got) != tt.want {
				t.Errorf("matched %d entries, want %d: %q", len(got), tt.want, got)
			}
		})
	}

	// Unrecognized-level entries pass a level filter (never silently hidden).
	got := filterLogEntries(entries, logLevelRanks["ERROR"], "")
	if got[len(got)-1] != "no recognizable level here at all" {
		t.Errorf("unleveled entry should pass the filter, got: %q", got)
	}
}

func TestFilterLogEntries_Pattern(t *testing.T) {
	entries := []string{
		"[09:12:03] INFO: GET /dashboard | status=200",
		"[09:14:05] ERROR: Failed to send email\n  smtp.Dial()\n      /app/mail.go:12",
		"[09:15:30] INFO: POST /register | status=201",
	}

	tests := []struct {
		name    string
		pattern string
		want    int
	}{
		{"no pattern keeps all", "", 3},
		{"case-insensitive match", "DASHBOARD", 1},
		{"matches continuation lines", "mail.go", 1},
		{"no match", "does-not-exist", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterLogEntries(entries, -1, strings.ToLower(tt.pattern))
			if len(got) != tt.want {
				t.Errorf("matched %d entries, want %d: %q", len(got), tt.want, got)
			}
		})
	}
}

func TestFilterLogEntries_LevelAndPattern(t *testing.T) {
	entries := []string{
		"[09:12:03] DEBUG: Query executed | sql=SELECT * FROM users",
		"[09:13:01] ERROR: Query failed | sql=SELECT * FROM users",
		"[09:14:05] ERROR: Connection refused",
	}

	got := filterLogEntries(entries, logLevelRanks["ERROR"], "query")
	if len(got) != 1 {
		t.Fatalf("matched %d entries, want 1: %q", len(got), got)
	}
	if !strings.Contains(got[0], "Query failed") {
		t.Errorf("wrong entry matched: %q", got[0])
	}
}

func TestCollapseLogEntries(t *testing.T) {
	entries := []string{
		"[09:12:00] INFO: Server started",
		"[09:12:05] DEBUG: Query executed | sql=SELECT DISTINCT server_id",
		"[09:12:10] DEBUG: Query executed | sql=SELECT DISTINCT server_id",
		"[09:12:15] DEBUG: Query executed | sql=SELECT DISTINCT server_id",
		"[09:12:20] INFO: GET / | status=200",
		"[09:12:25] DEBUG: Query executed | sql=SELECT DISTINCT server_id",
	}

	collapsed := collapseLogEntries(entries)
	if len(collapsed) != 4 {
		t.Fatalf("collapsed to %d rows, want 4: %+v", len(collapsed), collapsed)
	}

	wantCounts := []int{1, 3, 1, 1}
	for i, want := range wantCounts {
		if collapsed[i].count != want {
			t.Errorf("row %d count = %d, want %d", i, collapsed[i].count, want)
		}
	}

	// Non-consecutive repeat starts a fresh row, not a merge.
	if collapsed[3].text != "[09:12:25] DEBUG: Query executed | sql=SELECT DISTINCT server_id" {
		t.Errorf("row 3 = %q", collapsed[3].text)
	}
}

func TestCollapseLogEntries_DateTimeTimestamps(t *testing.T) {
	entries := []string{
		"2026-04-08 09:12:03 DEBUG Query executed",
		"2026-04-08 09:12:08 DEBUG Query executed",
	}

	collapsed := collapseLogEntries(entries)
	if len(collapsed) != 1 {
		t.Fatalf("collapsed to %d rows, want 1", len(collapsed))
	}
	if collapsed[0].count != 2 {
		t.Errorf("count = %d, want 2", collapsed[0].count)
	}
}

func TestFormatCollapsedEntry(t *testing.T) {
	tests := []struct {
		name  string
		entry collapsedEntry
		want  string
	}{
		{
			"single occurrence keeps timestamp",
			collapsedEntry{text: "[09:12:00] INFO: Server started", count: 1},
			"[09:12:00] INFO: Server started",
		},
		{
			"repeats drop timestamp and gain count",
			collapsedEntry{text: "[09:12:05] DEBUG: Query executed | sql=SELECT DISTINCT server_id", count: 37},
			"[x37] DEBUG: Query executed | sql=SELECT DISTINCT server_id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatCollapsedEntry(tt.entry); got != tt.want {
				t.Errorf("formatCollapsedEntry() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFindLogFileForDate(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "storage", "logs")
	os.MkdirAll(logDir, 0755)

	os.WriteFile(filepath.Join(logDir, "velocity-2026-01-01.log"), []byte("old"), 0644)
	os.WriteFile(filepath.Join(logDir, "velocity-2026-04-08.log"), []byte("new"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	tests := []struct {
		name    string
		date    string
		want    string
		wantErr string
	}{
		{"older file", "2026-01-01", "velocity-2026-01-01.log", ""},
		{"latest file", "2026-04-08", "velocity-2026-04-08.log", ""},
		{"missing day", "2026-02-02", "", "no log file for 2026-02-02"},
		{"bad format", "yesterday", "", "invalid date"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := findLogFileForDate(tt.date)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want containing %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasSuffix(got, tt.want) {
				t.Errorf("file = %q, want suffix %q", got, tt.want)
			}
		})
	}
}

func TestHandleLogEntries_Totals(t *testing.T) {
	dir := t.TempDir()
	logDir := filepath.Join(dir, "storage", "logs")
	os.MkdirAll(logDir, 0755)
	os.WriteFile(filepath.Join(logDir, "velocity-2026-04-08.log"), []byte(
		"[09:12:00] INFO: Server started\n"+
			"[09:12:05] DEBUG: Query executed | sql=SELECT DISTINCT server_id\n"+
			"[09:12:10] DEBUG: Query executed | sql=SELECT DISTINCT server_id\n"+
			"[09:12:15] DEBUG: Query executed | sql=SELECT DISTINCT server_id\n"+
			"[09:13:01] ERROR: Connection refused\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	t.Run("no params collapses and reports totals", func(t *testing.T) {
		result, err := HandleLogEntries(context.Background(), makeRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "Scanned: 5 | Matched: 5 | Returned: 3") {
			t.Errorf("totals wrong, got: %s", text)
		}
		if !strings.Contains(text, "[x3] DEBUG: Query executed | sql=SELECT DISTINCT server_id") {
			t.Errorf("expected collapsed DEBUG row, got: %s", text)
		}
	})

	t.Run("level filter changes matched total", func(t *testing.T) {
		result, _ := HandleLogEntries(context.Background(), makeRequest(map[string]any{
			"level": "ERROR",
		}))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "Scanned: 5 | Matched: 1 | Returned: 1") {
			t.Errorf("totals wrong, got: %s", text)
		}
		if strings.Contains(text, "DEBUG") {
			t.Errorf("DEBUG entries should be filtered out, got: %s", text)
		}
	})

	t.Run("pattern with no match reports zero returned", func(t *testing.T) {
		result, _ := HandleLogEntries(context.Background(), makeRequest(map[string]any{
			"pattern": "nothing-matches-this",
		}))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "Scanned: 5 | Matched: 0 | Returned: 0") {
			t.Errorf("totals wrong, got: %s", text)
		}
		if !strings.Contains(text, "No log entries matched.") {
			t.Errorf("expected no-match message, got: %s", text)
		}
	})

	t.Run("invalid level is a tool error", func(t *testing.T) {
		result, _ := HandleLogEntries(context.Background(), makeRequest(map[string]any{
			"level": "SEVERE",
		}))
		if !result.IsError() {
			t.Error("expected error for invalid level")
		}
	})

	t.Run("date selects that day's file", func(t *testing.T) {
		os.WriteFile(filepath.Join(logDir, "velocity-2026-01-01.log"),
			[]byte("[08:00:00] INFO: Old day entry\n"), 0644)

		result, _ := HandleLogEntries(context.Background(), makeRequest(map[string]any{
			"date": "2026-01-01",
		}))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "velocity-2026-01-01.log") {
			t.Errorf("expected older file in header, got: %s", text)
		}
		if !strings.Contains(text, "Old day entry") {
			t.Errorf("expected older file contents, got: %s", text)
		}
	})

	t.Run("missing date is a tool error", func(t *testing.T) {
		result, _ := HandleLogEntries(context.Background(), makeRequest(map[string]any{
			"date": "2020-12-31",
		}))
		if !result.IsError() {
			t.Error("expected error for missing date file")
		}
	})
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

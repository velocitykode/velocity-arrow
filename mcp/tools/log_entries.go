package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/velocitykode/velocity-mcp/server"
)

// logLevelRanks orders log levels by severity for minimum-level filtering.
var logLevelRanks = map[string]int{
	"DEBUG":   0,
	"INFO":    1,
	"WARN":    2,
	"WARNING": 2,
	"ERROR":   3,
}

// logLevelRe matches the level token on the first line of an entry.
var logLevelRe = regexp.MustCompile(`\b(DEBUG|INFO|WARNING|WARN|ERROR)\b`)

// logTimestampRe matches the leading timestamp of an entry's first line,
// either "[09:12:03] " or "2026-04-08 09:12:03 " style.
var logTimestampRe = regexp.MustCompile(`^(\[[^\]]*\]|\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(\.\d+)?)\s*`)

// collapsedEntry is one output row: a log entry plus how many consecutive
// occurrences (identical ignoring timestamp) it represents.
type collapsedEntry struct {
	text  string
	count int
}

// HandleLogEntries returns filtered log entries from the Velocity log file.
func HandleLogEntries(ctx context.Context, req *server.Request) (*server.Response, error) {
	limit := 10
	if v, ok := req.IntOK("limit"); ok {
		limit = int(v)
	} else if v, ok := req.IntOK("entries"); ok {
		// Legacy param name, kept for compatibility.
		limit = int(v)
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	minLevel := -1
	if v, ok := req.StringOK("level"); ok && v != "" {
		rank, known := logLevelRanks[strings.ToUpper(v)]
		if !known {
			return server.Error(fmt.Sprintf("invalid level %q: use DEBUG, INFO, WARNING or ERROR", v)), nil
		}
		minLevel = rank
	}

	pattern := strings.ToLower(req.String("pattern"))

	var logFile string
	var err error
	if date, ok := req.StringOK("date"); ok && date != "" {
		logFile, err = findLogFileForDate(date)
	} else {
		logFile, err = findLatestLogFile()
	}
	if err != nil {
		return server.Error(fmt.Sprintf("finding log file: %v", err)), nil
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		return server.Error(fmt.Sprintf("reading log file: %v", err)), nil
	}

	logEntries := parseLogEntries(string(data))
	scanned := len(logEntries)

	matched := filterLogEntries(logEntries, minLevel, pattern)
	collapsed := collapseLogEntries(matched)

	if len(collapsed) > limit {
		collapsed = collapsed[len(collapsed)-limit:]
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Log Entries (%s)\n\n", filepath.Base(logFile)))
	b.WriteString(fmt.Sprintf("Scanned: %d | Matched: %d | Returned: %d\n\n", scanned, len(matched), len(collapsed)))

	if len(collapsed) == 0 {
		b.WriteString("No log entries matched.\n")
		return server.Text(b.String()), nil
	}

	b.WriteString("```\n")
	for _, entry := range collapsed {
		b.WriteString(formatCollapsedEntry(entry))
		b.WriteString("\n")
	}
	b.WriteString("```\n")

	return server.Text(b.String()), nil
}

// findLogFileForDate returns the log file for a specific day (YYYY-MM-DD).
func findLogFileForDate(date string) (string, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return "", fmt.Errorf("invalid date %q: use YYYY-MM-DD", date)
	}

	logFile := filepath.Join(logDir(), fmt.Sprintf("velocity-%s.log", date))
	if _, err := os.Stat(logFile); err != nil {
		return "", fmt.Errorf("no log file for %s", date)
	}
	return logFile, nil
}

// filterLogEntries keeps entries at or above minLevel (pass -1 to skip) that
// contain pattern as a case-insensitive substring (pass "" to skip). Entries
// with no recognizable level pass a level filter, so odd lines are never
// silently hidden.
func filterLogEntries(entries []string, minLevel int, pattern string) []string {
	if minLevel < 0 && pattern == "" {
		return entries
	}

	var matched []string
	for _, entry := range entries {
		if minLevel >= 0 {
			if rank, ok := entryLevel(entry); ok && rank < minLevel {
				continue
			}
		}
		if pattern != "" && !strings.Contains(strings.ToLower(entry), pattern) {
			continue
		}
		matched = append(matched, entry)
	}
	return matched
}

// entryLevel returns the severity rank of an entry's first line.
func entryLevel(entry string) (int, bool) {
	firstLine := entry
	if i := strings.IndexByte(entry, '\n'); i >= 0 {
		firstLine = entry[:i]
	}
	m := logLevelRe.FindString(firstLine)
	if m == "" {
		return 0, false
	}
	return logLevelRanks[m], true
}

// collapseLogEntries merges consecutive entries that are identical once the
// leading timestamp is stripped.
func collapseLogEntries(entries []string) []collapsedEntry {
	var collapsed []collapsedEntry
	prevKey := ""

	for _, entry := range entries {
		key := logTimestampRe.ReplaceAllString(entry, "")
		if len(collapsed) > 0 && key == prevKey {
			collapsed[len(collapsed)-1].count++
			continue
		}
		collapsed = append(collapsed, collapsedEntry{text: entry, count: 1})
		prevKey = key
	}
	return collapsed
}

// formatCollapsedEntry renders one output row. Repeated entries drop their
// timestamp and gain a repeat count: "[x37] DEBUG: Query executed | ...".
func formatCollapsedEntry(entry collapsedEntry) string {
	if entry.count == 1 {
		return entry.text
	}
	return fmt.Sprintf("[x%d] %s", entry.count, logTimestampRe.ReplaceAllString(entry.text, ""))
}

func parseLogEntries(content string) []string {
	lines := strings.Split(content, "\n")
	var entries []string
	var current strings.Builder

	for _, line := range lines {
		if isLogEntryStart(line) {
			if current.Len() > 0 {
				entries = append(entries, strings.TrimRight(current.String(), "\n"))
				current.Reset()
			}
			current.WriteString(line)
		} else if current.Len() > 0 && line != "" {
			current.WriteString("\n")
			current.WriteString(line)
		}
	}

	if current.Len() > 0 {
		entries = append(entries, strings.TrimRight(current.String(), "\n"))
	}

	return entries
}

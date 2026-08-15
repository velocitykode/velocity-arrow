package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/velocitykode/velocity"
	"github.com/velocitykode/velocity-mcp/server"
)

// HandleLastError returns the last N ERROR entries from the Velocity logs,
// scanning log files newest-first until enough entries are found.
func HandleLastError(ctx context.Context, req *server.Request) (*server.Response, error) {
	count := 1
	if v, ok := req.IntOK("count"); ok {
		count = int(v)
	}
	if count <= 0 {
		count = 1
	}
	if count > 10 {
		count = 10
	}

	logFiles, err := findLogFiles()
	if err != nil {
		return server.Error(fmt.Sprintf("finding log files: %v", err)), nil
	}

	var found []logError
	for _, logFile := range logFiles {
		if len(found) >= count {
			break
		}
		data, err := os.ReadFile(logFile)
		if err != nil {
			return server.Error(fmt.Sprintf("reading log file: %v", err)), nil
		}
		for _, entry := range findErrors(string(data), count-len(found)) {
			found = append(found, logError{file: filepath.Base(logFile), entry: entry})
		}
	}

	if len(found) == 0 {
		oldest := logFileDate(logFiles[len(logFiles)-1])
		files := "files"
		if len(logFiles) == 1 {
			files = "file"
		}
		return server.Text(fmt.Sprintf("No ERROR entries in %d log %s scanned (oldest: %s).", len(logFiles), files, oldest)), nil
	}

	var b strings.Builder
	if len(found) == 1 {
		b.WriteString("# Last Error\n")
	} else {
		b.WriteString(fmt.Sprintf("# Last %d Errors (newest first)\n", len(found)))
	}
	for _, e := range found {
		entry := e.entry
		if len(entry) > 500 {
			entry = entry[:500] + "...\n(truncated)"
		}
		b.WriteString(fmt.Sprintf("\nFrom %s:\n\n```\n%s\n```\n", e.file, entry))
	}

	return server.Text(strings.TrimRight(b.String(), "\n")), nil
}

// logError is an ERROR entry paired with the log file it came from.
type logError struct {
	file  string
	entry string
}

// logDir returns the log directory from Velocity's log config.
func logDir() string {
	cfg := velocity.ConfigFromEnv()
	if path, ok := cfg.Log.Config["path"].(string); ok && path != "" {
		return path
	}
	return "./storage/logs"
}

func findLatestLogFile() (string, error) {
	dir := logDir()

	// Try today's log first
	today := time.Now().Format("2006-01-02")
	todayLog := filepath.Join(dir, fmt.Sprintf("velocity-%s.log", today))
	if _, err := os.Stat(todayLog); err == nil {
		return todayLog, nil
	}

	// Fall back to any velocity-*.log, most recent first
	matches, _ := filepath.Glob(filepath.Join(dir, "velocity-*.log"))
	if len(matches) > 0 {
		for i, j := 0, len(matches)-1; i < j; i, j = i+1, j-1 {
			matches[i], matches[j] = matches[j], matches[i]
		}
		return matches[0], nil
	}

	// Try a single velocity.log
	singleLog := filepath.Join(dir, "velocity.log")
	if _, err := os.Stat(singleLog); err == nil {
		return singleLog, nil
	}

	return "", fmt.Errorf("no log files found in %s", dir)
}

// findLogFiles returns all log files in the log directory, newest first.
func findLogFiles() ([]string, error) {
	dir := logDir()

	// Dated velocity-YYYY-MM-DD.log files sort newest-first lexically reversed
	matches, _ := filepath.Glob(filepath.Join(dir, "velocity-*.log"))
	if len(matches) > 0 {
		sort.Sort(sort.Reverse(sort.StringSlice(matches)))
		return matches, nil
	}

	// Fall back to a single velocity.log
	singleLog := filepath.Join(dir, "velocity.log")
	if _, err := os.Stat(singleLog); err == nil {
		return []string{singleLog}, nil
	}

	return nil, fmt.Errorf("no log files found in %s", dir)
}

// logFileDate extracts the date from a velocity-YYYY-MM-DD.log filename,
// falling back to the bare filename for undated logs.
func logFileDate(path string) string {
	name := filepath.Base(path)
	date := strings.TrimSuffix(strings.TrimPrefix(name, "velocity-"), ".log")
	if date == "" || date == name || strings.TrimSuffix(name, ".log") == "velocity" {
		return name
	}
	return date
}

func findLastError(content string) string {
	errors := findErrors(content, 1)
	if len(errors) == 0 {
		return ""
	}
	return errors[0]
}

// findErrors returns up to max ERROR entries from the log content, newest first.
func findErrors(content string, max int) []string {
	lines := strings.Split(content, "\n")
	var errors []string

	for i := len(lines) - 1; i >= 0 && len(errors) < max; i-- {
		if isErrorLine(lines[i]) {
			var entry []string
			entry = append(entry, lines[i])

			for j := i + 1; j < len(lines); j++ {
				if isLogEntryStart(lines[j]) {
					break
				}
				entry = append(entry, lines[j])
			}

			errors = append(errors, strings.Join(entry, "\n"))
		}
	}

	return errors
}

func isErrorLine(line string) bool {
	return strings.Contains(line, "ERROR") || strings.Contains(line, "error")
}

func isLogEntryStart(line string) bool {
	if len(line) == 0 {
		return false
	}
	return (len(line) > 10 && line[0] == '[') ||
		(len(line) > 10 && line[4] == '-' && line[7] == '-')
}

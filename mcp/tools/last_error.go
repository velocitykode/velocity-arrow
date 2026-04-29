package tools

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/velocitykode/velocity"
)

// HandleLastError returns the last ERROR entry from the Velocity log file.
func HandleLastError(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	logFile, err := findLatestLogFile()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("finding log file: %v", err)), nil
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reading log file: %v", err)), nil
	}

	lastError := findLastError(string(data))
	if lastError == "" {
		return mcp.NewToolResultText("No ERROR entries found in the log file."), nil
	}

	if len(lastError) > 500 {
		lastError = lastError[:500] + "...\n(truncated)"
	}

	return mcp.NewToolResultText(fmt.Sprintf("# Last Error\n\n```\n%s\n```", lastError)), nil
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

func findLastError(content string) string {
	lines := strings.Split(content, "\n")

	for i := len(lines) - 1; i >= 0; i-- {
		if isErrorLine(lines[i]) {
			var entry []string
			entry = append(entry, lines[i])

			for j := i + 1; j < len(lines); j++ {
				if isLogEntryStart(lines[j]) {
					break
				}
				entry = append(entry, lines[j])
			}

			return strings.Join(entry, "\n")
		}
	}

	return ""
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

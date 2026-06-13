package tools

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/velocitykode/velocity-mcp/server"
)

// HandleLogEntries returns the last N log entries from the Velocity log file.
func HandleLogEntries(ctx context.Context, req *server.Request) (*server.Response, error) {
	entries := 10
	if v, ok := req.IntOK("entries"); ok {
		entries = int(v)
	}
	if entries <= 0 {
		entries = 10
	}
	if entries > 100 {
		entries = 100
	}

	logFile, err := findLatestLogFile()
	if err != nil {
		return server.Error(fmt.Sprintf("finding log file: %v", err)), nil
	}

	data, err := os.ReadFile(logFile)
	if err != nil {
		return server.Error(fmt.Sprintf("reading log file: %v", err)), nil
	}

	logEntries := parseLogEntries(string(data))

	if len(logEntries) > entries {
		logEntries = logEntries[len(logEntries)-entries:]
	}

	if len(logEntries) == 0 {
		return server.Text("No log entries found."), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Last %d Log Entries\n\n", len(logEntries)))
	b.WriteString("```\n")
	for _, entry := range logEntries {
		b.WriteString(entry)
		b.WriteString("\n")
	}
	b.WriteString("```\n")

	return server.Text(b.String()), nil
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

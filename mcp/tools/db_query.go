package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

var allowedQueryPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)^\s*SELECT\b`),
	regexp.MustCompile(`(?i)^\s*SHOW\b`),
	regexp.MustCompile(`(?i)^\s*EXPLAIN\b`),
	regexp.MustCompile(`(?i)^\s*DESCRIBE\b`),
	regexp.MustCompile(`(?i)^\s*DESC\b`),
	regexp.MustCompile(`(?i)^\s*WITH\b[\s\S]+\bSELECT\b`),
}

var forbiddenPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\bINSERT\b`),
	regexp.MustCompile(`(?i)\bUPDATE\b`),
	regexp.MustCompile(`(?i)\bDELETE\b`),
	regexp.MustCompile(`(?i)\bDROP\b`),
	regexp.MustCompile(`(?i)\bALTER\b`),
	regexp.MustCompile(`(?i)\bCREATE\b`),
	regexp.MustCompile(`(?i)\bTRUNCATE\b`),
	regexp.MustCompile(`(?i)\bGRANT\b`),
	regexp.MustCompile(`(?i)\bREVOKE\b`),
}

// HandleDBQuery executes a read-only SQL query.
func HandleDBQuery(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	query, err := request.RequireString("query")
	if err != nil {
		return mcp.NewToolResultError("query parameter is required"), nil
	}

	database := request.GetString("database", "")

	if !isAllowedQuery(query) {
		return mcp.NewToolResultError("Only read-only queries are allowed: SELECT, SHOW, EXPLAIN, DESCRIBE, WITH...SELECT"), nil
	}

	db, _, err := openDB(database)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("database connection failed: %v", err)), nil
	}
	defer db.Close()

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("query error: %v", err)), nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("getting columns: %v", err)), nil
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("scanning row: %v", err)), nil
		}

		row := make(map[string]any)
		for i, col := range columns {
			val := values[i]
			if b, ok := val.([]byte); ok {
				val = string(b)
			}
			row[col] = val
		}
		results = append(results, row)
	}

	if err := rows.Err(); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("reading results: %v", err)), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Query returned %d rows.\n\n", len(results)))

	if len(results) > 0 {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return mcp.NewToolResultError(fmt.Sprintf("encoding results: %v", err)), nil
		}
		b.Write(data)
	}

	return mcp.NewToolResultText(b.String()), nil
}

func isAllowedQuery(query string) bool {
	// Check for forbidden patterns - always block, even inside WITH
	for _, pattern := range forbiddenPatterns {
		if pattern.MatchString(query) {
			return false
		}
	}

	for _, pattern := range allowedQueryPatterns {
		if pattern.MatchString(query) {
			return true
		}
	}
	return false
}

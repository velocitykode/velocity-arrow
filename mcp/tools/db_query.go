package tools

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/velocitykode/velocity-mcp/server"
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

// returningPattern matches a writing statement that still yields rows
// (e.g. Postgres INSERT/UPDATE/DELETE ... RETURNING), so it is routed
// through QueryContext rather than ExecContext.
var returningPattern = regexp.MustCompile(`(?i)\bRETURNING\b`)

// NewDBQueryHandler builds the velocity_db_query handler. When allowWrites
// is false (the default) the handler enforces read-only access via the
// allow/forbid regex gate. When true the gate is bypassed entirely and any
// SQL is executed - SELECT-shaped statements stream rows back, everything
// else runs through ExecContext and reports the affected-row count.
func NewDBQueryHandler(allowWrites bool) func(context.Context, *server.Request) (*server.Response, error) {
	return func(ctx context.Context, req *server.Request) (*server.Response, error) {
		query, ok := req.StringOK("query")
		if !ok {
			return server.Error("query parameter is required"), nil
		}

		database := req.String("database")

		if !allowWrites && !isAllowedQuery(query) {
			return server.Error("Only read-only queries are allowed: SELECT, SHOW, EXPLAIN, DESCRIBE, WITH...SELECT. Start arrow with --allow-writes (or ARROW_ALLOW_WRITES=1) to enable writes."), nil
		}

		db, _, err := openDB(database)
		if err != nil {
			return server.Error(fmt.Sprintf("database connection failed: %v", err)), nil
		}
		defer db.Close()

		// Route SELECT-shaped statements (and writes with RETURNING) through
		// QueryContext so rows stream back; route every other write through
		// ExecContext so a driver that refuses rows on a bare INSERT/UPDATE
		// does not error.
		if returnsRows(query) {
			return runRowQuery(ctx, db, query)
		}
		return runExec(ctx, db, query)
	}
}

func runRowQuery(ctx context.Context, db *sql.DB, query string) (*server.Response, error) {
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return server.Error(fmt.Sprintf("query error: %v", err)), nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return server.Error(fmt.Sprintf("getting columns: %v", err)), nil
	}

	var results []map[string]any
	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return server.Error(fmt.Sprintf("scanning row: %v", err)), nil
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
		return server.Error(fmt.Sprintf("reading results: %v", err)), nil
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Query returned %d rows.\n\n", len(results)))

	if len(results) > 0 {
		data, err := json.MarshalIndent(results, "", "  ")
		if err != nil {
			return server.Error(fmt.Sprintf("encoding results: %v", err)), nil
		}
		b.Write(data)
	}

	return server.Text(b.String()), nil
}

func runExec(ctx context.Context, db *sql.DB, query string) (*server.Response, error) {
	res, err := db.ExecContext(ctx, query)
	if err != nil {
		return server.Error(fmt.Sprintf("exec error: %v", err)), nil
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return server.Text("Statement executed (rows affected unavailable for this driver)."), nil
	}
	return server.Text(fmt.Sprintf("Statement executed. %d rows affected.", affected)), nil
}

// returnsRows reports whether a statement should be run through QueryContext
// (it yields a result set): any read-only statement, or a write with a
// RETURNING clause.
func returnsRows(query string) bool {
	if returningPattern.MatchString(query) {
		return true
	}
	for _, pattern := range allowedQueryPatterns {
		if pattern.MatchString(query) {
			return true
		}
	}
	return false
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

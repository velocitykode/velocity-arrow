package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/velocitykode/velocity-mcp/server"
)

// HandleDBSchema explores the database schema using Velocity's ORM schema
// introspection API, which compiles the dialect-specific SQL in the ORM
// grammars. The tool itself writes no dialect SQL, so it works identically
// across postgres, mysql, and sqlite.
func HandleDBSchema(ctx context.Context, req *server.Request) (*server.Response, error) {
	summary := true
	if v, ok := req.BoolOK("summary"); ok {
		summary = v
	}
	filter := req.String("filter")
	database := req.String("database")

	manager, err := openManager(database)
	if err != nil {
		return server.Error(fmt.Sprintf("database connection failed: %v", err)), nil
	}
	defer manager.DB().Close()

	tables, err := manager.ListTables(ctx)
	if err != nil {
		return server.Error(fmt.Sprintf("listing tables: %v", err)), nil
	}

	tables = filterTables(tables, filter)
	if len(tables) == 0 {
		return server.Text("No tables found."), nil
	}

	var b strings.Builder
	b.WriteString("# Database Schema\n\n")

	for _, table := range tables {
		b.WriteString(fmt.Sprintf("## %s\n", table))

		cols, err := manager.DescribeTable(ctx, table)
		if err != nil {
			b.WriteString(fmt.Sprintf("  Error: %v\n\n", err))
			continue
		}

		if summary {
			for _, col := range cols {
				b.WriteString(fmt.Sprintf("- %s %s\n", col.Name, col.DataType))
			}
		} else {
			b.WriteString("| Column | Type | Nullable | Default | Key |\n")
			b.WriteString("|--------|------|----------|---------|-----|\n")
			for _, col := range cols {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					col.Name, col.DataType, yesNo(col.Nullable), defaultOf(col.Default), keyOf(col.PrimaryKey)))
			}
		}
		b.WriteString("\n")
	}

	return server.Text(b.String()), nil
}

// filterTables keeps tables whose name contains filter (case-insensitive). An
// empty filter keeps all.
func filterTables(tables []string, filter string) []string {
	if filter == "" {
		return tables
	}
	needle := strings.ToLower(filter)
	out := tables[:0]
	for _, t := range tables {
		if strings.Contains(strings.ToLower(t), needle) {
			out = append(out, t)
		}
	}
	return out
}

func yesNo(nullable bool) string {
	if nullable {
		return "YES"
	}
	return "NO"
}

func defaultOf(def *string) string {
	if def == nil {
		return ""
	}
	return *def
}

func keyOf(primaryKey bool) string {
	if primaryKey {
		return "PRI"
	}
	return ""
}

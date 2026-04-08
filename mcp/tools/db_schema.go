package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

// HandleDBSchema explores the database schema.
func HandleDBSchema(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	summary := request.GetBool("summary", true)
	filter := request.GetString("filter", "")
	database := request.GetString("database", "")

	db, driver, err := openDB(database)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("database connection failed: %v", err)), nil
	}
	defer db.Close()

	tables, err := listTables(db, driver, filter)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("listing tables: %v", err)), nil
	}

	if len(tables) == 0 {
		return mcp.NewToolResultText("No tables found."), nil
	}

	var b strings.Builder
	b.WriteString("# Database Schema\n\n")

	for _, table := range tables {
		b.WriteString(fmt.Sprintf("## %s\n", table))

		cols, err := describeTable(db, driver, table, summary)
		if err != nil {
			b.WriteString(fmt.Sprintf("  Error: %v\n\n", err))
			continue
		}

		if summary {
			for _, col := range cols {
				b.WriteString(fmt.Sprintf("- %s %s\n", col.name, col.dataType))
			}
		} else {
			b.WriteString("| Column | Type | Nullable | Default | Key |\n")
			b.WriteString("|--------|------|----------|---------|-----|\n")
			for _, col := range cols {
				b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
					col.name, col.dataType, col.nullable, col.defaultVal, col.key))
			}
		}
		b.WriteString("\n")
	}

	return mcp.NewToolResultText(b.String()), nil
}

type columnInfo struct {
	name       string
	dataType   string
	nullable   string
	defaultVal string
	key        string
}

func listTables(db *sql.DB, driver, filter string) ([]string, error) {
	var query string
	switch driver {
	case "mysql":
		query = "SHOW TABLES"
	case "postgres":
		query = "SELECT tablename FROM pg_tables WHERE schemaname = 'public' ORDER BY tablename"
	case "sqlite", "sqlite3":
		query = "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if filter == "" || strings.Contains(strings.ToLower(name), strings.ToLower(filter)) {
			tables = append(tables, name)
		}
	}
	return tables, rows.Err()
}

func describeTable(db *sql.DB, driver, table string, summary bool) ([]columnInfo, error) {
	switch driver {
	case "mysql":
		return describeMysql(db, table, summary)
	case "postgres":
		return describePostgres(db, table, summary)
	case "sqlite", "sqlite3":
		return describeSqlite(db, table, summary)
	default:
		return nil, fmt.Errorf("unsupported driver: %s", driver)
	}
}

func describeMysql(db *sql.DB, table string, summary bool) ([]columnInfo, error) {
	rows, err := db.Query("DESCRIBE `" + table + "`")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var field, colType, null, key string
		var def, extra sql.NullString
		if err := rows.Scan(&field, &colType, &null, &key, &def, &extra); err != nil {
			return nil, err
		}
		col := columnInfo{
			name:     field,
			dataType: colType,
			nullable: null,
			key:      key,
		}
		if def.Valid {
			col.defaultVal = def.String
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func describePostgres(db *sql.DB, table string, summary bool) ([]columnInfo, error) {
	query := `SELECT column_name, data_type, is_nullable, column_default,
		CASE WHEN pk.column_name IS NOT NULL THEN 'PRI' ELSE '' END as key_type
		FROM information_schema.columns c
		LEFT JOIN (
			SELECT kcu.column_name
			FROM information_schema.table_constraints tc
			JOIN information_schema.key_column_usage kcu ON tc.constraint_name = kcu.constraint_name
			WHERE tc.table_name = $1 AND tc.constraint_type = 'PRIMARY KEY'
		) pk ON c.column_name = pk.column_name
		WHERE c.table_name = $1 AND c.table_schema = 'public'
		ORDER BY c.ordinal_position`

	rows, err := db.Query(query, table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var col columnInfo
		var def sql.NullString
		if err := rows.Scan(&col.name, &col.dataType, &col.nullable, &def, &col.key); err != nil {
			return nil, err
		}
		if def.Valid {
			col.defaultVal = def.String
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

func describeSqlite(db *sql.DB, table string, summary bool) ([]columnInfo, error) {
	rows, err := db.Query("PRAGMA table_info(`" + table + "`)")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []columnInfo
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull int
		var defVal sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &colType, &notNull, &defVal, &pk); err != nil {
			return nil, err
		}
		nullable := "YES"
		if notNull == 1 {
			nullable = "NO"
		}
		key := ""
		if pk > 0 {
			key = "PRI"
		}
		col := columnInfo{
			name:     name,
			dataType: colType,
			nullable: nullable,
			key:      key,
		}
		if defVal.Valid {
			col.defaultVal = defVal.String
		}
		cols = append(cols, col)
	}
	return cols, rows.Err()
}

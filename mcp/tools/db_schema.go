package tools

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/velocitykode/velocity-mcp/server"
	"github.com/velocitykode/velocity/orm"
	"github.com/velocitykode/velocity/orm/drivers"
)

// Detail levels for velocity_db_schema output.
const (
	detailColumns = "columns"
	detailFull    = "full"
)

// HandleDBSchema explores the database schema. Columns come from Velocity's
// ORM schema-introspection API (dialect SQL lives in the ORM grammars). The
// framework exposes no live index/constraint introspection, so full detail
// adds per-dialect catalog queries here: pg_indexes/pg_constraint for
// postgres, information_schema for mysql, and PRAGMAs for sqlite. Unknown
// drivers degrade with an explicit "unavailable" line rather than silently
// omitting the sections.
func HandleDBSchema(ctx context.Context, req *server.Request) (*server.Response, error) {
	detail := req.String("detail")
	var summary *bool
	if v, ok := req.BoolOK("summary"); ok {
		summary = &v
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

	level, err := resolveDetail(detail, summary, len(tables))
	if err != nil {
		return server.Error(err.Error()), nil
	}

	var b strings.Builder
	b.WriteString("# Database Schema\n\n")

	for _, table := range tables {
		writeTableSection(ctx, &b, manager, table, level)
	}

	return server.Text(b.String()), nil
}

// resolveDetail picks the output detail level. An explicit detail param wins,
// then the legacy summary flag (true = columns, false = full), then the
// default: full when exactly one table matched (one call must suffice for
// "explain this table"), columns for a multi-table listing so the
// whole-database dump stays lean.
func resolveDetail(detail string, summary *bool, tableCount int) (string, error) {
	switch detail {
	case detailColumns, detailFull:
		return detail, nil
	case "":
		// fall through to defaults
	default:
		return "", fmt.Errorf("invalid detail %q: must be %q or %q", detail, detailColumns, detailFull)
	}
	if summary != nil {
		if *summary {
			return detailColumns, nil
		}
		return detailFull, nil
	}
	if tableCount == 1 {
		return detailFull, nil
	}
	return detailColumns, nil
}

// writeTableSection renders one table at the requested detail level. Full
// detail adds the column table plus Indexes and Constraints sections.
func writeTableSection(ctx context.Context, b *strings.Builder, manager *orm.Manager, table, level string) {
	b.WriteString(fmt.Sprintf("## %s\n", table))

	cols, err := manager.DescribeTable(ctx, table)
	if err != nil {
		b.WriteString(fmt.Sprintf("  Error: %v\n\n", err))
		return
	}

	if level == detailColumns {
		for _, col := range cols {
			b.WriteString(fmt.Sprintf("- %s %s\n", col.Name, col.DataType))
		}
		b.WriteString("\n")
		return
	}

	b.WriteString("| Column | Type | Nullable | Default | Key |\n")
	b.WriteString("|--------|------|----------|---------|-----|\n")
	for _, col := range cols {
		b.WriteString(fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
			col.Name, col.DataType, yesNo(col.Nullable), defaultOf(col.Default), keyOf(col.PrimaryKey)))
	}
	b.WriteString("\n")

	writeIndexesAndConstraints(ctx, b, manager, manager.DriverName(), table, cols)
}

// writeIndexesAndConstraints appends the Indexes and Constraints sections for
// one table, dispatching on the connected driver. Drivers without an
// implementation get an explicit "unavailable" line - never a silent omission.
func writeIndexesAndConstraints(ctx context.Context, b *strings.Builder, manager *orm.Manager, driver, table string, cols []drivers.ColumnSchema) {
	var indexes, constraints []string
	var err error
	switch driver {
	case "postgres":
		indexes, constraints, err = postgresIndexesAndConstraints(ctx, manager, table)
	case "mysql":
		indexes, constraints, err = mysqlIndexesAndConstraints(ctx, manager, table)
	case "sqlite":
		indexes, constraints, err = sqliteIndexesAndConstraints(ctx, manager, table, cols)
	default:
		b.WriteString(fmt.Sprintf("Indexes: unavailable for driver %q\n", driver))
		b.WriteString(fmt.Sprintf("Constraints: unavailable for driver %q\n\n", driver))
		return
	}
	if err != nil {
		b.WriteString(fmt.Sprintf("Indexes/constraints error: %v\n\n", err))
		return
	}

	writeBulletSection(b, "Indexes", indexes)
	writeBulletSection(b, "Constraints", constraints)
}

// writeBulletSection renders a heading with one bullet per line, or an
// explicit "(none)" so an empty section is never mistaken for an omission.
func writeBulletSection(b *strings.Builder, heading string, lines []string) {
	b.WriteString(heading + ":\n")
	if len(lines) == 0 {
		b.WriteString("- (none)\n")
	}
	for _, line := range lines {
		b.WriteString("- " + line + "\n")
	}
	b.WriteString("\n")
}

// postgresIndexesAndConstraints reads pg_indexes for full index definitions
// (which carry UNIQUE and partial WHERE clauses) and pg_constraint for
// PK/unique/FK/check definitions via pg_get_constraintdef.
func postgresIndexesAndConstraints(ctx context.Context, manager *orm.Manager, table string) (indexes, constraints []string, err error) {
	indexes, err = collectPairs(ctx, manager,
		`SELECT indexname, indexdef FROM pg_indexes WHERE schemaname = current_schema() AND tablename = $1 ORDER BY indexname`,
		table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading pg_indexes: %w", err)
	}

	constraints, err = collectPairs(ctx, manager,
		`SELECT c.conname, pg_get_constraintdef(c.oid)
		 FROM pg_constraint c
		 JOIN pg_class t ON t.oid = c.conrelid
		 JOIN pg_namespace n ON n.oid = t.relnamespace
		 WHERE n.nspname = current_schema() AND t.relname = $1
		 ORDER BY c.conname`,
		table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading pg_constraint: %w", err)
	}
	return indexes, constraints, nil
}

// collectPairs runs a two-column (name, definition) query and formats each
// row as "name: definition".
func collectPairs(ctx context.Context, manager *orm.Manager, query string, args ...any) ([]string, error) {
	rows, err := manager.Raw(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []string
	for rows.Next() {
		var name, def string
		if err := rows.Scan(&name, &def); err != nil {
			return nil, err
		}
		out = append(out, fmt.Sprintf("%s: %s", name, def))
	}
	return out, rows.Err()
}

// mysqlIndexesAndConstraints reads information_schema.statistics for indexes
// (grouped into column lists with a UNIQUE flag) and table_constraints joined
// with key_column_usage for PK/unique/FK definitions.
func mysqlIndexesAndConstraints(ctx context.Context, manager *orm.Manager, table string) (indexes, constraints []string, err error) {
	rows, err := manager.Raw(ctx,
		`SELECT index_name,
		        GROUP_CONCAT(COALESCE(column_name, '<expr>') ORDER BY seq_in_index SEPARATOR ', '),
		        MIN(non_unique)
		 FROM information_schema.statistics
		 WHERE table_schema = DATABASE() AND table_name = ?
		 GROUP BY index_name
		 ORDER BY index_name`,
		table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading information_schema.statistics: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var name, columns string
		var nonUnique int
		if err := rows.Scan(&name, &columns, &nonUnique); err != nil {
			return nil, nil, err
		}
		line := fmt.Sprintf("%s: (%s)", name, columns)
		if nonUnique == 0 {
			line += " UNIQUE"
		}
		indexes = append(indexes, line)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	crows, err := manager.Raw(ctx,
		`SELECT tc.constraint_name, tc.constraint_type,
		        GROUP_CONCAT(kcu.column_name ORDER BY kcu.ordinal_position SEPARATOR ', '),
		        COALESCE(kcu.referenced_table_name, ''),
		        COALESCE(GROUP_CONCAT(kcu.referenced_column_name ORDER BY kcu.ordinal_position SEPARATOR ', '), '')
		 FROM information_schema.table_constraints tc
		 JOIN information_schema.key_column_usage kcu
		   ON kcu.constraint_schema = tc.constraint_schema
		  AND kcu.constraint_name = tc.constraint_name
		  AND kcu.table_name = tc.table_name
		 WHERE tc.table_schema = DATABASE() AND tc.table_name = ?
		 GROUP BY tc.constraint_name, tc.constraint_type, kcu.referenced_table_name
		 ORDER BY tc.constraint_name`,
		table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading information_schema.table_constraints: %w", err)
	}
	defer crows.Close()

	for crows.Next() {
		var name, ctype, columns, refTable, refColumns string
		if err := crows.Scan(&name, &ctype, &columns, &refTable, &refColumns); err != nil {
			return nil, nil, err
		}
		line := fmt.Sprintf("%s: %s (%s)", name, ctype, columns)
		if refTable != "" {
			line += fmt.Sprintf(" REFERENCES %s(%s)", refTable, refColumns)
		}
		constraints = append(constraints, line)
	}
	if err := crows.Err(); err != nil {
		return nil, nil, err
	}
	return indexes, constraints, nil
}

// sqliteIndexMeta is one row of PRAGMA index_list.
type sqliteIndexMeta struct {
	name    string
	unique  bool
	origin  string // "c" = CREATE INDEX, "u" = UNIQUE constraint, "pk" = PRIMARY KEY
	partial bool
}

// sqliteIndexesAndConstraints reads PRAGMA index_list/index_info for indexes
// and PRAGMA foreign_key_list for FKs. The PK constraint is derived from the
// already-introspected column schema and UNIQUE constraints from the indexes
// SQLite creates for them (origin "u"). PRAGMA takes no bind parameters, so
// the table name - which always comes from ListTables, never from raw client
// input - is embedded as a quoted identifier.
func sqliteIndexesAndConstraints(ctx context.Context, manager *orm.Manager, table string, cols []drivers.ColumnSchema) (indexes, constraints []string, err error) {
	metas, err := sqliteIndexList(ctx, manager, table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading index_list: %w", err)
	}

	indexColumns := make(map[string][]string, len(metas))
	for _, meta := range metas {
		columns, err := sqliteIndexColumns(ctx, manager, meta.name)
		if err != nil {
			return nil, nil, fmt.Errorf("reading index_info for %s: %w", meta.name, err)
		}
		indexColumns[meta.name] = columns

		line := fmt.Sprintf("%s: (%s)", meta.name, strings.Join(columns, ", "))
		if meta.unique {
			line += " UNIQUE"
		}
		if meta.partial {
			line += " PARTIAL"
		}
		indexes = append(indexes, line)
	}

	if pk := primaryKeyColumns(cols); len(pk) > 0 {
		constraints = append(constraints, fmt.Sprintf("PRIMARY KEY (%s)", strings.Join(pk, ", ")))
	}
	for _, meta := range metas {
		if meta.origin == "u" {
			constraints = append(constraints, fmt.Sprintf("UNIQUE (%s)", strings.Join(indexColumns[meta.name], ", ")))
		}
	}

	fks, err := sqliteForeignKeys(ctx, manager, table)
	if err != nil {
		return nil, nil, fmt.Errorf("reading foreign_key_list: %w", err)
	}
	constraints = append(constraints, fks...)

	return indexes, constraints, nil
}

func sqliteIndexList(ctx context.Context, manager *orm.Manager, table string) ([]sqliteIndexMeta, error) {
	rows, err := manager.Raw(ctx, "PRAGMA index_list("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []sqliteIndexMeta
	for rows.Next() {
		var seq, unique, partial int
		var name, origin string
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, err
		}
		metas = append(metas, sqliteIndexMeta{name: name, unique: unique != 0, origin: origin, partial: partial != 0})
	}
	return metas, rows.Err()
}

func sqliteIndexColumns(ctx context.Context, manager *orm.Manager, index string) ([]string, error) {
	rows, err := manager.Raw(ctx, "PRAGMA index_info("+quoteIdent(index)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var seqno, cid int
		var name sql.NullString // NULL for expression index members
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, err
		}
		if name.Valid {
			columns = append(columns, name.String)
		} else {
			columns = append(columns, "<expr>")
		}
	}
	return columns, rows.Err()
}

func sqliteForeignKeys(ctx context.Context, manager *orm.Manager, table string) ([]string, error) {
	rows, err := manager.Raw(ctx, "PRAGMA foreign_key_list("+quoteIdent(table)+")")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Composite FKs span multiple rows sharing an id; group columns by id in
	// row order.
	type fk struct {
		table string
		from  []string
		to    []string
	}
	var order []int
	fks := make(map[int]*fk)
	for rows.Next() {
		var id, seq int
		var refTable, from, onUpdate, onDelete, match string
		var to sql.NullString // NULL when referencing the implicit PK
		if err := rows.Scan(&id, &seq, &refTable, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		entry, ok := fks[id]
		if !ok {
			entry = &fk{table: refTable}
			fks[id] = entry
			order = append(order, id)
		}
		entry.from = append(entry.from, from)
		if to.Valid {
			entry.to = append(entry.to, to.String)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var out []string
	for _, id := range order {
		entry := fks[id]
		line := fmt.Sprintf("FOREIGN KEY (%s) REFERENCES %s", strings.Join(entry.from, ", "), entry.table)
		if len(entry.to) > 0 {
			line += fmt.Sprintf("(%s)", strings.Join(entry.to, ", "))
		}
		out = append(out, line)
	}
	return out, nil
}

// primaryKeyColumns returns the names of the primary-key columns in order.
func primaryKeyColumns(cols []drivers.ColumnSchema) []string {
	var pk []string
	for _, col := range cols {
		if col.PrimaryKey {
			pk = append(pk, col.Name)
		}
	}
	return pk
}

// quoteIdent double-quotes an identifier for embedding where bind parameters
// are not accepted (SQLite PRAGMAs).
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
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

package tools

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/velocitykode/velocity-mcp/content"
	"github.com/velocitykode/velocity-mcp/server"
	"github.com/velocitykode/velocity/orm"
	ormtesting "github.com/velocitykode/velocity/orm/testing"
)

func makeRequest(args map[string]any) *server.Request {
	return server.NewRequest(args)
}

func setupFixtureProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "go.mod"), []byte(`module testapp

go 1.26

require (
	github.com/velocitykode/velocity v0.20.3
	github.com/joho/godotenv v1.5.1
)
`), 0644)

	os.WriteFile(filepath.Join(dir, ".env"), []byte(`APP_NAME=TestApp
APP_ENV=testing
APP_DEBUG=true
PORT=4000
DB_CONNECTION=sqlite
DB_DATABASE=:memory:
DB_HOST=127.0.0.1
LOG_DRIVER=file
LOG_PATH=./storage/logs
`), 0644)

	routesDir := filepath.Join(dir, "routes")
	os.MkdirAll(routesDir, 0755)
	os.WriteFile(filepath.Join(routesDir, "web.go"), []byte(`package routes

func web(r router.Router) {
	r.Get("/", homeHandler)
	r.Post("/login", loginHandler)
}
`), 0644)

	logDir := filepath.Join(dir, "storage", "logs")
	os.MkdirAll(logDir, 0755)
	os.WriteFile(filepath.Join(logDir, "velocity-2026-04-08.log"), []byte(`[09:12:03] INFO: Server started on :4000
[09:13:01] WARN: Slow query | duration=450ms
[09:14:05] ERROR: Connection refused | host=smtp:587
[09:15:30] INFO: GET /dashboard | status=200
`), 0644)

	appDir := filepath.Join(dir, "app")
	os.MkdirAll(appDir, 0755)
	os.WriteFile(filepath.Join(appDir, "provider.go"), []byte(`package app

func setup(reg *ProviderRegistry) {
	reg.Add(&AuthProvider{})
}
`), 0644)

	return dir
}

func withWorkDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	old, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(old)
	fn()
}

// newTestManager creates an ORM manager with SQLite :memory: using the framework's own helpers.
func newTestManager(t *testing.T) *orm.Manager {
	t.Helper()
	manager, err := orm.NewManager(orm.ManagerConfig{
		Driver:   "sqlite",
		Database: ":memory:",
	})
	if err != nil {
		t.Fatalf("creating test ORM manager: %v", err)
	}
	t.Cleanup(func() { manager.Shutdown(context.Background()) })
	return manager
}

// --- HandleAppInfo ---

func TestHandleAppInfo(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, err := HandleAppInfo(context.Background(), makeRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.IsError() {
			t.Fatalf("tool error: %s", result.Contents()[0].(*content.Text).String())
		}

		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "testapp") {
			t.Error("should contain module name")
		}
		if !strings.Contains(text, "1.26") {
			t.Error("should contain Go version")
		}
		if !strings.Contains(text, "v0.20.3") {
			t.Error("should contain Velocity version")
		}
		if !strings.Contains(text, "AuthProvider") {
			t.Error("should contain detected provider")
		}
	})
}

func TestHandleAppInfo_NoGoMod(t *testing.T) {
	dir := t.TempDir()
	withWorkDir(t, dir, func() {
		result, _ := HandleAppInfo(context.Background(), makeRequest(nil))
		if !result.IsError() {
			t.Error("expected error for missing go.mod")
		}
	})
}

// --- HandleConfig ---

func TestHandleConfig_SpecificKey(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, err := HandleConfig(context.Background(), makeRequest(map[string]any{
			"key": "APP_ENV",
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "testing") {
			t.Errorf("expected APP_ENV=testing, got: %s", text)
		}
	})
}

func TestHandleConfig_SecretKey(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, ".env"), []byte("DB_PASSWORD=supersecret\n"), 0644)
	withWorkDir(t, dir, func() {
		result, _ := HandleConfig(context.Background(), makeRequest(map[string]any{
			"key": "DB_PASSWORD",
		}))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "REDACTED") {
			t.Error("secret key should be redacted")
		}
		if strings.Contains(text, "supersecret") {
			t.Error("secret value should not appear")
		}
	})
}

func TestHandleConfig_AllKeys(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, _ := HandleConfig(context.Background(), makeRequest(nil))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "# Configuration") {
			t.Error("should contain heading")
		}
		if !strings.Contains(text, "APP_NAME") {
			t.Error("should list APP_NAME")
		}
		if !strings.Contains(text, "TestApp") {
			t.Error("should show app name value")
		}
	})
}

func TestHandleConfig_MissingKey(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, _ := HandleConfig(context.Background(), makeRequest(map[string]any{
			"key": "NONEXISTENT_KEY",
		}))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "not found") {
			t.Errorf("expected 'not found', got: %s", text)
		}
	})
}

func TestHandleConfig_NoEnv(t *testing.T) {
	dir := t.TempDir()
	withWorkDir(t, dir, func() {
		result, _ := HandleConfig(context.Background(), makeRequest(map[string]any{
			"key": "APP_ENV",
		}))
		if !result.IsError() {
			t.Error("expected error for missing .env")
		}
	})
}

// --- HandleRoutes ---

func TestHandleRoutes(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, err := HandleRoutes(context.Background(), makeRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "GET") {
			t.Error("should contain GET route")
		}
		if !strings.Contains(text, "/login") {
			t.Error("should contain /login path")
		}
		if !strings.Contains(text, "2 routes") {
			t.Errorf("expected 2 routes, got: %s", text)
		}
	})
}

func TestHandleRoutes_EmptyProject(t *testing.T) {
	dir := t.TempDir()
	withWorkDir(t, dir, func() {
		result, _ := HandleRoutes(context.Background(), makeRequest(nil))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "No routes found") {
			t.Errorf("expected 'No routes found', got: %s", text)
		}
	})
}

// --- HandleSearchDocs ---

func TestHandleSearchDocs(t *testing.T) {
	result, err := HandleSearchDocs(context.Background(), makeRequest(map[string]any{
		"queries": []any{"velocity orm"},
	}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	text := result.Contents()[0].(*content.Text).String()
	if !strings.Contains(text, "Documentation Search Results") {
		t.Error("should contain results heading")
	}
}

func TestHandleSearchDocs_NoQueries(t *testing.T) {
	result, _ := HandleSearchDocs(context.Background(), makeRequest(nil))
	if !result.IsError() {
		t.Error("expected error for missing queries")
	}
}

func TestHandleSearchDocs_WithPackageFilter(t *testing.T) {
	result, _ := HandleSearchDocs(context.Background(), makeRequest(map[string]any{
		"queries":  []any{"models queries"},
		"packages": []any{"orm"},
	}))
	text := result.Contents()[0].(*content.Text).String()
	if strings.Contains(text, "Getting Started") {
		t.Error("package filter should exclude getting-started")
	}
}

func TestHandleSearchDocs_TokenLimit(t *testing.T) {
	result, _ := HandleSearchDocs(context.Background(), makeRequest(map[string]any{
		"queries":     []any{"velocity"},
		"token_limit": float64(10),
	}))
	text := result.Contents()[0].(*content.Text).String()
	if len(text) > 500 {
		t.Error("response should be limited by token_limit")
	}
}

// --- HandleLastError ---

func TestHandleLastError(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, err := HandleLastError(context.Background(), makeRequest(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "Connection refused") {
			t.Errorf("should find the error entry, got: %s", text)
		}
	})
}

func TestHandleLastError_NoLogs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "storage", "logs"), 0755)
	withWorkDir(t, dir, func() {
		result, _ := HandleLastError(context.Background(), makeRequest(nil))
		if !result.IsError() {
			t.Error("expected error for no log files")
		}
	})
}

// --- HandleLogEntries ---

func TestHandleLogEntries(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, err := HandleLogEntries(context.Background(), makeRequest(map[string]any{
			"entries": float64(2),
		}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "Last 2 Log Entries") {
			t.Errorf("should show last 2, got: %s", text)
		}
	})
}

func TestHandleLogEntries_Default(t *testing.T) {
	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		result, _ := HandleLogEntries(context.Background(), makeRequest(nil))
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "Log Entries") {
			t.Error("should contain log entries")
		}
	})
}

func TestHandleLogEntries_NoLogs(t *testing.T) {
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "storage", "logs"), 0755)
	withWorkDir(t, dir, func() {
		result, _ := HandleLogEntries(context.Background(), makeRequest(nil))
		if !result.IsError() {
			t.Error("expected error for no log files")
		}
	})
}

// --- HandleDBSchema + HandleDBQuery (using Velocity ORM testing helpers) ---

func TestHandleDBSchema_SQLite(t *testing.T) {
	manager := newTestManager(t)
	tc := ormtesting.Setup(t, manager)
	db := tc.DB()

	// Seed tables
	db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT, email TEXT)")
	db.Exec("CREATE TABLE posts (id INTEGER PRIMARY KEY, title TEXT, user_id INTEGER)")
	db.Exec("INSERT INTO users (name, email) VALUES ('Alice', 'alice@test.com')")

	// Point .env at the same in-memory DB won't work across connections.
	// Instead, test the lower-level functions directly with the live *sql.DB.

	// Test listTables
	tables, err := listTables(db, "sqlite", "")
	if err != nil {
		t.Fatalf("listTables error: %v", err)
	}
	if len(tables) < 2 {
		t.Errorf("expected at least 2 tables, got %d: %v", len(tables), tables)
	}
	hasUsers := false
	hasPosts := false
	for _, table := range tables {
		if table == "users" {
			hasUsers = true
		}
		if table == "posts" {
			hasPosts = true
		}
	}
	if !hasUsers {
		t.Error("should have users table")
	}
	if !hasPosts {
		t.Error("should have posts table")
	}

	// Test listTables with filter
	filtered, _ := listTables(db, "sqlite", "user")
	if len(filtered) != 1 || filtered[0] != "users" {
		t.Errorf("filter=user should return [users], got %v", filtered)
	}

	noMatch, _ := listTables(db, "sqlite", "zzz_nonexistent")
	if len(noMatch) != 0 {
		t.Errorf("filter=nonexistent should return empty, got %v", noMatch)
	}

	// Test describeSqlite summary
	cols, err := describeSqlite(db, "users", true)
	if err != nil {
		t.Fatalf("describeSqlite error: %v", err)
	}
	if len(cols) != 3 {
		t.Errorf("users should have 3 columns, got %d", len(cols))
	}
	if cols[0].name != "id" {
		t.Errorf("first column should be id, got %s", cols[0].name)
	}
	if cols[0].key != "PRI" {
		t.Errorf("id should be primary key, got key=%s", cols[0].key)
	}

	// Test describeSqlite detailed
	detailedCols, _ := describeSqlite(db, "users", false)
	if len(detailedCols) != 3 {
		t.Errorf("detailed should also have 3 columns, got %d", len(detailedCols))
	}

	// Test describeTable dispatch
	dispatchCols, err := describeTable(db, "sqlite", "users", true)
	if err != nil {
		t.Fatalf("describeTable dispatch error: %v", err)
	}
	if len(dispatchCols) != 3 {
		t.Error("describeTable should dispatch to describeSqlite")
	}

	// Test unsupported driver
	_, err = describeTable(db, "oracle", "users", true)
	if err == nil {
		t.Error("expected error for unsupported driver")
	}

	_, err = listTables(db, "oracle", "")
	if err == nil {
		t.Error("expected error for unsupported driver in listTables")
	}
}

func TestHandleDBQuery_WithTestManager(t *testing.T) {
	manager := newTestManager(t)
	tc := ormtesting.Setup(t, manager)
	db := tc.DB()

	db.Exec("CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT)")
	db.Exec("INSERT INTO users (name) VALUES ('Alice'), ('Bob')")

	// Test via manager.Raw (same as handler would use internally)
	rows, err := manager.Raw(context.Background(), "SELECT * FROM users ORDER BY name")
	if err != nil {
		t.Fatalf("raw query error: %v", err)
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var id int
		var name string
		rows.Scan(&id, &name)
		names = append(names, name)
	}
	if len(names) != 2 {
		t.Errorf("expected 2 rows, got %d", len(names))
	}
	if names[0] != "Alice" {
		t.Errorf("first name should be Alice, got %s", names[0])
	}
	if names[1] != "Bob" {
		t.Errorf("second name should be Bob, got %s", names[1])
	}
}

func TestHandleDBQuery_Forbidden(t *testing.T) {
	// These don't need a DB - they're blocked before execution
	forbidden := []string{
		"INSERT INTO users (name) VALUES ('Eve')",
		"UPDATE users SET name = 'hacked'",
		"DELETE FROM users WHERE id = 1",
		"DROP TABLE users",
		"ALTER TABLE users ADD COLUMN age INT",
		"CREATE TABLE evil (id INT)",
		"TRUNCATE TABLE users",
	}

	for _, q := range forbidden {
		result, _ := HandleDBQuery(context.Background(), makeRequest(map[string]any{
			"query": q,
		}))
		if !result.IsError() {
			t.Errorf("query %q should be blocked", q)
		}
		text := result.Contents()[0].(*content.Text).String()
		if !strings.Contains(text, "read-only") {
			t.Errorf("error for %q should mention read-only, got: %s", q, text)
		}
	}
}

func TestHandleDBQuery_MissingParam(t *testing.T) {
	result, _ := HandleDBQuery(context.Background(), makeRequest(nil))
	if !result.IsError() {
		t.Error("missing query param should error")
	}
}

func TestHandleDBSchema_DefaultConfig(t *testing.T) {
	// Without .env, velocity.ConfigFromEnv() uses defaults (sqlite, empty DB path).
	// It should either connect to a default sqlite or error gracefully.
	dir := t.TempDir()
	withWorkDir(t, dir, func() {
		result, _ := HandleDBSchema(context.Background(), makeRequest(map[string]any{
			"summary": true,
		}))
		// Should get a result (empty DB or error) - not panic
		if result == nil {
			t.Error("result should not be nil")
		}
	})
}

func TestHandleDBQuery_DefaultConfig(t *testing.T) {
	dir := t.TempDir()
	withWorkDir(t, dir, func() {
		result, _ := HandleDBQuery(context.Background(), makeRequest(map[string]any{
			"query": "SELECT 1",
		}))
		// Should get a result (success or error) - not panic
		if result == nil {
			t.Error("result should not be nil")
		}
	})
}

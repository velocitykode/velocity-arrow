package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/velocitykode/velocity-mcp/content"
)

// stubVelRunner replaces the vel CLI runner for the duration of a test.
func stubVelRunner(t *testing.T, fn func(ctx context.Context, args ...string) ([]byte, error)) {
	t.Helper()
	orig := velRunner
	velRunner = fn
	t.Cleanup(func() { velRunner = orig })
}

// velUnavailable simulates a machine without the vel CLI.
func velUnavailable(context.Context, ...string) ([]byte, error) {
	return nil, errors.New("vel: executable not found")
}

const routesJSONFixture = `[
  {"method": "GET", "path": "/users", "handler": "handlers.UserIndex", "middleware": ["auth"], "name": "users.index"},
  {"method": "POST", "path": "/users", "handler": "handlers.UserStore", "middleware": [], "name": "users.store"},
  {"method": "GET", "path": "/health", "handler": "handlers.Health", "middleware": [], "name": ""}
]`

// routesTextFixture mimics the prism table `vel routes` prints, including
// ANSI-styled header, separator, and footer.
const routesTextFixture = "\n" +
	"  \x1b[1mMethod \x1b[0m \x1b[1mPath     \x1b[0m \x1b[1mName         \x1b[0m\n" +
	"  \x1b[2m────────────────────────────────────\x1b[0m\n" +
	"  GET      /users     users.index   \n" +
	"  POST     /users     users.store   \n" +
	"  GET      /health                  \n" +
	"\n" +
	"Showing 3 rows\n"

func handlerText(t *testing.T, args map[string]any) string {
	t.Helper()
	result, err := HandleRoutes(context.Background(), makeRequest(args))
	if err != nil {
		t.Fatalf("HandleRoutes: %v", err)
	}
	return result.Contents()[0].(*content.Text).String()
}

func TestParseRoutesJSON(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int
		wantErr bool
	}{
		{name: "contract fixture", input: routesJSONFixture, want: 3},
		{name: "empty array", input: "[]", want: 0},
		{name: "human table not JSON", input: "  Method  Path  Name\n  GET  /users", wantErr: true},
		{name: "object not array", input: `{"method":"GET"}`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes, err := parseRoutesJSON([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(routes) != tt.want {
				t.Fatalf("routes count = %d, want %d", len(routes), tt.want)
			}
		})
	}
}

func TestParseRoutesJSON_Fields(t *testing.T) {
	routes, err := parseRoutesJSON([]byte(routesJSONFixture))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	first := routes[0]
	if first.Method != "GET" || first.Path != "/users" || first.Handler != "handlers.UserIndex" || first.Name != "users.index" {
		t.Errorf("first route = %+v, want GET /users handlers.UserIndex users.index", first)
	}
	if len(first.Middleware) != 1 || first.Middleware[0] != "auth" {
		t.Errorf("first route middleware = %v, want [auth]", first.Middleware)
	}
}

func TestParseRoutesText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []routeEntry
	}{
		{
			name:  "prism table with ANSI codes",
			input: routesTextFixture,
			want: []routeEntry{
				{Method: "GET", Path: "/users", Name: "users.index"},
				{Method: "POST", Path: "/users", Name: "users.store"},
				{Method: "GET", Path: "/health"},
			},
		},
		{
			name:  "empty output",
			input: "",
			want:  nil,
		},
		{
			name:  "no route rows",
			input: "No routes registered.\n",
			want:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			routes := parseRoutesText(tt.input)
			if len(routes) != len(tt.want) {
				t.Fatalf("routes count = %d, want %d", len(routes), len(tt.want))
			}
			for i, want := range tt.want {
				got := routes[i]
				if got.Method != want.Method || got.Path != want.Path || got.Name != want.Name {
					t.Errorf("routes[%d] = %+v, want %+v", i, got, want)
				}
			}
		})
	}
}

func TestFilterRoutes(t *testing.T) {
	routes := []routeEntry{
		{Method: "GET", Path: "/users", Handler: "handlers.UserIndex", Name: "users.index"},
		{Method: "POST", Path: "/users", Handler: "handlers.UserStore", Name: "users.store"},
		{Method: "GET", Path: "/health", Handler: "handlers.Health"},
	}

	tests := []struct {
		name string
		q    routeQuery
		want []string // expected paths, in order
	}{
		{name: "no filters", q: routeQuery{}, want: []string{"/users", "/users", "/health"}},
		{name: "method", q: routeQuery{method: "POST"}, want: []string{"/users"}},
		{name: "filter on path", q: routeQuery{filter: "health"}, want: []string{"/health"}},
		{name: "filter on handler case-insensitive", q: routeQuery{filter: "userstore"}, want: []string{"/users"}},
		{name: "filter on name", q: routeQuery{filter: "users.index"}, want: []string{"/users"}},
		{name: "filter and method", q: routeQuery{filter: "users", method: "GET"}, want: []string{"/users"}},
		{name: "no match", q: routeQuery{filter: "zzz"}, want: nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := filterRoutes(routes, tt.q)
			if len(got) != len(tt.want) {
				t.Fatalf("matched %d routes, want %d", len(got), len(tt.want))
			}
			for i, path := range tt.want {
				if got[i].Path != path {
					t.Errorf("matched[%d].Path = %q, want %q", i, got[i].Path, path)
				}
			}
		})
	}
}

func TestHandleRoutes_JSONSource(t *testing.T) {
	var gotArgs []string
	stubVelRunner(t, func(ctx context.Context, args ...string) ([]byte, error) {
		gotArgs = append([]string(nil), args...)
		return []byte(routesJSONFixture), nil
	})

	tests := []struct {
		name        string
		args        map[string]any
		contains    []string
		notContains []string
	}{
		{
			name:     "no params",
			args:     nil,
			contains: []string{"# Routes (source: json)", "Total: 3 routes", "/health", "auth"},
		},
		{
			name:        "filter trims but total stays",
			args:        map[string]any{"filter": "users"},
			contains:    []string{"Total: 3 routes, 2 matched", "users.index", "users.store"},
			notContains: []string{"/health"},
		},
		{
			name:        "method filter",
			args:        map[string]any{"method": "get"},
			contains:    []string{"Total: 3 routes, 2 matched", "/health"},
			notContains: []string{"users.store"},
		},
		{
			name:     "limit only",
			args:     map[string]any{"limit": 1},
			contains: []string{"Total: 3 routes, showing 1", "users.index"},
		},
		{
			name:        "filter method and limit combined",
			args:        map[string]any{"filter": "users", "method": "GET", "limit": 1},
			contains:    []string{"Total: 3 routes, 1 matched", "users.index"},
			notContains: []string{"users.store", "/health"},
		},
		{
			name:        "filter and limit truncate",
			args:        map[string]any{"filter": "users", "limit": 1},
			contains:    []string{"Total: 3 routes, 2 matched, showing 1", "users.index"},
			notContains: []string{"users.store"},
		},
		{
			name:     "no match still reports total",
			args:     map[string]any{"filter": "zzz"},
			contains: []string{"Total: 3 routes, 0 matched", "No routes matched"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text := handlerText(t, tt.args)
			for _, want := range tt.contains {
				if !strings.Contains(text, want) {
					t.Errorf("output missing %q:\n%s", want, text)
				}
			}
			for _, unwanted := range tt.notContains {
				if strings.Contains(text, unwanted) {
					t.Errorf("output should not contain %q:\n%s", unwanted, text)
				}
			}
		})
	}

	if len(gotArgs) != 2 || gotArgs[0] != "routes" || gotArgs[1] != "--json" {
		t.Errorf("vel invoked with %v, want [routes --json]", gotArgs)
	}
}

func TestHandleRoutes_TextFallback(t *testing.T) {
	tests := []struct {
		name     string
		jsonMode func() ([]byte, error) // what `vel routes --json` returns
	}{
		{
			name:     "old vel rejects the flag",
			jsonMode: func() ([]byte, error) { return nil, errors.New("unknown flag: --json") },
		},
		{
			name:     "old vel ignores the flag and prints the table",
			jsonMode: func() ([]byte, error) { return []byte(routesTextFixture), nil },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stubVelRunner(t, func(ctx context.Context, args ...string) ([]byte, error) {
				if len(args) == 2 && args[1] == "--json" {
					return tt.jsonMode()
				}
				return []byte(routesTextFixture), nil
			})

			text := handlerText(t, map[string]any{"method": "GET", "filter": "users"})
			for _, want := range []string{"# Routes (source: text)", "Total: 3 routes, 1 matched", "users.index"} {
				if !strings.Contains(text, want) {
					t.Errorf("output missing %q:\n%s", want, text)
				}
			}
			for _, unwanted := range []string{"users.store", "/health"} {
				if strings.Contains(text, unwanted) {
					t.Errorf("output should not contain %q:\n%s", unwanted, text)
				}
			}
		})
	}
}

func TestHandleRoutes_ASTFallbackHonoursParams(t *testing.T) {
	stubVelRunner(t, velUnavailable)

	dir := setupFixtureProject(t)
	withWorkDir(t, dir, func() {
		text := handlerText(t, map[string]any{"filter": "/login", "method": "POST"})
		for _, want := range []string{"# Routes (source: ast)", "Total: 2 routes, 1 matched", "/login"} {
			if !strings.Contains(text, want) {
				t.Errorf("output missing %q:\n%s", want, text)
			}
		}
		if strings.Contains(text, "homeHandler") {
			t.Errorf("output should not contain the filtered-out GET / route:\n%s", text)
		}
	})
}

func TestRunVel_Timeout(t *testing.T) {
	dir := t.TempDir()
	script := "#!/bin/sh\nsleep 5\n"
	if err := os.WriteFile(filepath.Join(dir, "vel"), []byte(script), 0755); err != nil {
		t.Fatalf("writing fake vel: %v", err)
	}
	t.Setenv("PATH", dir)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := runVel(ctx, "routes", "--json")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected error from timed-out vel, got nil")
	}
	if elapsed > 3*time.Second {
		t.Fatalf("runVel took %v; context timeout not honoured", elapsed)
	}
}

func TestParseRoutesFromFile_HTTPMethods(t *testing.T) {
	dir := t.TempDir()
	code := `package routes

func registerRoutes(r router.Router) {
	r.Get("/", homeHandler)
	r.Post("/login", authHandler)
	r.Put("/users/:id", updateUser)
	r.Delete("/users/:id", deleteUser)
	r.Patch("/profile", patchProfile)
}
`
	file := filepath.Join(dir, "routes.go")
	os.WriteFile(file, []byte(code), 0644)

	routes := parseRoutesFromFile(file)
	if len(routes) != 5 {
		t.Fatalf("routes count = %d, want 5", len(routes))
	}

	expected := []struct {
		method, path string
	}{
		{"GET", "/"},
		{"POST", "/login"},
		{"PUT", "/users/:id"},
		{"DELETE", "/users/:id"},
		{"PATCH", "/profile"},
	}

	for i, exp := range expected {
		if routes[i].Method != exp.method {
			t.Errorf("routes[%d].Method = %q, want %q", i, routes[i].Method, exp.method)
		}
		if routes[i].Path != exp.path {
			t.Errorf("routes[%d].Path = %q, want %q", i, routes[i].Path, exp.path)
		}
	}
}

func TestParseRoutesFromFile_Resource(t *testing.T) {
	dir := t.TempDir()
	code := `package routes

func registerRoutes(r router.Router) {
	r.Resource("/posts", &PostController{})
}
`
	file := filepath.Join(dir, "routes.go")
	os.WriteFile(file, []byte(code), 0644)

	routes := parseRoutesFromFile(file)
	if len(routes) != 7 {
		t.Fatalf("routes count = %d, want 7 (resource generates 7 routes)", len(routes))
	}

	// Verify first and last
	if routes[0].Method != "GET" || routes[0].Path != "/posts" {
		t.Errorf("first resource route = %s %s, want GET /posts", routes[0].Method, routes[0].Path)
	}
	if routes[6].Method != "DELETE" || routes[6].Path != "/posts/:id" {
		t.Errorf("last resource route = %s %s, want DELETE /posts/:id", routes[6].Method, routes[6].Path)
	}
}

func TestParseRoutesFromFile_Empty(t *testing.T) {
	dir := t.TempDir()
	code := `package routes

func init() {
	// no routes
}
`
	file := filepath.Join(dir, "routes.go")
	os.WriteFile(file, []byte(code), 0644)

	routes := parseRoutesFromFile(file)
	if len(routes) != 0 {
		t.Errorf("expected no routes, got %d", len(routes))
	}
}

func TestParseRoutesFromFile_InvalidFile(t *testing.T) {
	routes := parseRoutesFromFile("/nonexistent/file.go")
	if len(routes) != 0 {
		t.Errorf("expected no routes for missing file, got %d", len(routes))
	}
}

func TestScanRoutes_MultipleFiles(t *testing.T) {
	dir := t.TempDir()

	routesDir := filepath.Join(dir, "routes")
	os.MkdirAll(routesDir, 0755)

	web := `package routes
func web(r router.Router) {
	r.Get("/", homeHandler)
}
`
	api := `package routes
func api(r router.Router) {
	r.Get("/api/health", healthHandler)
	r.Post("/api/login", loginHandler)
}
`
	os.WriteFile(filepath.Join(routesDir, "web.go"), []byte(web), 0644)
	os.WriteFile(filepath.Join(routesDir, "api.go"), []byte(api), 0644)

	routes := scanRoutes(dir)
	if len(routes) != 3 {
		t.Errorf("routes count = %d, want 3", len(routes))
	}
}

func TestScanRoutes_NestedDirs(t *testing.T) {
	dir := t.TempDir()

	webDir := filepath.Join(dir, "routes", "web")
	infraDir := filepath.Join(dir, "routes", "infra")
	os.MkdirAll(webDir, 0755)
	os.MkdirAll(infraDir, 0755)

	web := `package web
func Register(r router.Router) {
	r.Get("/projects", h.Index)
	r.Post("/projects", h.Store)
}
`
	infra := `package infra
func Register(r router.Router) {
	r.Get("/healthz", healthz)
}
`
	webTest := `package web
func TestNothing(t *testing.T) {}
`
	os.WriteFile(filepath.Join(webDir, "projects.go"), []byte(web), 0644)
	os.WriteFile(filepath.Join(infraDir, "health.go"), []byte(infra), 0644)
	os.WriteFile(filepath.Join(webDir, "projects_test.go"), []byte(webTest), 0644)

	routes := scanRoutes(dir)
	if len(routes) != 3 {
		t.Errorf("routes count = %d, want 3 (nested dirs walked, _test.go skipped)", len(routes))
	}
}

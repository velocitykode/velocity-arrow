package tools

import (
	"os"
	"path/filepath"
	"testing"
)

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

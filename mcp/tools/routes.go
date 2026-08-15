package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/velocitykode/velocity-mcp/server"
)

// Route sources, surfaced in the response header. The paths disagree on
// fidelity: json and text come from the running app via the vel CLI, ast is
// static analysis (which, e.g., expands Resource calls to 7 routes itself).
const (
	sourceJSON = "json"
	sourceText = "text"
	sourceAST  = "ast"
)

// velTimeout bounds every vel CLI invocation so a hung binary cannot hang the
// tool call.
const velTimeout = 10 * time.Second

// velRunner executes the vel CLI; a package variable so tests can inject a
// fake runner.
var velRunner = runVel

type routeEntry struct {
	Method     string
	Path       string
	Handler    string
	Middleware []string
	Name       string
}

// routeQuery holds the filtering parameters of a velocity_routes call.
type routeQuery struct {
	filter string // case-insensitive substring on path/handler/name
	method string // exact HTTP method match, normalized upper-case
	limit  int    // max routes rendered; 0 = all
}

// HandleRoutes lists registered routes. Prefers `vel routes --json` (stable
// machine contract since velocity v0.73.3), then the human `vel routes` table,
// then AST parsing. All sources honour the filter/method/limit params, and the
// response always carries the total route count.
func HandleRoutes(ctx context.Context, req *server.Request) (*server.Response, error) {
	q := routeQuery{
		filter: req.String("filter"),
		method: strings.ToUpper(strings.TrimSpace(req.String("method"))),
	}
	if v, ok := req.IntOK("limit"); ok && v > 0 {
		q.limit = int(v)
	}

	routes, source := loadRoutes(ctx)
	if len(routes) == 0 {
		if source == sourceAST {
			return server.Text("No routes found. Install `vel` CLI for accurate route listing."), nil
		}
		return server.Text("No routes registered."), nil
	}

	return server.Text(renderRoutes(routes, q, source)), nil
}

// loadRoutes returns the full route table and the source that produced it:
// `vel routes --json` when the installed vel supports it, the text table
// otherwise, AST scanning as the last resort.
func loadRoutes(ctx context.Context) ([]routeEntry, string) {
	// vel >= 0.73.3: routes --json emits a stable array of {method, path,
	// handler, middleware, name} on stdout (bootstrap logs go to stderr).
	// Older vels reject the flag or print the table; both fall through.
	if out, err := velRunner(ctx, "routes", "--json"); err == nil {
		if routes, jsonErr := parseRoutesJSON(out); jsonErr == nil {
			return routes, sourceJSON
		}
	}

	if out, err := velRunner(ctx, "routes"); err == nil {
		if routes := parseRoutesText(string(out)); len(routes) > 0 {
			return routes, sourceText
		}
	}

	dir, err := os.Getwd()
	if err != nil {
		return nil, sourceAST
	}
	return scanRoutes(dir), sourceAST
}

// runVel executes the vel CLI under a context timeout so a hung binary cannot
// stall the tool call.
func runVel(ctx context.Context, args ...string) ([]byte, error) {
	velPath, err := exec.LookPath("vel")
	if err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(ctx, velTimeout)
	defer cancel()

	return exec.CommandContext(ctx, velPath, args...).Output()
}

// parseRoutesJSON decodes the `vel routes --json` contract: an array of
// {method, path, handler, middleware, name} objects, middleware always a
// non-null array.
func parseRoutesJSON(out []byte) ([]routeEntry, error) {
	var decoded []struct {
		Method     string   `json:"method"`
		Path       string   `json:"path"`
		Handler    string   `json:"handler"`
		Middleware []string `json:"middleware"`
		Name       string   `json:"name"`
	}
	if err := json.Unmarshal(out, &decoded); err != nil {
		return nil, err
	}

	routes := make([]routeEntry, 0, len(decoded))
	for _, d := range decoded {
		routes = append(routes, routeEntry{
			Method:     d.Method,
			Path:       d.Path,
			Handler:    d.Handler,
			Middleware: d.Middleware,
			Name:       d.Name,
		})
	}
	return routes, nil
}

var (
	ansiPattern      = regexp.MustCompile(`\x1b\[[0-9;]*m`)
	columnGapPattern = regexp.MustCompile(`\s{2,}`)
)

// parseRoutesText parses the human `vel routes` table (columns: Method, Path,
// Name). Rows are recognized by their leading HTTP method, which skips the
// header, separator, and "Showing N rows" footer for free.
func parseRoutesText(out string) []routeEntry {
	methods := make(map[string]bool, len(httpMethods))
	for _, m := range httpMethods {
		methods[m] = true
	}

	var routes []routeEntry
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(ansiPattern.ReplaceAllString(line, ""))
		fields := columnGapPattern.Split(line, -1)
		if len(fields) < 2 || !methods[fields[0]] {
			continue
		}
		entry := routeEntry{Method: fields[0], Path: fields[1]}
		if len(fields) > 2 {
			entry.Name = fields[2]
		}
		routes = append(routes, entry)
	}
	return routes
}

// --- filtering and rendering ---

// filterRoutes applies method and substring filters; limit is applied by the
// renderer so the matched count stays visible.
func filterRoutes(routes []routeEntry, q routeQuery) []routeEntry {
	out := make([]routeEntry, 0, len(routes))
	for _, r := range routes {
		if q.method != "" && r.Method != q.method {
			continue
		}
		if q.filter != "" && !matchesFilter(r, q.filter) {
			continue
		}
		out = append(out, r)
	}
	return out
}

// matchesFilter reports whether the filter is a case-insensitive substring of
// the route's path, handler, or name.
func matchesFilter(r routeEntry, filter string) bool {
	needle := strings.ToLower(filter)
	for _, hay := range []string{r.Path, r.Handler, r.Name} {
		if hay != "" && strings.Contains(strings.ToLower(hay), needle) {
			return true
		}
	}
	return false
}

// renderRoutes renders the filtered route table. The header always tags the
// source and the total route count, even when params trim the slice.
func renderRoutes(all []routeEntry, q routeQuery, source string) string {
	total := len(all)
	matched := filterRoutes(all, q)
	shown := matched
	if q.limit > 0 && len(shown) > q.limit {
		shown = shown[:q.limit]
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("# Routes (source: %s)\n\n", source))
	if source == sourceAST {
		b.WriteString("From static analysis - install `vel` CLI for accurate results.\n\n")
	}

	b.WriteString(fmt.Sprintf("Total: %d routes", total))
	if len(matched) != total {
		b.WriteString(fmt.Sprintf(", %d matched", len(matched)))
	}
	if len(shown) != len(matched) {
		b.WriteString(fmt.Sprintf(", showing %d", len(shown)))
	}
	b.WriteString("\n\n")

	if len(shown) == 0 {
		b.WriteString("No routes matched the given filters.\n")
		return b.String()
	}

	// Only sources that carry a column get to print it: json has all five,
	// text has name, ast has handler.
	hasHandler, hasName, hasMiddleware := false, false, false
	for _, r := range shown {
		hasHandler = hasHandler || r.Handler != ""
		hasName = hasName || r.Name != ""
		hasMiddleware = hasMiddleware || len(r.Middleware) > 0
	}

	headers := []string{"Method", "Path"}
	if hasHandler {
		headers = append(headers, "Handler")
	}
	if hasName {
		headers = append(headers, "Name")
	}
	if hasMiddleware {
		headers = append(headers, "Middleware")
	}

	b.WriteString("| " + strings.Join(headers, " | ") + " |\n")
	b.WriteString("|" + strings.Repeat("--------|", len(headers)) + "\n")

	for _, r := range shown {
		cells := []string{r.Method, r.Path}
		if hasHandler {
			cells = append(cells, r.Handler)
		}
		if hasName {
			cells = append(cells, r.Name)
		}
		if hasMiddleware {
			cells = append(cells, strings.Join(r.Middleware, ", "))
		}
		b.WriteString("| " + strings.Join(cells, " | ") + " |\n")
	}

	return b.String()
}

// --- AST fallback ---

func scanRoutes(dir string) []routeEntry {
	var routes []routeEntry

	// routes/ may nest arbitrarily (routes/web, routes/infra, ...), so walk it.
	var files []string
	_ = filepath.WalkDir(filepath.Join(dir, "routes"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() && strings.HasSuffix(path, ".go") && !strings.HasSuffix(path, "_test.go") {
			files = append(files, path)
		}
		return nil
	})

	patterns := []string{
		filepath.Join(dir, "app", "routes.go"),
		filepath.Join(dir, "app", "routes", "*.go"),
		filepath.Join(dir, "main.go"),
	}

	for _, pattern := range patterns {
		matches, _ := filepath.Glob(pattern)
		files = append(files, matches...)
	}

	for _, file := range files {
		found := parseRoutesFromFile(file)
		routes = append(routes, found...)
	}

	return routes
}

var httpMethods = map[string]string{
	"Get":     "GET",
	"Post":    "POST",
	"Put":     "PUT",
	"Delete":  "DELETE",
	"Patch":   "PATCH",
	"Options": "OPTIONS",
	"Head":    "HEAD",
	"Any":     "ANY",
}

func parseRoutesFromFile(filename string) []routeEntry {
	var routes []routeEntry

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil
	}

	ast.Inspect(f, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		methodName := sel.Sel.Name

		if httpMethod, isRoute := httpMethods[methodName]; isRoute {
			if len(call.Args) >= 2 {
				path := extractStringLit(call.Args[0])
				handler := extractHandlerName(call.Args[1])
				routes = append(routes, routeEntry{
					Method:  httpMethod,
					Path:    path,
					Handler: handler,
				})
			}
			return true
		}

		if methodName == "Resource" && len(call.Args) >= 2 {
			path := extractStringLit(call.Args[0])
			controller := extractHandlerName(call.Args[1])
			for _, m := range []struct{ method, suffix, action string }{
				{"GET", "", "Index"},
				{"GET", "/create", "Create"},
				{"POST", "", "Store"},
				{"GET", "/:id", "Show"},
				{"GET", "/:id/edit", "Edit"},
				{"PUT", "/:id", "Update"},
				{"DELETE", "/:id", "Destroy"},
			} {
				routes = append(routes, routeEntry{
					Method:  m.method,
					Path:    path + m.suffix,
					Handler: controller + "." + m.action,
				})
			}
		}

		return true
	})

	return routes
}

func extractStringLit(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		return strings.Trim(lit.Value, `"`)
	}
	return "<dynamic>"
}

func extractHandlerName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.SelectorExpr:
		if x, ok := v.X.(*ast.Ident); ok {
			return x.Name + "." + v.Sel.Name
		}
	case *ast.FuncLit:
		return "<closure>"
	case *ast.UnaryExpr:
		if comp, ok := v.X.(*ast.CompositeLit); ok {
			return typeName(comp.Type)
		}
	case *ast.CompositeLit:
		return typeName(v.Type)
	case *ast.CallExpr:
		return extractHandlerName(v.Fun)
	}
	return "<unknown>"
}

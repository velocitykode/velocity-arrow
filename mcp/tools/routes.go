package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

type routeEntry struct {
	Method     string
	Path       string
	Handler    string
	Middleware []string
}

// HandleRoutes lists registered routes. Prefers `vel routes` CLI, falls back to AST parsing.
func HandleRoutes(ctx context.Context, request mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	// Try vel CLI first
	if output, err := tryVelRoutes(); err == nil {
		return mcp.NewToolResultText(fmt.Sprintf("# Routes\n\n%s", output)), nil
	}

	// Fall back to AST parsing
	dir, err := os.Getwd()
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("getting working directory: %v", err)), nil
	}

	routes := scanRoutes(dir)

	if len(routes) == 0 {
		return mcp.NewToolResultText("No routes found. Install `vel` CLI for accurate route listing."), nil
	}

	var b strings.Builder
	b.WriteString("# Routes (from static analysis - install `vel` CLI for accurate results)\n\n")
	b.WriteString("| Method | Path | Handler |\n")
	b.WriteString("|--------|------|---------|\n")

	for _, r := range routes {
		b.WriteString(fmt.Sprintf("| %s | %s | %s |\n", r.Method, r.Path, r.Handler))
	}

	b.WriteString(fmt.Sprintf("\nTotal: %d routes\n", len(routes)))
	return mcp.NewToolResultText(b.String()), nil
}

// tryVelRoutes shells out to `vel routes` if the CLI is installed.
func tryVelRoutes() (string, error) {
	velPath, err := exec.LookPath("vel")
	if err != nil {
		return "", err
	}

	cmd := exec.Command(velPath, "routes")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// --- AST fallback ---

func scanRoutes(dir string) []routeEntry {
	var routes []routeEntry

	patterns := []string{
		filepath.Join(dir, "routes", "*.go"),
		filepath.Join(dir, "app", "routes.go"),
		filepath.Join(dir, "app", "routes", "*.go"),
		filepath.Join(dir, "main.go"),
	}

	var files []string
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

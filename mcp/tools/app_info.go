package tools

import (
	"context"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/velocitykode/velocity-mcp/server"
)

// HandleAppInfo returns Velocity application info by parsing go.mod and provider files.
func HandleAppInfo(ctx context.Context, req *server.Request) (*server.Response, error) {
	dir, err := os.Getwd()
	if err != nil {
		return server.Error(fmt.Sprintf("getting working directory: %v", err)), nil
	}

	goMod, err := parseGoMod(dir)
	if err != nil {
		return server.Error(fmt.Sprintf("parsing go.mod: %v", err)), nil
	}

	providers := scanProviders(dir)

	var b strings.Builder
	b.WriteString("# Velocity Application Info\n\n")

	b.WriteString("## Module\n")
	b.WriteString(fmt.Sprintf("- Module: %s\n", goMod.module))
	b.WriteString(fmt.Sprintf("- Go version: %s\n", goMod.goVersion))

	// Find Velocity version
	velVersion := "not found"
	for _, dep := range goMod.deps {
		if strings.Contains(dep.path, "velocitykode/velocity") && !strings.Contains(dep.path, "/") ||
			dep.path == "github.com/velocitykode/velocity" {
			velVersion = dep.version
			break
		}
	}
	b.WriteString(fmt.Sprintf("- Velocity version: %s\n", velVersion))

	b.WriteString("\n## Dependencies\n")
	for _, dep := range goMod.deps {
		b.WriteString(fmt.Sprintf("- %s %s\n", dep.path, dep.version))
	}

	if len(providers) > 0 {
		b.WriteString("\n## Registered Providers\n")
		for _, p := range providers {
			b.WriteString(fmt.Sprintf("- %s\n", p))
		}
	}

	return server.Text(b.String()), nil
}

type goModInfo struct {
	module    string
	goVersion string
	deps      []dependency
}

type dependency struct {
	path    string
	version string
}

func parseGoMod(dir string) (*goModInfo, error) {
	data, err := os.ReadFile(filepath.Join(dir, "go.mod"))
	if err != nil {
		return nil, fmt.Errorf("reading go.mod: %w", err)
	}

	info := &goModInfo{}
	lines := strings.Split(string(data), "\n")
	inRequire := false

	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "module ") {
			info.module = strings.TrimPrefix(line, "module ")
			continue
		}

		if strings.HasPrefix(line, "go ") {
			info.goVersion = strings.TrimPrefix(line, "go ")
			continue
		}

		if line == "require (" {
			inRequire = true
			continue
		}

		if line == ")" {
			inRequire = false
			continue
		}

		if inRequire {
			parts := strings.Fields(line)
			if len(parts) >= 2 && !strings.HasPrefix(parts[0], "//") {
				info.deps = append(info.deps, dependency{
					path:    parts[0],
					version: parts[1],
				})
			}
		}

		// Single-line require
		if strings.HasPrefix(line, "require ") && !strings.HasSuffix(line, "(") {
			parts := strings.Fields(strings.TrimPrefix(line, "require "))
			if len(parts) >= 2 {
				info.deps = append(info.deps, dependency{
					path:    parts[0],
					version: parts[1],
				})
			}
		}
	}

	return info, nil
}

// scanProviders looks for provider registration patterns in Go files.
func scanProviders(dir string) []string {
	var providers []string

	// Look for common provider registration patterns
	patterns := []string{
		filepath.Join(dir, "app", "*.go"),
		filepath.Join(dir, "cmd", "*.go"),
		filepath.Join(dir, "main.go"),
	}

	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, file := range matches {
			found := findProvidersInFile(file)
			providers = append(providers, found...)
		}
	}

	return providers
}

func findProvidersInFile(filename string) []string {
	var providers []string

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, filename, nil, parser.ParseComments)
	if err != nil {
		return nil
	}

	ast.Inspect(f, func(n ast.Node) bool {
		// Look for calls like reg.Add(...) or WithProviders(...)
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}

		funcName := callFuncName(call)
		if funcName != "Add" && funcName != "WithProviders" {
			return true
		}

		for _, arg := range call.Args {
			// Look for &SomeProvider{} or SomeProvider{}
			switch v := arg.(type) {
			case *ast.UnaryExpr:
				if comp, ok := v.X.(*ast.CompositeLit); ok {
					if name := typeName(comp.Type); name != "" {
						providers = append(providers, name)
					}
				}
			case *ast.CompositeLit:
				if name := typeName(v.Type); name != "" {
					providers = append(providers, name)
				}
			}
		}

		return true
	})

	return providers
}

func callFuncName(call *ast.CallExpr) string {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name
	case *ast.SelectorExpr:
		return fn.Sel.Name
	}
	return ""
}

func typeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		if x, ok := t.X.(*ast.Ident); ok {
			return x.Name + "." + t.Sel.Name
		}
	}
	return ""
}

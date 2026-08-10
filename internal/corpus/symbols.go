package corpus

import (
	"bytes"
	"context"
	"go/ast"
	"go/doc"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

// symbolWalk drives the package-by-package extraction so Symbols stays small.
type symbolWalk struct {
	root    string
	version string
	out     []kb.Entry
}

// Symbols implements the documented contract: parse exported declarations from
// the velocity source tree at velocityRoot into KindSymbol entries.
func Symbols(ctx context.Context, velocityRoot, version string) ([]kb.Entry, error) {
	w := &symbolWalk{root: velocityRoot, version: version}

	// Collect candidate package directories first, then parse each in isolation
	// so a single bad package cannot abort the whole walk.
	dirs, err := packageDirs(velocityRoot)
	if err != nil {
		return nil, err
	}

	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return w.out, err
		}
		w.collectDir(dir)
	}
	return w.out, nil
}

// skipDir reports whether a directory must not be descended into.
func skipDir(name string) bool {
	if name == "vendor" || name == "testdata" {
		return true
	}
	// Hidden directories (.git, .github, ...).
	if strings.HasPrefix(name, ".") && name != "." && name != ".." {
		return true
	}
	return false
}

// skipPackagePath reports whether a directory (relative to root) is a heavy
// driver leaf that the public-surface snapshot deliberately ignores.
func skipPackagePath(rel string) bool {
	rel = filepath.ToSlash(rel)
	heavy := []string{"cache/redis", "storage/s3"}
	for _, h := range heavy {
		if rel == h || strings.HasSuffix(rel, "/"+h) || strings.Contains(rel, "/"+h+"/") || strings.HasPrefix(rel, h+"/") {
			return true
		}
	}
	return false
}

// packageDirs returns every directory under root that should be parsed.
func packageDirs(root string) ([]string, error) {
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Tolerate unreadable entries: skip them, keep walking.
			if d != nil && d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if path == root {
			return nil
		}
		if skipDir(d.Name()) {
			return filepath.SkipDir
		}
		rel, rerr := filepath.Rel(root, path)
		if rerr == nil && skipPackagePath(rel) {
			return filepath.SkipDir
		}
		dirs = append(dirs, path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	// Include root itself as a candidate package directory.
	dirs = append([]string{root}, dirs...)
	return dirs, nil
}

// collectDir parses the Go package in a single directory and appends entries.
// Parse failures for individual files are swallowed so the walk continues. Files
// are parsed one at a time and grouped by package name, then handed to go/doc;
// this avoids the deprecated whole-directory parse and ast.Package paths.
func (w *symbolWalk) collectDir(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	fset := token.NewFileSet()
	byPkg := map[string][]*ast.File{}
	for _, de := range entries {
		if de.IsDir() {
			continue
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, perr := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.ParseComments)
		if perr != nil || f == nil {
			continue
		}
		pkgName := f.Name.Name
		byPkg[pkgName] = append(byPkg[pkgName], f)
	}
	if len(byPkg) == 0 {
		return
	}

	rel, rerr := filepath.Rel(w.root, dir)
	if rerr != nil {
		rel = dir
	}
	if rel == "." {
		rel = ""
	}
	pkgPath := filepath.ToSlash(rel)

	for name, files := range byPkg {
		// Skip any *_test external test package.
		if strings.HasSuffix(name, "_test") {
			continue
		}
		w.collectPackage(fset, files, pkgPath)
	}
}

// collectPackage extracts exported funcs, methods and types from one package's
// parsed files. go/doc surfaces only exported objects in default mode and gives
// tidy first-sentence synopses for the body.
func (w *symbolWalk) collectPackage(fset *token.FileSet, files []*ast.File, pkgPath string) {
	importPath := pkgPath
	if importPath == "" {
		importPath = "."
	}
	d, err := doc.NewFromFiles(fset, files, importPath)
	if err != nil || d == nil {
		return
	}

	for _, t := range d.Types {
		w.addType(fset, d, t, pkgPath)
		for _, f := range t.Funcs {
			// Constructor-style functions associated with the type.
			w.addFunc(fset, d, f, pkgPath, "")
		}
		for _, m := range t.Methods {
			w.addFunc(fset, d, m, pkgPath, t.Name)
		}
	}
	for _, f := range d.Funcs {
		w.addFunc(fset, d, f, pkgPath, "")
	}
}

// addType appends a KindSymbol entry for an exported type declaration.
func (w *symbolWalk) addType(fset *token.FileSet, d *doc.Package, t *doc.Type, pkgPath string) {
	if t.Decl == nil || len(t.Decl.Specs) == 0 {
		return
	}
	sig := renderNode(fset, t.Decl)
	w.out = append(w.out, kb.Entry{
		Kind:      kb.KindSymbol,
		Title:     t.Name,
		Signature: sig,
		Package:   pkgPath,
		Body:      synopsis(d, t.Doc),
		Ref:       refFor(fset, t.Decl.Pos(), w.root),
		Version:   w.version,
	})
}

// addFunc appends a KindSymbol entry for a func or method. recvType is the
// receiver type name for methods, "" for plain functions.
func (w *symbolWalk) addFunc(fset *token.FileSet, d *doc.Package, f *doc.Func, pkgPath, recvType string) {
	if f.Decl == nil {
		return
	}
	title := f.Name
	if recvType != "" {
		title = recvType + "." + f.Name
	}
	w.out = append(w.out, kb.Entry{
		Kind:      kb.KindSymbol,
		Title:     title,
		Signature: renderFuncSignature(fset, f.Decl),
		Package:   pkgPath,
		Body:      synopsis(d, f.Doc),
		Ref:       refFor(fset, f.Decl.Pos(), w.root),
		Version:   w.version,
	})
}

// renderFuncSignature prints a func declaration with its body stripped, so only
// the signature (receiver, name, params, results) is emitted.
func renderFuncSignature(fset *token.FileSet, decl *ast.FuncDecl) string {
	// Shallow-copy so we do not mutate the shared AST when nilling the body.
	stripped := *decl
	stripped.Body = nil
	stripped.Doc = nil
	return renderNode(fset, &stripped)
}

// renderNode pretty-prints any AST node using go/printer.
func renderNode(fset *token.FileSet, node ast.Node) string {
	var buf bytes.Buffer
	cfg := printer.Config{Mode: printer.UseSpaces | printer.TabIndent, Tabwidth: 8}
	if err := cfg.Fprint(&buf, fset, node); err != nil {
		return ""
	}
	return strings.TrimSpace(buf.String())
}

// refFor builds a "relpath:line" provenance string relative to root.
func refFor(fset *token.FileSet, pos token.Pos, root string) string {
	p := fset.Position(pos)
	rel, err := filepath.Rel(root, p.Filename)
	if err != nil {
		rel = p.Filename
	}
	rel = filepath.ToSlash(rel)
	return rel + ":" + itoa(p.Line)
}

// synopsis returns the first sentence of a doc comment, trimmed. It uses the
// package's Synopsis so doc links are handled correctly.
func synopsis(d *doc.Package, text string) string {
	text = strings.TrimSpace(text)
	if text == "" {
		return ""
	}
	return strings.TrimSpace(d.Synopsis(text))
}

// itoa avoids pulling strconv just for a positive line number.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

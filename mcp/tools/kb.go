package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-mcp/server"
)

// kbDefaultLimit is the result count kb-search returns when the caller does not
// ask for one.
const kbDefaultLimit = 5

// NewKBSearchHandler builds the kb-search handler: hybrid keyword plus semantic
// retrieval over the whole knowledge-base snapshot for an intent query.
func NewKBSearchHandler(s *store.Store) func(context.Context, *server.Request) (*server.Response, error) {
	return func(ctx context.Context, req *server.Request) (*server.Response, error) {
		limit := int(req.Int("limit"))
		if limit <= 0 {
			limit = kbDefaultLimit
		}
		results, err := s.Search(ctx, req.String("query"), kb.Kind(req.String("kind")), limit)
		if err != nil {
			return server.Error(fmt.Sprintf("search failed: %v", err)), nil
		}
		if len(results) == 0 {
			return server.Text("No matching entries in the knowledge base. " +
				"Absence here does NOT mean the framework lacks it; verify against source."), nil
		}
		return server.Text(formatKBResults(results)), nil
	}
}

// NewKBSymbolHandler builds the kb-symbol handler: exact API lookup that grounds
// a signature instead of letting the caller guess one.
func NewKBSymbolHandler(s *store.Store) func(context.Context, *server.Request) (*server.Response, error) {
	return func(ctx context.Context, req *server.Request) (*server.Response, error) {
		entries, err := s.Symbol(ctx, req.String("name"))
		if err != nil {
			return server.Error(fmt.Sprintf("symbol lookup failed: %v", err)), nil
		}
		if len(entries) == 0 {
			return server.Text("No such symbol in the knowledge base. " +
				"Absence here does NOT mean it is absent from the framework; verify against source."), nil
		}
		return server.Text(formatKBEntries(entries)), nil
	}
}

// NewKBGuardHandler builds the kb-guard handler: curated negative knowledge for
// a topic or package, the "use velocity X not stdlib Y" map and known gotchas.
func NewKBGuardHandler(s *store.Store) func(context.Context, *server.Request) (*server.Response, error) {
	return func(ctx context.Context, req *server.Request) (*server.Response, error) {
		entries, err := s.Guards(ctx, req.String("topic"))
		if err != nil {
			return server.Error(fmt.Sprintf("guard lookup failed: %v", err)), nil
		}
		if len(entries) == 0 {
			return server.Text("No guards recorded for that topic."), nil
		}
		return server.Text(formatKBEntries(entries)), nil
	}
}

// formatKBResults renders scored results as compact answer cards.
func formatKBResults(rs []kb.Result) string {
	var b strings.Builder
	for i := range rs {
		writeKBCard(&b, &rs[i].Entry)
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatKBEntries renders entries as compact answer cards.
func formatKBEntries(es []kb.Entry) string {
	var b strings.Builder
	for i := range es {
		writeKBCard(&b, &es[i])
	}
	return strings.TrimRight(b.String(), "\n")
}

// writeKBCard renders one entry as an answer-shaped block: headline, optional
// signature, body, deprecation/use-instead, and provenance.
func writeKBCard(b *strings.Builder, e *kb.Entry) {
	head := e.Title
	if e.Package != "" {
		head = e.Package + ": " + head
	}
	fmt.Fprintf(b, "## [%s] %s\n", e.Kind, head)
	if e.Signature != "" {
		fmt.Fprintf(b, "```go\n%s\n```\n", e.Signature)
	}
	if e.Body != "" {
		fmt.Fprintf(b, "%s\n", strings.TrimSpace(e.Body))
	}
	if e.Deprecated {
		if e.UseInstead != "" {
			fmt.Fprintf(b, "DEPRECATED. Use instead: %s\n", e.UseInstead)
		} else {
			b.WriteString("DEPRECATED.\n")
		}
	} else if e.UseInstead != "" {
		fmt.Fprintf(b, "Use instead: %s\n", e.UseInstead)
	}
	if e.Ref != "" {
		fmt.Fprintf(b, "ref: %s", e.Ref)
		if e.Version != "" {
			fmt.Fprintf(b, " (%s)", e.Version)
		}
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-mcp/server"
)

// docsSiteBase prefixes entry refs (site paths like "/docs/core/async") so the
// caller gets a fetchable page URL.
const docsSiteBase = "https://vel.build"

// docsPerQueryLimit is how many pages each query pulls from the snapshot before
// merging; the token budget, not this, bounds what is actually returned.
const docsPerQueryLimit = 8

// docsDefaultTokenLimit caps the response when the caller sets no budget.
const docsDefaultTokenLimit = 3000

// NewSearchDocsHandler builds the velocity_search_docs handler: FTS5 retrieval
// over the doc pages baked into the knowledge-base snapshot (the published
// vel.build corpus). Page bodies are included best-first until the token budget
// runs out; pages past the budget are listed as title + URL so the caller knows
// they exist and can fetch or re-query.
func NewSearchDocsHandler(s *store.Store) func(context.Context, *server.Request) (*server.Response, error) {
	return func(ctx context.Context, req *server.Request) (*server.Response, error) {
		queries := stringSliceArg(req, "queries")
		if len(queries) == 0 {
			return server.Error("queries parameter is required"), nil
		}

		tokenLimit := docsDefaultTokenLimit
		if v, ok := req.IntOK("token_limit"); ok && v > 0 {
			tokenLimit = int(v)
		}

		results, err := searchDocPages(ctx, s, queries)
		if err != nil {
			return server.Error(fmt.Sprintf("docs search failed: %v", err)), nil
		}
		if len(results) == 0 {
			return server.Text("No documentation pages matched. The corpus is the published " +
				"vel.build docs; a miss here likely means the topic is undocumented - " +
				"fall back to framework source, do not guess."), nil
		}
		return server.Text(formatDocResults(results, tokenLimit)), nil
	}
}

// searchDocPages runs every query against the doc-kind entries and merges the
// ranked lists, keeping each page's best score and preserving best-first order.
func searchDocPages(ctx context.Context, s *store.Store, queries []string) ([]kb.Result, error) {
	var merged []kb.Result
	seen := map[int64]bool{}
	for _, q := range queries {
		rs, err := s.Search(ctx, q, kb.KindDoc, docsPerQueryLimit)
		if err != nil {
			return nil, err
		}
		for _, r := range rs {
			if seen[r.ID] {
				continue
			}
			seen[r.ID] = true
			merged = append(merged, r)
		}
	}
	return merged, nil
}

// formatDocResults renders matched pages: a totals header, then full bodies
// best-first within the token budget, then a link list for the remainder.
func formatDocResults(results []kb.Result, tokenLimit int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Documentation Search Results\nPages matched: %d\n\n", len(results))

	budget := tokenLimit - estimateTokens(b.String())
	var overflow []kb.Result
	for i, r := range results {
		card := formatDocCard(&r.Entry)
		cost := estimateTokens(card)
		if i > 0 && cost > budget {
			overflow = append(overflow, results[i:]...)
			break
		}
		if cost > budget {
			// Even the best page exceeds the budget: clamp its body rather
			// than returning nothing useful.
			card = clampToTokens(card, budget)
			cost = estimateTokens(card)
		}
		b.WriteString(card)
		budget -= cost
	}

	if len(overflow) > 0 {
		b.WriteString("## More matches (over token budget - fetch or re-query)\n")
		for _, r := range overflow {
			fmt.Fprintf(&b, "- %s - %s%s\n", r.Title, docsSiteBase, r.Ref)
		}
	}
	return strings.TrimRight(b.String(), "\n")
}

// formatDocCard renders one page: title with its docs section, source URL, body.
func formatDocCard(e *kb.Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "## %s (%s)\n", e.Title, docSection(e))
	fmt.Fprintf(&b, "source: %s%s\n\n", docsSiteBase, e.Ref)
	b.WriteString(strings.TrimSpace(e.Body))
	b.WriteString("\n\n---\n\n")
	return b.String()
}

// docSection reports the docs section a page belongs to ("core", "advanced",
// ...), derived from its site path.
func docSection(e *kb.Entry) string {
	parts := strings.Split(strings.TrimPrefix(e.Ref, "/docs/"), "/")
	if len(parts) > 1 && parts[0] != "" {
		return parts[0]
	}
	return "docs"
}

// clampToTokens truncates text to approximately the given token budget,
// marking the cut.
func clampToTokens(text string, tokens int) string {
	limit := tokens * 4 // inverse of estimateTokens
	if limit >= len(text) {
		return text
	}
	if limit < 0 {
		limit = 0
	}
	return text[:limit] + "\n[truncated: token budget reached - raise token_limit or fetch the source URL]\n\n"
}

// stringSliceArg returns the named argument as a []string: a []string passes
// through, a []any keeps only its string elements, anything else yields nil.
func stringSliceArg(req *server.Request, key string) []string {
	switch v := req.Get(key).(type) {
	case []string:
		return v
	case []any:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return nil
}

// estimateTokens is the rough ~4-chars-per-token budget heuristic.
func estimateTokens(text string) int {
	return len(text) / 4
}

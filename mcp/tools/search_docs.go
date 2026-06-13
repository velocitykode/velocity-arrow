package tools

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"

	"github.com/velocitykode/velocity-arrow/docs"
	"github.com/velocitykode/velocity-mcp/server"
)

// HandleSearchDocs searches embedded Velocity documentation using TF-IDF.
func HandleSearchDocs(ctx context.Context, req *server.Request) (*server.Response, error) {
	queries := stringSliceArg(req, "queries")
	if len(queries) == 0 {
		return server.Error("queries parameter is required"), nil
	}

	packages := stringSliceArg(req, "packages")
	tokenLimit := 3000
	if v, ok := req.IntOK("token_limit"); ok {
		tokenLimit = int(v)
	}
	if tokenLimit <= 0 {
		tokenLimit = 3000
	}

	results := searchDocs(queries, packages, tokenLimit)

	if len(results) == 0 {
		return server.Text("No documentation found matching your queries."), nil
	}

	var b strings.Builder
	b.WriteString("# Documentation Search Results\n\n")

	totalTokens := 0
	for _, r := range results {
		entry := fmt.Sprintf("## %s\n\n%s\n\n---\n\n", r.title, r.content)
		entryTokens := estimateTokens(entry)
		if totalTokens+entryTokens > tokenLimit {
			break
		}
		b.WriteString(entry)
		totalTokens += entryTokens
	}

	return server.Text(b.String()), nil
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

type searchResult struct {
	title   string
	content string
	score   float64
}

func searchDocs(queries, packages []string, tokenLimit int) []searchResult {
	entries := docs.AllDocs()

	if len(entries) == 0 {
		return nil
	}

	// Filter by packages if specified
	if len(packages) > 0 {
		var filtered []docs.DocEntry
		for _, entry := range entries {
			for _, pkg := range packages {
				if strings.Contains(strings.ToLower(entry.Path), strings.ToLower(pkg)) {
					filtered = append(filtered, entry)
					break
				}
			}
		}
		entries = filtered
	}

	// Score each document against all queries
	var results []searchResult
	for _, entry := range entries {
		score := 0.0
		for _, query := range queries {
			score += tfidfScore(query, entry.Content, entries)
		}
		if score > 0 {
			results = append(results, searchResult{
				title:   entry.Title,
				content: entry.Content,
				score:   score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	return results
}

func tfidfScore(query, document string, corpus []docs.DocEntry) float64 {
	queryTerms := tokenize(query)
	docTerms := tokenize(document)

	if len(docTerms) == 0 {
		return 0
	}

	// Term frequency in document
	termCount := make(map[string]int)
	for _, t := range docTerms {
		termCount[t]++
	}

	score := 0.0
	for _, term := range queryTerms {
		tf := float64(termCount[term]) / float64(len(docTerms))

		// Document frequency across corpus
		df := 0
		for _, entry := range corpus {
			if strings.Contains(strings.ToLower(entry.Content), term) {
				df++
			}
		}

		idf := math.Log(float64(len(corpus)+1) / float64(df+1))
		score += tf * idf
	}

	return score
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	for _, word := range strings.Fields(text) {
		word = strings.Trim(word, ".,;:!?()[]{}\"'`*#")
		if len(word) > 1 {
			tokens = append(tokens, word)
		}
	}
	return tokens
}

func estimateTokens(text string) int {
	// Rough estimate: ~4 chars per token
	return len(text) / 4
}

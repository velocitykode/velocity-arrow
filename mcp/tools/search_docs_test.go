package tools

import (
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/docs"
)

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! This is a test.")
	expected := []string{"hello", "world", "this", "is", "test"}
	if len(tokens) != len(expected) {
		t.Fatalf("tokenize length = %d, want %d: %v", len(tokens), len(expected), tokens)
	}
	for i, tok := range tokens {
		if tok != expected[i] {
			t.Errorf("token[%d] = %q, want %q", i, tok, expected[i])
		}
	}
}

func TestTokenize_Empty(t *testing.T) {
	tokens := tokenize("")
	if len(tokens) != 0 {
		t.Errorf("expected empty, got %v", tokens)
	}
}

func TestTokenize_SingleChar(t *testing.T) {
	tokens := tokenize("a b c go")
	if len(tokens) != 1 || tokens[0] != "go" {
		t.Errorf("expected [go], got %v", tokens)
	}
}

func TestEstimateTokens(t *testing.T) {
	if estimateTokens("12345678901234567890") != 5 {
		t.Error("20 chars should estimate to 5 tokens")
	}
	if estimateTokens("") != 0 {
		t.Error("empty string should estimate to 0 tokens")
	}
}

func TestTfidfScore_HigherForRelevantDoc(t *testing.T) {
	corpus := docs.AllDocs()
	if len(corpus) == 0 {
		t.Skip("no embedded docs")
	}

	// Find the ORM doc and the getting-started doc
	var ormContent, gsContent string
	for _, entry := range corpus {
		if strings.Contains(entry.Path, "orm") {
			ormContent = entry.Content
		}
		if strings.Contains(entry.Path, "getting-started") {
			gsContent = entry.Content
		}
	}

	if ormContent == "" || gsContent == "" {
		t.Skip("need both orm and getting-started docs")
	}

	ormScore := tfidfScore("orm models queries", ormContent, corpus)
	gsScore := tfidfScore("orm models queries", gsContent, corpus)

	if ormScore <= gsScore {
		t.Errorf("ORM doc should score higher than getting-started for 'orm models queries': orm=%f, gs=%f", ormScore, gsScore)
	}
}

func TestTfidfScore_ZeroForNoMatch(t *testing.T) {
	corpus := docs.AllDocs()
	score := tfidfScore("xyzzy_nonexistent_12345", "some random document content", corpus)
	if score != 0 {
		t.Errorf("score should be 0 for non-matching query, got %f", score)
	}
}

func TestSearchDocs_ResultsSortedByRelevance(t *testing.T) {
	results := searchDocs([]string{"orm models database"}, nil, 3000)
	if len(results) < 2 {
		t.Skip("need at least 2 results to test sorting")
	}

	// Results should be in descending score order
	for i := 1; i < len(results); i++ {
		if results[i].score > results[i-1].score {
			t.Errorf("results not sorted: [%d].score=%f > [%d].score=%f", i, results[i].score, i-1, results[i-1].score)
		}
	}
}

func TestSearchDocs_PackageFilter_ExcludesOtherPackages(t *testing.T) {
	// Without filter: should return both getting-started and orm docs
	allResults := searchDocs([]string{"velocity"}, nil, 3000)
	if len(allResults) < 2 {
		t.Skip("need at least 2 docs to test filtering")
	}

	// With orm filter: should only return orm-path docs
	filtered := searchDocs([]string{"velocity"}, []string{"orm"}, 3000)

	// Filtered should have fewer results than unfiltered
	if len(filtered) >= len(allResults) {
		t.Errorf("filter should reduce results: all=%d, filtered=%d", len(allResults), len(filtered))
	}

	// No filtered result should be from the getting-started path
	for _, r := range filtered {
		if strings.Contains(strings.ToLower(r.title), "getting started") {
			t.Errorf("orm filter should exclude getting-started doc, got %q", r.title)
		}
	}
}

func TestSearchDocs_NoMatch(t *testing.T) {
	results := searchDocs([]string{"xyzzy_nonexistent_term_12345"}, nil, 3000)
	if len(results) != 0 {
		t.Errorf("expected no results, got %d", len(results))
	}
}

func TestSearchDocs_TokenLimit_Truncates(t *testing.T) {
	unlimited := searchDocs([]string{"velocity"}, nil, 1000000)
	limited := searchDocs([]string{"velocity"}, nil, 50)

	if len(unlimited) == 0 {
		t.Skip("no results for velocity query")
	}

	// With a 50-token limit (~200 chars), we should get fewer results than unlimited
	if len(limited) >= len(unlimited) && len(unlimited) > 1 {
		t.Errorf("token limit should reduce results: unlimited=%d, limited=%d", len(unlimited), len(limited))
	}
}

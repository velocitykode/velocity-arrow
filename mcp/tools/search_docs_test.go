package tools

import (
	"strings"
	"testing"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

func docResult(id int64, title, ref, body string) kb.Result {
	return kb.Result{Entry: kb.Entry{ID: id, Kind: kb.KindDoc, Title: title, Ref: ref, Body: body}}
}

func TestFormatDocResults_HeaderAndCard(t *testing.T) {
	out := formatDocResults([]kb.Result{
		docResult(1, "Async", "/docs/core/async", "Body about async."),
	}, 3000)

	for _, want := range []string{
		"Pages matched: 1",
		"## Async (core)",
		"source: https://vel.build/docs/core/async",
		"Body about async.",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestFormatDocResults_OverflowListsLinks(t *testing.T) {
	big := strings.Repeat("filler content for the async body ", 100) // ~3400 chars, ~850 tokens
	out := formatDocResults([]kb.Result{
		docResult(1, "Async", "/docs/core/async", big),
		docResult(2, "Queue", "/docs/advanced/queue", big),
	}, 900)

	if !strings.Contains(out, "filler content") {
		t.Errorf("best page body should be included:\n%s", out)
	}
	if !strings.Contains(out, "More matches (over token budget") {
		t.Errorf("overflow header missing:\n%s", out)
	}
	if !strings.Contains(out, "- Queue - https://vel.build/docs/advanced/queue") {
		t.Errorf("overflow link missing:\n%s", out)
	}
	if strings.Count(out, "filler content") > 100 {
		t.Error("second page body should not be inlined")
	}
}

func TestFormatDocResults_ClampsSingleOversizedPage(t *testing.T) {
	big := strings.Repeat("word ", 2000) // ~10k chars, ~2500 tokens
	out := formatDocResults([]kb.Result{
		docResult(1, "Async", "/docs/core/async", big),
	}, 500)

	if !strings.Contains(out, "[truncated: token budget reached") {
		t.Errorf("expected truncation marker:\n%s", out)
	}
	if len(out) > 500*4+400 {
		t.Errorf("output %d chars exceeds clamped budget", len(out))
	}
}

func TestDocSection(t *testing.T) {
	cases := []struct{ ref, want string }{
		{"/docs/core/async", "core"},
		{"/docs/advanced/queue", "advanced"},
		{"/docs/overview", "docs"},
		{"/docs", "docs"},
	}
	for _, c := range cases {
		if got := docSection(&kb.Entry{Ref: c.ref}); got != c.want {
			t.Errorf("docSection(%q) = %q, want %q", c.ref, got, c.want)
		}
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

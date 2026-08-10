package corpus

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

const ruleDoc = `---
title: Use the crypto helper, never raw stdlib hashing
kind: rule
package: crypto
tags: [security, hashing, velocity-max]
use_instead: crypto.Hash
deprecated: false
---
Reach for the crypto helper for password hashing.

It wraps the algorithm choice so callers never touch stdlib directly.
`

const recipeDocCRLF = "---\r\n" +
	"title: Wire a stdio MCP server\r\n" +
	"kind: recipe\r\n" +
	"package: server\r\n" +
	"tags: mcp, stdio, transport\r\n" +
	"deprecated: true\r\n" +
	"---\r\n" +
	"Construct the server, register tools, then serve over stdio.\r\n"

const defaultKindDoc = `---
title: A document with no explicit kind
---
Body text only.
`

const noFenceDoc = `title: I forgot the fences
This file has no front-matter fence at all.
`

func TestMarkdown(t *testing.T) {
	fsys := fstest.MapFS{
		"rules/crypto.md":   {Data: []byte(ruleDoc)},
		"recipes/stdio.md":  {Data: []byte(recipeDocCRLF)},
		"concepts/plain.md": {Data: []byte(defaultKindDoc)},
		"notes/ignore.txt":  {Data: []byte("not markdown")},
		"nested/deep/x.md":  {Data: []byte(ruleDoc)},
	}

	entries, err := Markdown(fsys, "v9")
	if err != nil {
		t.Fatalf("Markdown: %v", err)
	}

	// 4 .md files (the .txt is ignored).
	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}

	byRef := make(map[string]kb.Entry, len(entries))
	for _, e := range entries {
		byRef[e.Ref] = e
		if e.Version != "v9" {
			t.Errorf("%s: version = %q, want v9", e.Ref, e.Version)
		}
	}

	rule := byRef["rules/crypto.md"]
	if rule.Kind != kb.KindRule {
		t.Errorf("rule kind = %q, want rule", rule.Kind)
	}
	if rule.Title != "Use the crypto helper, never raw stdlib hashing" {
		t.Errorf("rule title = %q", rule.Title)
	}
	if rule.Package != "crypto" {
		t.Errorf("rule package = %q, want crypto", rule.Package)
	}
	if rule.UseInstead != "crypto.Hash" {
		t.Errorf("rule use_instead = %q", rule.UseInstead)
	}
	if rule.Deprecated {
		t.Error("rule should not be deprecated")
	}
	wantTags := []string{"security", "hashing", "velocity-max"}
	if !equalStrings(rule.Tags, wantTags) {
		t.Errorf("rule tags = %v, want %v", rule.Tags, wantTags)
	}
	if !strings.HasPrefix(rule.Body, "Reach for the crypto helper") {
		t.Errorf("rule body = %q", rule.Body)
	}
	if strings.Contains(rule.Body, "---") {
		t.Errorf("rule body should not contain the fence: %q", rule.Body)
	}

	recipe := byRef["recipes/stdio.md"]
	if recipe.Kind != kb.KindRecipe {
		t.Errorf("recipe kind = %q, want recipe", recipe.Kind)
	}
	if !recipe.Deprecated {
		t.Error("recipe should be deprecated")
	}
	wantRecipeTags := []string{"mcp", "stdio", "transport"}
	if !equalStrings(recipe.Tags, wantRecipeTags) {
		t.Errorf("recipe tags = %v, want %v (comma form)", recipe.Tags, wantRecipeTags)
	}
	if !strings.HasPrefix(recipe.Body, "Construct the server") {
		t.Errorf("recipe body = %q (CRLF not normalised?)", recipe.Body)
	}

	plain := byRef["concepts/plain.md"]
	if plain.Kind != kb.KindRule {
		t.Errorf("missing-kind default = %q, want rule", plain.Kind)
	}
}

func TestMarkdownUnknownKindDefaults(t *testing.T) {
	doc := "---\ntitle: Has a bogus kind\nkind: nonsense\n---\nbody\n"
	fsys := fstest.MapFS{"a.md": {Data: []byte(doc)}}
	entries, err := Markdown(fsys, "v0")
	if err != nil {
		t.Fatalf("Markdown: %v", err)
	}
	if entries[0].Kind != kb.KindRule {
		t.Errorf("unknown kind = %q, want default rule", entries[0].Kind)
	}
}

func TestMarkdownErrors(t *testing.T) {
	tests := []struct {
		name    string
		doc     string
		wantSub string
	}{
		{name: "no fence", doc: noFenceDoc, wantSub: "no front-matter fence"},
		{name: "missing title", doc: "---\nkind: rule\n---\nbody\n", wantSub: "missing a title"},
		{name: "unterminated", doc: "---\ntitle: x\nbody with no close\n", wantSub: "unterminated"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fsys := fstest.MapFS{"bad.md": {Data: []byte(tc.doc)}}
			_, err := Markdown(fsys, "v0")
			if err == nil {
				t.Fatalf("expected error for %s", tc.name)
			}
			if !strings.Contains(err.Error(), tc.wantSub) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSub)
			}
			if !strings.Contains(err.Error(), "bad.md") {
				t.Errorf("error %q should identify the file", err.Error())
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

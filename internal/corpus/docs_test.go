package corpus

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/velocitykode/velocity-arrow/internal/kb"
)

func docsFixture() fstest.MapFS {
	return fstest.MapFS{
		"core/async.md": &fstest.MapFile{Data: []byte(`---
title: "Async"
description: Run concurrent operations.
weight: 40
---

Body about async.

{{< tabs items="A,B" >}}
{{< tab >}}
` + "```go\nasync.Run(fn)\n```" + `
{{< /tab >}}
{{< /tabs >}}
`)},
		"advanced/_index.md": &fstest.MapFile{Data: []byte(`---
title: "Advanced"
---
Section overview.
`)},
		"core/draft.md": &fstest.MapFile{Data: []byte(`---
title: "WIP"
draft: true
---
Not published.
`)},
		"core/empty.md": &fstest.MapFile{Data: []byte(`---
title: "Empty"
---
{{< placeholder >}}
`)},
	}
}

func TestDocs(t *testing.T) {
	entries, err := Docs(docsFixture(), "v1.0.0")
	if err != nil {
		t.Fatalf("Docs: %v", err)
	}
	// draft.md and empty.md are skipped.
	if len(entries) != 2 {
		t.Fatalf("entries = %d, want 2: %+v", len(entries), entries)
	}

	byTitle := map[string]kb.Entry{}
	for _, e := range entries {
		byTitle[e.Title] = e
	}

	async, ok := byTitle["Async"]
	if !ok {
		t.Fatal("missing Async entry")
	}
	if async.Kind != kb.KindDoc {
		t.Errorf("kind = %q, want doc", async.Kind)
	}
	if async.Ref != "/docs/core/async" {
		t.Errorf("ref = %q, want /docs/core/async", async.Ref)
	}
	if async.Version != "v1.0.0" {
		t.Errorf("version = %q, want v1.0.0", async.Version)
	}
	// Description leads the body; shortcode lines are stripped; code survives.
	for _, want := range []string{"Run concurrent operations.", "Body about async.", "async.Run(fn)"} {
		if !strings.Contains(async.Body, want) {
			t.Errorf("body missing %q:\n%s", want, async.Body)
		}
	}
	if strings.Contains(async.Body, "{{<") {
		t.Errorf("body still has shortcodes:\n%s", async.Body)
	}
	if len(async.Tags) != 2 || async.Tags[1] != "core" {
		t.Errorf("tags = %v, want [docs core]", async.Tags)
	}

	index, ok := byTitle["Advanced"]
	if !ok {
		t.Fatal("missing Advanced entry")
	}
	if index.Ref != "/docs/advanced" {
		t.Errorf("_index ref = %q, want /docs/advanced", index.Ref)
	}
}

func TestDocSitePath(t *testing.T) {
	cases := []struct{ in, ref, section string }{
		{"core/async.md", "/docs/core/async", "core"},
		{"advanced/_index.md", "/docs/advanced", "advanced"},
		{"_index.md", "/docs", "docs"},
		{"overview.md", "/docs/overview", "overview"},
	}
	for _, c := range cases {
		ref, section := docSitePath(c.in)
		if ref != c.ref || section != c.section {
			t.Errorf("docSitePath(%q) = (%q, %q), want (%q, %q)", c.in, ref, section, c.ref, c.section)
		}
	}
}

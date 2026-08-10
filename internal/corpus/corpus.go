// Package corpus turns sources into knowledge entries for the ingester: exact
// exported symbols parsed from the velocity source tree, curated guard rules and
// recipes loaded from markdown, and the chunking that keeps each entry atomic.
// It produces []kb.Entry; it does not embed or persist (the ingester wires those
// stages together).
package corpus

// Symbols and Markdown are implemented in symbols.go and markdown.go
// respectively. This file documents the package surface and the front-matter
// format the markdown loader recognises:
//
//	title:       headline (required)
//	kind:        rule | recipe | concept (default rule)
//	package:     owning velocity package, or "(general)"
//	tags:        [comma, or, bracket, list]
//	use_instead: replacement pointer
//	deprecated:  true | false
//
// Each entry is stamped with version and a Ref of the source file path. An
// unrecognised or empty kind defaults to KindRule.

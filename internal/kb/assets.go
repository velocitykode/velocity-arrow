package kb

import "embed"

// RulesFS holds the curated guard rule markdown (one file per rule). The
// snapshot builder reads it to produce KindRule entries; embedding keeps the
// rules in the module source of truth.
//
// The knowledge base itself is not embedded. It is a build output per velocity
// version, cached on disk and served for the version the current app compiles
// against (see internal/kbsource).
//
//go:embed rules
var RulesFS embed.FS

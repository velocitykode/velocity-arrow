package kb

import "embed"

// SnapshotDB is the baked, read-only knowledge-base snapshot built by
// cmd/ingest. It is embedded so arrow stays a single self-contained binary with
// no external file or network dependency at startup.
//
//go:embed data/velocity-kb.db
var SnapshotDB []byte

// RulesFS holds the curated guard rule markdown (one file per rule). The
// ingester reads it to build KindRule entries; embedding keeps the rules in the
// module source of truth.
//
//go:embed rules
var RulesFS embed.FS

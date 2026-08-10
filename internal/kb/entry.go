// Package kb holds the shared domain types for the knowledge base: the atomic
// Entry card, scored search Result, and coverage Manifest. It is a leaf package
// (no velocity dependencies) so every other package can import it freely.
package kb

// Kind classifies a knowledge entry by what question it answers.
type Kind string

const (
	// KindSymbol is an exact exported API surface (signature + provenance).
	KindSymbol Kind = "symbol"
	// KindHelper points at a framework helper for an intent ("hash a password").
	KindHelper Kind = "helper"
	// KindRule is curated negative knowledge: a guard, gotcha, or use-X-not-Y.
	KindRule Kind = "rule"
	// KindRecipe is a canonical wiring snippet for a task.
	KindRecipe Kind = "recipe"
	// KindConcept is a framework concept the LLM cannot infer from source alone.
	KindConcept Kind = "concept"
)

// Kinds lists every valid Kind, in retrieval-priority order.
var Kinds = []Kind{KindRule, KindSymbol, KindHelper, KindRecipe, KindConcept}

// Entry is one atomic knowledge card: the smallest correct unit an LLM can act
// on. Bodies are answer-shaped, not document-shaped. Every Entry carries
// provenance (Ref, Version) so a consumer can trust and verify it.
type Entry struct {
	ID         int64     // stable row id within a built snapshot
	Kind       Kind      // classification
	Title      string    // short headline used in ranked result lists
	Body       string    // answer-shaped content
	Signature  string    // exact declaration, for KindSymbol
	Package    string    // owning velocity package, e.g. "crypto"
	Ref        string    // provenance: "file:line" or a docs URL
	Version    string    // velocity version this entry was built against
	Deprecated bool      // true when the surface is deprecated
	UseInstead string    // replacement pointer when Deprecated or a rule forbids it
	Tags       []string  // free-form retrieval tags
	Embedding  []float32 // semantic vector; nil when not embedded
}

// Result is an Entry scored by a search, higher Score is more relevant.
type Result struct {
	Entry
	Score float64
}

// Manifest describes what a built snapshot covers, so a consumer knows the
// boundary and can treat a miss as "not in KB" rather than "not in framework".
type Manifest struct {
	VelocityVersion string       // framework version the snapshot was built against
	BuiltAt         string       // RFC3339 build timestamp, stamped by the ingester
	Packages        []string     // velocity packages covered
	Counts          map[Kind]int // entry count per Kind
	Total           int          // total entries
}

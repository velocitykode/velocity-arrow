// Package embed turns text into semantic vectors for the knowledge base. It
// wraps the velocity-ai embeddings surface at ingest time (batch) and query
// time (single). When no embedding backend is configured the knowledge base
// degrades to keyword-only retrieval, so callers must tolerate an error from
// Embed rather than failing hard.
package embed

import (
	"context"
	"errors"
)

// ErrUnavailable is returned by Embed when no embedding backend is configured
// (for example, no API credentials). Callers fall back to keyword-only search.
var ErrUnavailable = errors.New("embed: no embedding backend available")

// Embedder turns batches of text into fixed-width vectors.
type Embedder interface {
	// Embed returns one vector per input text, in order. It returns
	// ErrUnavailable when no backend is configured.
	Embed(ctx context.Context, texts []string) ([][]float32, error)
	// Dimensions reports the vector width this Embedder produces.
	Dimensions() int
}

// Dimensions is the default embedding width used across the knowledge base.
// Kept small to keep the baked snapshot compact and linear cosine fast.
const Dimensions = 256

// New returns the configured Embedder. The v1 knowledge base ships keyword-only
// (FTS5) retrieval, so this returns a Noop: queries are not vectorised and the
// snapshot stores no embeddings. A real backend (an embeddings provider behind
// the Embedder interface) can be swapped in later without touching callers, at
// which point the dormant cosine path in the store activates.
func New() Embedder {
	return Noop()
}

// Noop is an Embedder that always reports ErrUnavailable. It lets the server and
// ingester run keyword-only with no backend.
func Noop() Embedder { return noop{} }

type noop struct{}

func (noop) Embed(context.Context, []string) ([][]float32, error) {
	return nil, ErrUnavailable
}

func (noop) Dimensions() int { return Dimensions }

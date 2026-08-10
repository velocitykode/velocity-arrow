package tools

import (
	"context"

	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-mcp/server"
)

// KBManifestURI is the URI of the knowledge-base coverage manifest resource.
const KBManifestURI = "kb://manifest"

// NewKBManifestResource returns the kb://manifest resource: a JSON description
// of what the snapshot covers (velocity version, packages, per-kind counts,
// build time) so a consumer knows the boundary and can treat a miss as
// "not in KB" rather than "not in the framework".
func NewKBManifestResource(s *store.Store) server.Resource {
	return &kbManifestResource{store: s}
}

type kbManifestResource struct {
	store *store.Store
}

func (r *kbManifestResource) Name() string        { return "kb-manifest" }
func (r *kbManifestResource) Description() string { return "Knowledge-base coverage manifest." }
func (r *kbManifestResource) URI() string         { return KBManifestURI }
func (r *kbManifestResource) MimeType() string    { return "application/json" }

func (r *kbManifestResource) Read(ctx context.Context, _ *server.Request) (*server.Response, error) {
	m, err := r.store.Manifest(ctx)
	if err != nil {
		return server.Error(err.Error()), nil
	}
	return server.JSON(m)
}

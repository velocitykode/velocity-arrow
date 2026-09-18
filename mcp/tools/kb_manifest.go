package tools

import (
	"context"

	"github.com/velocitykode/velocity-arrow/internal/kb"
	"github.com/velocitykode/velocity-arrow/internal/kbsource"
	"github.com/velocitykode/velocity-arrow/internal/store"
	"github.com/velocitykode/velocity-mcp/server"
)

// KBManifestURI is the URI of the knowledge-base coverage manifest resource.
const KBManifestURI = "kb://manifest"

// NewKBManifestResource returns the kb://manifest resource: a JSON description
// of what the snapshot covers (velocity version, packages, per-kind counts,
// build time) so a consumer knows the boundary and can treat a miss as
// "not in KB" rather than "not in the framework".
func NewKBManifestResource(s *store.Store, status kbsource.Status) server.Resource {
	return &kbManifestResource{store: s, status: status}
}

type kbManifestResource struct {
	store  *store.Store
	status kbsource.Status
}

// manifestView is the kb://manifest payload: the snapshot manifest plus where
// it came from and how the framework has moved since the app's pin.
type manifestView struct {
	kb.Manifest
	Pin     string       `json:"Pin"`
	PinFrom string       `json:"PinFrom"`
	Source  string       `json:"Source"`
	Path    string       `json:"Path"`
	Gap     kbsource.Gap `json:"Gap"`
	Nudge   string       `json:"Nudge,omitempty"`
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
	return server.JSON(manifestView{Manifest: m, Pin: r.status.Pin.Version, PinFrom: r.status.Pin.Origin, Source: r.status.Snapshot.Source, Path: r.status.Snapshot.Path, Gap: r.status.Gap, Nudge: r.status.Nudge()})
}

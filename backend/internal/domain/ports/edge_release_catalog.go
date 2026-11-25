package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// EdgeReleaseCatalog reads the published edge releases. Manifests are read one at a time so a
// caller can stop at the first release it accepts.
type EdgeReleaseCatalog interface {
	// ListEdgeReleases returns the edge/vX.Y.Z releases, in any order; other channels are left out.
	ListEdgeReleases(ctx context.Context) ([]dto.EdgeRelease, error)
	// GetEdgeManifest reads one release's manifest. An installer address outside the project's
	// release downloads is dropped, leaving InstallerURL empty.
	GetEdgeManifest(ctx context.Context, version string) (*dto.EdgeManifest, error)
}

package ports

import "context"

// BoxOrganizationRepository keeps the box's own organization row, which every tenant row on
// the box references. The cloud never sends it: the box adds it from its configuration.
type BoxOrganizationRepository interface {
	// Ensure inserts the organization when the box has no row for it, and leaves an existing row alone.
	Ensure(ctx context.Context, organizationID string) error
}

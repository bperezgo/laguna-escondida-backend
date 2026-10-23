package repository

import (
	"context"

	"laguna-escondida/backend/internal/domain/tenant"
)

// lagunaCtx is the context a request of Laguna's runs with: every repository reads and
// writes only that organization's rows.
func lagunaCtx() context.Context {
	return tenant.WithOrg(context.Background(), lagunaOrgID)
}

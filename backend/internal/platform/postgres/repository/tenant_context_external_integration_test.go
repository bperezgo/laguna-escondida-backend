package repository_test

import (
	"context"

	"laguna-escondida/backend/internal/domain/tenant"
)

const lagunaOrgID = "bb0cad3e-be13-4bad-9a37-bba9525bfece"

// lagunaCtx is the context a request of Laguna's runs with.
func lagunaCtx() context.Context {
	return tenant.WithOrg(context.Background(), lagunaOrgID)
}

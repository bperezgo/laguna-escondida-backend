// Package tenant carries the organization a request or job acts for through its context.
package tenant

import (
	"context"

	domainError "laguna-escondida/backend/internal/domain/error"
)

type orgKey struct{}

func WithOrg(ctx context.Context, orgID string) context.Context {
	return context.WithValue(ctx, orgKey{}, orgID)
}

func OrgFrom(ctx context.Context) (string, bool) {
	orgID, ok := ctx.Value(orgKey{}).(string)
	return orgID, ok && orgID != ""
}

// RequireOrg is OrgFrom for callers that must not proceed without an organization.
func RequireOrg(ctx context.Context) (string, error) {
	orgID, ok := OrgFrom(ctx)
	if !ok {
		return "", domainError.ErrNoOrganization
	}
	return orgID, nil
}

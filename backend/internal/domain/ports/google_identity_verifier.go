package ports

import (
	"context"

	"laguna-escondida/backend/internal/domain/dto"
)

// GoogleIdentityVerifier checks a Google ID token's signature, issuer, audience, expiry and verified email.
type GoogleIdentityVerifier interface {
	Verify(ctx context.Context, idToken string) (*dto.GoogleIdentity, error)
}

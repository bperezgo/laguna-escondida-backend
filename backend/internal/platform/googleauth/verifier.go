// Package googleauth verifies Google ID tokens from Google Identity Services.
package googleauth

import (
	"context"
	"errors"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/coreos/go-oidc/v3/oidc"
)

const (
	GoogleIssuer  = "https://accounts.google.com"
	GoogleJWKSURL = "https://www.googleapis.com/oauth2/v3/certs"
)

type Verifier struct {
	verifier *oidc.IDTokenVerifier
}

// New fetches and rotates Google's keys lazily, on the first token and whenever a token names an unknown key.
func New(clientID string) ports.GoogleIdentityVerifier {
	return NewWithKeys(clientID, GoogleIssuer, GoogleJWKSURL)
}

// NewWithKeys lets tests point at a local issuer and key set.
func NewWithKeys(clientID, issuer, jwksURL string) ports.GoogleIdentityVerifier {
	keys := oidc.NewRemoteKeySet(context.Background(), jwksURL)
	return &Verifier{verifier: oidc.NewVerifier(issuer, keys, &oidc.Config{ClientID: clientID})}
}

type claims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
}

func (v *Verifier) Verify(ctx context.Context, idToken string) (*dto.GoogleIdentity, error) {
	if idToken == "" {
		return nil, fmt.Errorf("%w: empty token", domainError.ErrGoogleTokenInvalid)
	}
	token, err := v.verifier.Verify(ctx, idToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrGoogleTokenInvalid, err)
	}
	var c claims
	if err := token.Claims(&c); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrGoogleTokenInvalid, err)
	}
	if c.Email == "" || !c.EmailVerified {
		return nil, fmt.Errorf("%w: %w", domainError.ErrGoogleTokenInvalid, errors.New("email missing or not verified by Google"))
	}
	return &dto.GoogleIdentity{Email: c.Email, Name: c.Name, Subject: token.Subject}, nil
}

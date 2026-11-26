package googleauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testClientID = "123-test.apps.googleusercontent.com"
	testKeyID    = "test-key"
)

type issuer struct {
	key    *rsa.PrivateKey
	server *httptest.Server
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := map[string][]map[string]string{"keys": {{
		"kty": "RSA", "alg": "RS256", "use": "sig", "kid": testKeyID,
		"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
		"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
	}}}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(server.Close)
	return &issuer{key: key, server: server}
}

func (i *issuer) sign(t *testing.T, mutate func(jwt.MapClaims)) string {
	t.Helper()
	c := jwt.MapClaims{
		"iss": GoogleIssuer, "aud": testClientID, "sub": "1084",
		"email": "rosa.perez@example.com", "email_verified": true, "name": "Rosa Pérez",
		"iat": time.Now().Add(-time.Minute).Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	if mutate != nil {
		mutate(c)
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, c)
	token.Header["kid"] = testKeyID
	signed, err := token.SignedString(i.key)
	require.NoError(t, err)
	return signed
}

func (i *issuer) verifier() ports.GoogleIdentityVerifier {
	return NewWithKeys(testClientID, GoogleIssuer, i.server.URL)
}

func TestVerifier_ValidToken(t *testing.T) {
	iss := newIssuer(t)

	identity, err := iss.verifier().Verify(context.Background(), iss.sign(t, nil))

	require.NoError(t, err)
	assert.Equal(t, "rosa.perez@example.com", identity.Email)
	assert.Equal(t, "Rosa Pérez", identity.Name)
	assert.Equal(t, "1084", identity.Subject)
}

func TestVerifier_Refusals(t *testing.T) {
	iss := newIssuer(t)
	other := newIssuer(t)
	cases := map[string]string{
		"wrong audience":        iss.sign(t, func(c jwt.MapClaims) { c["aud"] = "someone-else.apps.googleusercontent.com" }),
		"expired":               iss.sign(t, func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Minute).Unix() }),
		"unverified email":      iss.sign(t, func(c jwt.MapClaims) { c["email_verified"] = false }),
		"no email":              iss.sign(t, func(c jwt.MapClaims) { delete(c, "email") }),
		"wrong issuer":          iss.sign(t, func(c jwt.MapClaims) { c["iss"] = "https://evil.example.com" }),
		"signed by another key": other.sign(t, nil),
		"not a token":           "garbage",
		"empty":                 "",
	}
	for name, token := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := iss.verifier().Verify(context.Background(), token)
			assert.ErrorIs(t, err, domainError.ErrGoogleTokenInvalid)
		})
	}
}

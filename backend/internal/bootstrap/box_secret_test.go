package bootstrap

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestResolveJWTSecret_EnvironmentValueWins(t *testing.T) {
	store := mocks.NewMockBoxSecretRepository(t)

	secret, err := resolveJWTSecret(context.Background(), "from-env", store, slog.New(slog.DiscardHandler))

	require.NoError(t, err)
	assert.Equal(t, "from-env", secret)
}

func TestResolveJWTSecret_CreatesAtLeast256BitsAndNeverLogsIt(t *testing.T) {
	store := mocks.NewMockBoxSecretRepository(t)
	var offered string
	store.EXPECT().GetOrCreate(mock.Anything, jwtSigningSecretName, mock.Anything).
		RunAndReturn(func(_ context.Context, _, candidate string) (string, error) {
			offered = candidate
			return candidate, nil
		})
	logs := &bytes.Buffer{}

	secret, err := resolveJWTSecret(context.Background(), "", store, slog.New(slog.NewTextHandler(logs, nil)))

	require.NoError(t, err)
	raw, err := base64.RawURLEncoding.DecodeString(offered)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(raw), 32)
	assert.Equal(t, offered, secret)
	assert.NotContains(t, logs.String(), secret)
}

func TestResolveJWTSecret_UsesTheStoredValueOverTheCandidate(t *testing.T) {
	store := mocks.NewMockBoxSecretRepository(t)
	store.EXPECT().GetOrCreate(mock.Anything, jwtSigningSecretName, mock.Anything).Return("stored", nil)

	secret, err := resolveJWTSecret(context.Background(), "", store, slog.New(slog.DiscardHandler))

	require.NoError(t, err)
	assert.Equal(t, "stored", secret)
}

func TestResolveJWTSecret_StoreFailureStopsTheBoot(t *testing.T) {
	store := mocks.NewMockBoxSecretRepository(t)
	store.EXPECT().GetOrCreate(mock.Anything, jwtSigningSecretName, mock.Anything).Return("", errors.New("permission denied"))

	_, err := resolveJWTSecret(context.Background(), "", store, slog.New(slog.DiscardHandler))

	require.ErrorContains(t, err, "permission denied")
}

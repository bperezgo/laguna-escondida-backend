package bootstrap

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"

	"laguna-escondida/backend/internal/domain/ports"
)

const jwtSigningSecretName = "jwt_signing"

// resolveJWTSecret prefers the environment's value; otherwise the box's own, created on the
// first boot and kept in its database so sessions outlive restarts and appliance updates.
func resolveJWTSecret(ctx context.Context, configured string, store ports.BoxSecretRepository, logger *slog.Logger) (string, error) {
	if configured != "" {
		return configured, nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate the box signing secret: %w", err)
	}
	secret, err := store.GetOrCreate(ctx, jwtSigningSecretName, base64.RawURLEncoding.EncodeToString(raw))
	if err != nil {
		return "", fmt.Errorf("resolve the box signing secret: %w", err)
	}
	logger.Info("Signing tokens with the box's own secret; set JWT_SECRET to override it")
	return secret, nil
}

package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const secretsKeySize = 32

// Integrations holds the key that encrypts organizations' provider credentials; without it
// integrations can be neither saved nor read.
type Integrations struct {
	SecretsKey []byte
}

func (i Integrations) Enabled() bool {
	return len(i.SecretsKey) > 0
}

func loadIntegrations() (Integrations, error) {
	key, err := loadSecretsKey("INTEGRATION_SECRETS_KEY")
	if err != nil {
		return Integrations{}, err
	}
	return Integrations{SecretsKey: key}, nil
}

// loadSecretsKey reads a base64 AES-256 key: unset is nil, anything malformed is an error naming the variable.
func loadSecretsKey(name string) ([]byte, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64: %w", name, err)
	}
	if len(key) != secretsKeySize {
		return nil, fmt.Errorf("%s must decode to %d bytes, got %d", name, secretsKeySize, len(key))
	}
	return key, nil
}

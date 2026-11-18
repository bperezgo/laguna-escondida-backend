package config

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

const notificationSecretsKeySize = 32

// StaffNotifications holds the key that encrypts channel secrets; without it notifications are off.
type StaffNotifications struct {
	SecretsKey []byte
}

func (s StaffNotifications) Enabled() bool {
	return len(s.SecretsKey) > 0
}

func loadStaffNotifications() (StaffNotifications, error) {
	raw := strings.TrimSpace(os.Getenv("NOTIFICATION_SECRETS_KEY"))
	if raw == "" {
		return StaffNotifications{}, nil
	}
	key, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return StaffNotifications{}, fmt.Errorf("NOTIFICATION_SECRETS_KEY must be base64: %w", err)
	}
	if len(key) != notificationSecretsKeySize {
		return StaffNotifications{}, fmt.Errorf("NOTIFICATION_SECRETS_KEY must decode to %d bytes, got %d", notificationSecretsKeySize, len(key))
	}
	return StaffNotifications{SecretsKey: key}, nil
}

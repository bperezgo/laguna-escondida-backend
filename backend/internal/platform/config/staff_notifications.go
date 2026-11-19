package config

// StaffNotifications holds the key that encrypts channel secrets; without it notifications are off.
type StaffNotifications struct {
	SecretsKey []byte
}

func (s StaffNotifications) Enabled() bool {
	return len(s.SecretsKey) > 0
}

func loadStaffNotifications() (StaffNotifications, error) {
	key, err := loadSecretsKey("NOTIFICATION_SECRETS_KEY")
	if err != nil {
		return StaffNotifications{}, err
	}
	return StaffNotifications{SecretsKey: key}, nil
}

package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCloud_SignupDefaults(t *testing.T) {
	setCloudEnv(t)

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.False(t, cfg.Signup.Enabled, "signup is off until switched on")
	assert.False(t, cfg.Signup.GoogleEnabled())
	assert.Equal(t, "https://app.example.com", cfg.Signup.PublicAppURL)
}

func TestLoadCloud_SignupFromEnv(t *testing.T) {
	setCloudEnv(t)
	t.Setenv("SIGNUP_ENABLED", "true")
	t.Setenv("GOOGLE_CLIENT_ID", "123.apps.googleusercontent.com")
	t.Setenv("PUBLIC_APP_URL", "https://app.kuenta.co/")

	cfg, err := LoadCloud()

	require.NoError(t, err)
	assert.True(t, cfg.Signup.Enabled)
	assert.Equal(t, "123.apps.googleusercontent.com", cfg.Signup.GoogleClientID)
	assert.Equal(t, "https://app.kuenta.co", cfg.Signup.PublicAppURL)
}

func TestLoadCloud_SignupInvalid(t *testing.T) {
	cases := map[string][2]string{
		"bad switch":      {"SIGNUP_ENABLED", "yes please"},
		"no app url":      {"PUBLIC_APP_URL", ""},
		"app url w/ path": {"PUBLIC_APP_URL", "https://app.example.com/x"},
		"app url scheme":  {"PUBLIC_APP_URL", "app.example.com"},
	}
	for name, env := range cases {
		t.Run(name, func(t *testing.T) {
			setCloudEnv(t)
			t.Setenv(env[0], env[1])
			_, err := LoadCloud()
			require.Error(t, err)
			assert.Contains(t, err.Error(), env[0])
		})
	}
}

func TestLoadEdge_DoesNotReadSignup(t *testing.T) {
	setEdgeEnv(t)
	t.Setenv("PUBLIC_APP_URL", "")
	t.Setenv("SIGNUP_ENABLED", "nonsense")

	_, err := LoadEdge()

	require.NoError(t, err)
}

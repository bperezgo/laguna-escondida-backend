package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// Signup is self-serve signup and Google sign-in. Without a client id no Google route is registered.
type Signup struct {
	// Enabled registers POST /api/signup/google; off by default so the cloud ships before signup opens.
	Enabled        bool
	GoogleClientID string
	// PublicAppURL is the operations app's origin; invitation links point at it.
	PublicAppURL string
}

func (s Signup) GoogleEnabled() bool { return s.GoogleClientID != "" }

func loadSignup() (Signup, error) {
	enabled := false
	if raw := os.Getenv("SIGNUP_ENABLED"); raw != "" {
		parsed, err := strconv.ParseBool(raw)
		if err != nil {
			return Signup{}, fmt.Errorf("invalid SIGNUP_ENABLED %q: want true or false", raw)
		}
		enabled = parsed
	}
	appURL, err := loadPublicAppURL()
	if err != nil {
		return Signup{}, err
	}
	return Signup{Enabled: enabled, GoogleClientID: strings.TrimSpace(os.Getenv("GOOGLE_CLIENT_ID")), PublicAppURL: appURL}, nil
}

func loadPublicAppURL() (string, error) {
	raw := os.Getenv("PUBLIC_APP_URL")
	if raw == "" {
		return "", errors.New("PUBLIC_APP_URL is not set")
	}
	parsed, err := url.Parse(raw)
	invalid := err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
		strings.Trim(parsed.Path, "/") != "" || parsed.RawQuery != "" || parsed.Fragment != "" || parsed.User != nil
	if invalid {
		return "", fmt.Errorf("invalid PUBLIC_APP_URL %q: want an absolute http(s) URL with a host and no path, query or fragment", raw)
	}
	return strings.TrimSuffix(raw, "/"), nil
}

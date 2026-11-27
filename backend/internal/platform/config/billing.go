package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Billing is the platform's own Wompi account, the one restaurants pay their plan to. It is not
// an organization's Wompi integration. Without keys, signup ends in a status page and no card is asked.
//
// The events and integrity secrets are what charging needs on top: with the keys alone cards are
// saved and nothing is ever charged.
type Billing struct {
	WompiPublicKey       string
	WompiPrivateKey      string
	WompiEventsSecret    string
	WompiIntegritySecret string
}

func (b Billing) CardEntry() bool { return b.WompiPublicKey != "" && b.WompiPrivateKey != "" }

// Charging is on only with all four values.
func (b Billing) Charging() bool {
	return b.CardEntry() && b.WompiEventsSecret != "" && b.WompiIntegritySecret != ""
}

func loadBilling() (Billing, error) {
	billing, err := NewBilling(strings.TrimSpace(os.Getenv("BILLING_WOMPI_PUBLIC_KEY")), strings.TrimSpace(os.Getenv("BILLING_WOMPI_PRIVATE_KEY")))
	if err != nil {
		return Billing{}, err
	}
	return billing.WithChargeSecrets(strings.TrimSpace(os.Getenv("BILLING_WOMPI_EVENTS_SECRET")), strings.TrimSpace(os.Getenv("BILLING_WOMPI_INTEGRITY_SECRET")))
}

// WithChargeSecrets refuses one secret without the other, secrets without the keys, and secrets of
// another environment than the keys: each would start charges that can never be made or settled.
func (b Billing) WithChargeSecrets(eventsSecret, integritySecret string) (Billing, error) {
	if eventsSecret == "" && integritySecret == "" {
		return b, nil
	}
	if eventsSecret == "" || integritySecret == "" {
		return Billing{}, errors.New("BILLING_WOMPI_EVENTS_SECRET and BILLING_WOMPI_INTEGRITY_SECRET must be set together")
	}
	if !b.CardEntry() {
		return Billing{}, errors.New("BILLING_WOMPI_EVENTS_SECRET and BILLING_WOMPI_INTEGRITY_SECRET need BILLING_WOMPI_PUBLIC_KEY and BILLING_WOMPI_PRIVATE_KEY")
	}
	keysEnv, _ := wompiKeyEnvironment(b.WompiPublicKey, "pub_")
	for _, secret := range []struct{ name, value, kind string }{
		{"BILLING_WOMPI_EVENTS_SECRET", eventsSecret, "events"},
		{"BILLING_WOMPI_INTEGRITY_SECRET", integritySecret, "integrity"},
	} {
		env, ok := wompiSecretEnvironment(secret.value, secret.kind)
		if !ok {
			return Billing{}, fmt.Errorf("%s must start with test_%s_ or prod_%s_", secret.name, secret.kind, secret.kind)
		}
		if env != keysEnv {
			return Billing{}, fmt.Errorf("%s is of another environment than the billing Wompi keys: it is %s, the keys are %s", secret.name, env, keysEnv)
		}
	}
	b.WompiEventsSecret, b.WompiIntegritySecret = eventsSecret, integritySecret
	return b, nil
}

func wompiSecretEnvironment(secret, kind string) (string, bool) {
	for _, env := range []string{"test", "prod"} {
		prefix := env + "_" + kind + "_"
		if strings.HasPrefix(secret, prefix) && len(secret) > len(prefix) {
			return env, true
		}
	}
	return "", false
}

// NewBilling refuses half a key pair and a pair from two environments: either would save cards
// that can never be charged.
func NewBilling(publicKey, privateKey string) (Billing, error) {
	if publicKey == "" && privateKey == "" {
		return Billing{}, nil
	}
	if publicKey == "" || privateKey == "" {
		return Billing{}, errors.New("BILLING_WOMPI_PUBLIC_KEY and BILLING_WOMPI_PRIVATE_KEY must be set together")
	}
	publicEnv, ok := wompiKeyEnvironment(publicKey, "pub_")
	if !ok {
		return Billing{}, errors.New("BILLING_WOMPI_PUBLIC_KEY must start with pub_test_ or pub_prod_")
	}
	privateEnv, ok := wompiKeyEnvironment(privateKey, "prv_")
	if !ok {
		return Billing{}, errors.New("BILLING_WOMPI_PRIVATE_KEY must start with prv_test_ or prv_prod_")
	}
	if publicEnv != privateEnv {
		return Billing{}, fmt.Errorf("the billing Wompi keys are of different environments: public is %s, private is %s", publicEnv, privateEnv)
	}
	return Billing{WompiPublicKey: publicKey, WompiPrivateKey: privateKey}, nil
}

func wompiKeyEnvironment(key, prefix string) (string, bool) {
	rest, ok := strings.CutPrefix(key, prefix)
	if !ok {
		return "", false
	}
	for _, env := range []string{"test", "prod"} {
		if strings.HasPrefix(rest, env+"_") && len(rest) > len(env)+1 {
			return env, true
		}
	}
	return "", false
}

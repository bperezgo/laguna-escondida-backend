package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
)

// Billing is the platform's own Wompi account, the one restaurants pay their plan to. It is not
// an organization's Wompi integration. Without keys, signup ends in a status page and no card is asked.
type Billing struct {
	WompiPublicKey  string
	WompiPrivateKey string
}

func (b Billing) CardEntry() bool { return b.WompiPublicKey != "" && b.WompiPrivateKey != "" }

func loadBilling() (Billing, error) {
	return NewBilling(strings.TrimSpace(os.Getenv("BILLING_WOMPI_PUBLIC_KEY")), strings.TrimSpace(os.Getenv("BILLING_WOMPI_PRIVATE_KEY")))
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

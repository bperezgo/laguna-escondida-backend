package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBilling(t *testing.T) {
	cases := []struct {
		name, public, private string
		cardEntry, wantErr    bool
	}{
		{"no keys", "", "", false, false},
		{"both test keys", "pub_test_abc", "prv_test_def", true, false},
		{"both prod keys", "pub_prod_abc", "prv_prod_def", true, false},
		{"public alone", "pub_test_abc", "", false, true},
		{"private alone", "", "prv_test_def", false, true},
		{"test public with prod private", "pub_test_abc", "prv_prod_def", false, true},
		{"prod public with test private", "pub_prod_abc", "prv_test_def", false, true},
		{"keys swapped", "prv_test_def", "pub_test_abc", false, true},
		{"unknown environment", "pub_stage_abc", "prv_stage_def", false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			billing, err := NewBilling(tc.public, tc.private)
			if tc.wantErr {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "def", "an error never repeats a key")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.cardEntry, billing.CardEntry())
		})
	}
}

func TestLoadBilling_ReadsTheEnvironment(t *testing.T) {
	t.Setenv("BILLING_WOMPI_PUBLIC_KEY", " pub_test_abc ")
	t.Setenv("BILLING_WOMPI_PRIVATE_KEY", "prv_test_def")

	billing, err := loadBilling()

	require.NoError(t, err)
	assert.Equal(t, Billing{WompiPublicKey: "pub_test_abc", WompiPrivateKey: "prv_test_def"}, billing)
}

func TestBilling_WithChargeSecrets(t *testing.T) {
	const pub, prv = "pub_test_abc", "prv_test_def"
	cases := []struct {
		name                         string
		public, private              string
		events, integrity            string
		cardEntry, charging, wantErr bool
	}{
		{"all four values", pub, prv, "test_events_ghi", "test_integrity_jkl", true, true, false},
		{"all four production values", "pub_prod_abc", "prv_prod_def", "prod_events_ghi", "prod_integrity_jkl", true, true, false},
		{"public and private only", pub, prv, "", "", true, false, false},
		{"nothing set", "", "", "", "", false, false, false},
		{"events secret alone with the keys", pub, prv, "test_events_ghi", "", false, false, true},
		{"integrity secret alone with the keys", pub, prv, "", "test_integrity_jkl", false, false, true},
		{"both secrets without the keys", "", "", "test_events_ghi", "test_integrity_jkl", false, false, true},
		{"events secret without the keys", "", "", "test_events_ghi", "", false, false, true},
		{"production secrets with test keys", pub, prv, "prod_events_ghi", "prod_integrity_jkl", false, false, true},
		{"secrets swapped", pub, prv, "test_integrity_jkl", "test_events_ghi", false, false, true},
		{"a secret that is not Wompi's", pub, prv, "ghi", "test_integrity_jkl", false, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			keys, err := NewBilling(tc.public, tc.private)
			require.NoError(t, err)

			billing, err := keys.WithChargeSecrets(tc.events, tc.integrity)

			if tc.wantErr {
				require.Error(t, err)
				assert.NotContains(t, err.Error(), "ghi", "an error never repeats a secret")
				assert.NotContains(t, err.Error(), "jkl", "an error never repeats a secret")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.cardEntry, billing.CardEntry())
			assert.Equal(t, tc.charging, billing.Charging())
		})
	}
}

func TestLoadBilling_ReadsTheChargeSecrets(t *testing.T) {
	t.Setenv("BILLING_WOMPI_PUBLIC_KEY", "pub_test_abc")
	t.Setenv("BILLING_WOMPI_PRIVATE_KEY", "prv_test_def")
	t.Setenv("BILLING_WOMPI_EVENTS_SECRET", " test_events_ghi ")
	t.Setenv("BILLING_WOMPI_INTEGRITY_SECRET", "test_integrity_jkl")

	billing, err := loadBilling()

	require.NoError(t, err)
	assert.True(t, billing.Charging())
	assert.Equal(t, "test_events_ghi", billing.WompiEventsSecret)
}

func TestLoadBilling_HalfTheChargeSecretsFailsStartup(t *testing.T) {
	t.Setenv("BILLING_WOMPI_PUBLIC_KEY", "pub_test_abc")
	t.Setenv("BILLING_WOMPI_PRIVATE_KEY", "prv_test_def")
	t.Setenv("BILLING_WOMPI_EVENTS_SECRET", "test_events_ghi")
	t.Setenv("BILLING_WOMPI_INTEGRITY_SECRET", "")

	_, err := loadBilling()

	require.Error(t, err)
}

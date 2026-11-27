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

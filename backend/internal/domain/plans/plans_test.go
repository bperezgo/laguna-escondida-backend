package plans

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFeatureForRoute_SpecTable(t *testing.T) {
	cases := []struct {
		path string
		want Feature
	}{
		{"/api/invoices", FeatureEInvoicing},
		{"/api/invoices/:id/pdf", FeatureEInvoicing},
		{"/api/support-documents", FeatureEInvoicing},
		{"/api/support-documents/:id", FeatureEInvoicing},
		{"/api/reservations", FeatureCustomerChannels},
		{"/api/reservations/:id/confirm", FeatureCustomerChannels},
		{"/api/sse/reservations", FeatureCustomerChannels},
		{"/api/contacts/:id", FeatureCustomerChannels},
		{"/api/feedback", FeatureCustomerChannels},
		{"/api/delivery-zones/:id", FeatureCustomerChannels},
		{"/api/deliveries/:id/claim", FeatureCustomerChannels},
		{"/api/notification-channels", FeatureCustomerChannels},
		{"/api/notification-rules/:id", FeatureCustomerChannels},
		{"/api/organization/integrations/wompi", FeatureIntegrations},
		{"/api/organization/payment-settings", FeatureIntegrations},
		{"/api/organization/api-keys/:id", FeatureAgents},
		{"/api/products", FeatureCore},
		{"/api/orders/:id/pay", FeatureCore},
		{"/api/organization/settings", FeatureCore},
		{"/api/sse/commands", FeatureCore},
		{"/api/edge/enrollment-codes", FeatureCore},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			assert.Equal(t, tc.want, FeatureForRoute(tc.path))
		})
	}
}

func TestIncludesAndRequiredPlan(t *testing.T) {
	cases := []struct {
		feature  Feature
		starter  bool
		einv     bool
		full     bool
		required Plan
	}{
		{FeatureCore, true, true, true, Starter},
		{FeatureEInvoicing, false, true, true, EInvoicing},
		{FeatureCustomerChannels, false, false, true, Full},
		{FeatureIntegrations, false, false, true, Full},
		{FeatureAgents, false, false, true, Full},
	}
	for _, tc := range cases {
		t.Run(string(tc.feature), func(t *testing.T) {
			assert.Equal(t, tc.starter, Includes(Starter, tc.feature))
			assert.Equal(t, tc.einv, Includes(EInvoicing, tc.feature))
			assert.Equal(t, tc.full, Includes(Full, tc.feature))
			assert.Equal(t, tc.required, RequiredPlan(tc.feature))
		})
	}
}

func TestValid(t *testing.T) {
	assert.True(t, Valid("starter"))
	assert.True(t, Valid("einvoicing"))
	assert.True(t, Valid("full"))
	assert.False(t, Valid("premium"))
	assert.False(t, Valid(""))
}

func TestExpired(t *testing.T) {
	now := time.Date(2026, 11, 25, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	cases := []struct {
		name   string
		status string
		ends   *time.Time
		want   bool
	}{
		{"trial before its end", StatusTrial, at(time.Second), false},
		{"trial exactly at its end", StatusTrial, at(0), true},
		{"trial after its end", StatusTrial, at(-time.Hour), true},
		{"active with a past end never expires", StatusActive, at(-time.Hour), false},
		{"active without an end", StatusActive, nil, false},
		{"suspended is not expired", StatusSuspended, at(-time.Hour), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, Expired(tc.status, tc.ends, now))
		})
	}
}

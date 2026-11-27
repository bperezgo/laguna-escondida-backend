package plans

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrice_SpecTable(t *testing.T) {
	cases := []struct {
		plan    Plan
		monthly int
		annual  int
	}{
		{Starter, 25000, 250000},
		{EInvoicing, 40000, 400000},
		{Full, 100000, 1000000},
	}
	for _, tc := range cases {
		t.Run(string(tc.plan), func(t *testing.T) {
			assert.Equal(t, tc.monthly, Price(tc.plan, Monthly))
			assert.Equal(t, tc.annual, Price(tc.plan, Annual))
			assert.Equal(t, Price(tc.plan, Monthly)*10, Price(tc.plan, Annual))
		})
	}
}

func TestPrice_UnknownPlanOrPeriodIsZero(t *testing.T) {
	assert.Zero(t, Price("gold", Monthly))
	assert.Zero(t, Price(Full, "weekly"))
}

func TestValidPeriod(t *testing.T) {
	assert.True(t, ValidPeriod("monthly"))
	assert.True(t, ValidPeriod("annual"))
	assert.False(t, ValidPeriod("weekly"))
	assert.False(t, ValidPeriod(""))
}

func TestTrialDays(t *testing.T) {
	assert.Equal(t, 14, TrialDays)
}

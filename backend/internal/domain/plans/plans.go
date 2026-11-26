// Package plans decides which cloud features an organization's plan includes and when its trial ends.
package plans

import (
	"strings"
	"time"
)

type Plan string

const (
	Starter    Plan = "starter"
	EInvoicing Plan = "einvoicing"
	Full       Plan = "full"
)

// Ordered cheapest first: RequiredPlan returns the first that includes a feature.
var All = []Plan{Starter, EInvoicing, Full}

type Feature string

const (
	FeatureCore             Feature = "core"
	FeatureEInvoicing       Feature = "einvoicing"
	FeatureCustomerChannels Feature = "customer_channels"
	FeatureIntegrations     Feature = "integrations"
	FeatureAgents           Feature = "agents"
)

var features = map[Plan][]Feature{
	Starter:    {FeatureCore},
	EInvoicing: {FeatureCore, FeatureEInvoicing},
	Full:       {FeatureCore, FeatureEInvoicing, FeatureCustomerChannels, FeatureIntegrations, FeatureAgents},
}

type RouteRule struct {
	Prefix  string
	Feature Feature
	// Exact matches only the path itself; otherwise every path starting with Prefix matches.
	Exact bool
}

// Routes is the organization-plans spec table; a route not listed is core.
var Routes = []RouteRule{
	{Prefix: "/api/invoices", Feature: FeatureEInvoicing},
	{Prefix: "/api/support-documents", Feature: FeatureEInvoicing},
	{Prefix: "/api/reservations", Feature: FeatureCustomerChannels},
	{Prefix: "/api/sse/reservations", Feature: FeatureCustomerChannels, Exact: true},
	{Prefix: "/api/contacts", Feature: FeatureCustomerChannels},
	{Prefix: "/api/feedback", Feature: FeatureCustomerChannels},
	{Prefix: "/api/delivery-zones", Feature: FeatureCustomerChannels},
	{Prefix: "/api/deliveries", Feature: FeatureCustomerChannels},
	{Prefix: "/api/notification-channels", Feature: FeatureCustomerChannels},
	{Prefix: "/api/notification-rules", Feature: FeatureCustomerChannels},
	{Prefix: "/api/organization/integrations", Feature: FeatureIntegrations},
	{Prefix: "/api/organization/payment-settings", Feature: FeatureIntegrations},
	{Prefix: "/api/organization/api-keys", Feature: FeatureAgents},
}

func Valid(plan string) bool {
	_, ok := features[Plan(plan)]
	return ok
}

func Includes(plan Plan, feature Feature) bool {
	for _, f := range features[plan] {
		if f == feature {
			return true
		}
	}
	return false
}

func RequiredPlan(feature Feature) Plan {
	for _, p := range All {
		if Includes(p, feature) {
			return p
		}
	}
	return Full
}

func FeatureForRoute(path string) Feature {
	for _, r := range Routes {
		if r.Exact {
			if path == r.Prefix {
				return r.Feature
			}
			continue
		}
		if strings.HasPrefix(path, r.Prefix) {
			return r.Feature
		}
	}
	return FeatureCore
}

// Expired is computed at every check, so extending trial_ends_at takes effect with no job.
func Expired(status string, trialEndsAt *time.Time, now time.Time) bool {
	if status != StatusTrial || trialEndsAt == nil {
		return false
	}
	return !now.Before(*trialEndsAt)
}

const (
	StatusPending   = "pending"
	StatusTrial     = "trial"
	StatusActive    = "active"
	StatusSuspended = "suspended"
)

const TrialLength = 14 * 24 * time.Hour

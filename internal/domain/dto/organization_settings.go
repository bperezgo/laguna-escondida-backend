package dto

import "time"

// OrganizationSettings is the organization's settings bag, stored as one JSON document.
type OrganizationSettings struct {
	Ordering OrderingSettings `json:"ordering"`
}

type OrderingSettings struct {
	Delivery OrderTypeSettings `json:"delivery"`
	Takeaway OrderTypeSettings `json:"takeaway"`
}

// OrderTypeSettings: an empty schedule means always open.
type OrderTypeSettings struct {
	Paused       bool            `json:"paused"`
	PausedReason *string         `json:"paused_reason"`
	Schedule     []OpeningWindow `json:"schedule"`
}

// OpeningWindow is read in the organization's timezone; Closes before Opens runs past midnight.
type OpeningWindow struct {
	Day    string `json:"day"`
	Opens  string `json:"opens"`
	Closes string `json:"closes"`
}

type AvailabilityReason string

const (
	AvailabilityReasonPaused       AvailabilityReason = "paused"
	AvailabilityReasonOutsideHours AvailabilityReason = "outside_hours"
)

type Availability struct {
	Available    bool                `json:"available"`
	Reason       *AvailabilityReason `json:"reason"`
	PausedReason *string             `json:"paused_reason"`
	ClosesAt     *time.Time          `json:"closes_at"`
	NextOpensAt  *time.Time          `json:"next_opens_at"`
}

type OrderingAvailability struct {
	Delivery Availability `json:"delivery"`
	Takeaway Availability `json:"takeaway"`
}

// OrganizationSettingsView is one read of what availability needs: the ordering settings and the timezone.
type OrganizationSettingsView struct {
	Ordering OrderingSettings `json:"ordering"`
	Timezone string           `json:"timezone"`
}

type OrganizationSettingsResponse struct {
	Ordering     OrderingSettings     `json:"ordering"`
	Availability OrderingAvailability `json:"availability"`
	Timezone     string               `json:"timezone"`
}

// UpdateOrganizationSettingsRequest: a nil field is left unchanged.
type UpdateOrganizationSettingsRequest struct {
	Ordering *OrderingSettingsPatch `json:"ordering"`
}

type OrderingSettingsPatch struct {
	Delivery *OrderTypeSettingsPatch `json:"delivery"`
	Takeaway *OrderTypeSettingsPatch `json:"takeaway"`
}

// OrderTypeSettingsPatch: Schedule replaces the whole schedule; an empty list clears it.
type OrderTypeSettingsPatch struct {
	Paused       *bool            `json:"paused"`
	PausedReason *string          `json:"paused_reason"`
	Schedule     *[]OpeningWindow `json:"schedule"`
}

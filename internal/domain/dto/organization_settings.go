package dto

import "time"

// OrganizationSettings is the organization's settings bag, stored as one JSON document.
type OrganizationSettings struct {
	Ordering OrderingSettings `json:"ordering"`
	Feedback FeedbackSettings `json:"feedback"`
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

// OrganizationSettingsView is one read of the settings and the timezone they are read in.
type OrganizationSettingsView struct {
	Ordering OrderingSettings `json:"ordering"`
	Feedback FeedbackSettings `json:"feedback"`
	Timezone string           `json:"timezone"`
}

type OrganizationSettingsResponse struct {
	Ordering     OrderingSettings     `json:"ordering"`
	Feedback     FeedbackSettings     `json:"feedback"`
	Availability OrderingAvailability `json:"availability"`
	Timezone     string               `json:"timezone"`
}

// UpdateOrganizationSettingsRequest: a nil field is left unchanged.
type UpdateOrganizationSettingsRequest struct {
	Ordering *OrderingSettingsPatch `json:"ordering"`
	Feedback *FeedbackSettingsPatch `json:"feedback"`
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

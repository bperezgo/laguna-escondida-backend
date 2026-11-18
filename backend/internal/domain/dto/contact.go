package dto

import "time"

// Attribution is the ad that brought a guest, as the agent runtime reported it.
type Attribution struct {
	Source       *string    `json:"source,omitempty"`
	AdID         *string    `json:"ad_id,omitempty"`
	ClickedAt    *time.Time `json:"clicked_at,omitempty"`
	CampaignID   *string    `json:"campaign_id,omitempty"`
	CampaignName *string    `json:"campaign_name,omitempty"`
}

// Origin is where an agent reservation or order came from; every field is optional.
type Origin struct {
	ChannelID      *string      `json:"channel_id,omitempty"`
	ConversationID *string      `json:"conversation_id,omitempty"`
	Attribution    *Attribution `json:"attribution,omitempty"`
}

// OriginRequest is what create endpoints accept. ClickedAt stays a string so a garbled value
// can be dropped instead of failing the whole body; unknown keys such as ctwa_clid are ignored.
type OriginRequest struct {
	ChannelID      *string             `json:"channel_id,omitempty"`
	ConversationID *string             `json:"conversation_id,omitempty"`
	Attribution    *AttributionRequest `json:"attribution,omitempty"`
}

type AttributionRequest struct {
	Source       *string `json:"source,omitempty"`
	AdID         *string `json:"ad_id,omitempty"`
	ClickedAt    *string `json:"clicked_at,omitempty"`
	CampaignID   *string `json:"campaign_id,omitempty"`
	CampaignName *string `json:"campaign_name,omitempty"`
}

// Contact is one guest phone within an organization.
type Contact struct {
	ID          string       `json:"id"`
	Phone       string       `json:"phone"`
	Name        *string      `json:"name,omitempty"`
	FirstTouch  *Attribution `json:"first_touch,omitempty"`
	FirstSeenAt time.Time    `json:"first_seen_at"`
	LastSeenAt  time.Time    `json:"last_seen_at"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ContactTouch is one reservation or agent order seen for a phone.
type ContactTouch struct {
	Phone       string
	Name        *string
	Attribution *Attribution
	At          time.Time
}

type ContactDetail struct {
	Contact
	Reservations []*Reservation `json:"reservations"`
	Feedback     []*Feedback    `json:"feedback"`
}

// ContactListFilter: Search matches the phone or the name; zero Limit means the default page.
type ContactListFilter struct {
	Search string
	Limit  int
	Offset int
}

type ContactListResponse struct {
	Contacts []*Contact `json:"contacts"`
	Total    int        `json:"total"`
	Limit    int        `json:"limit"`
	Offset   int        `json:"offset"`
}

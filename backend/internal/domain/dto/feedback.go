package dto

import "time"

type FeedbackRequest struct {
	ID            string    `json:"id"`
	ReservationID string    `json:"reservation_id"`
	ContactID     string    `json:"contact_id"`
	RequestedAt   time.Time `json:"requested_at"`
	ExpiresAt     time.Time `json:"expires_at"`
}

type FeedbackReservation struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	PartySize int       `json:"party_size"`
	StartsAt  time.Time `json:"starts_at"`
}

type Feedback struct {
	ID                string              `json:"id"`
	FeedbackRequestID string              `json:"feedback_request_id"`
	ContactID         string              `json:"contact_id"`
	ContactPhone      string              `json:"contact_phone"`
	Rating            int                 `json:"rating"`
	Comment           *string             `json:"comment,omitempty"`
	NeedsAttention    bool                `json:"needs_attention"`
	Reservation       FeedbackReservation `json:"reservation"`
	CreatedAt         time.Time           `json:"created_at"`
	UpdatedAt         time.Time           `json:"updated_at"`
}

type RecordFeedbackRequest struct {
	CustomerPhone string  `json:"customer_phone"`
	Rating        int     `json:"rating"`
	Comment       *string `json:"comment,omitempty"`
}

// RecordFeedbackResponse: ReviewURL comes only from the organization's settings, never from the rating.
type RecordFeedbackResponse struct {
	Feedback  *Feedback `json:"feedback"`
	Escalate  bool      `json:"escalate"`
	ReviewURL *string   `json:"review_url,omitempty"`
}

// FeedbackListFilter: From inclusive, To exclusive, on when the feedback was last recorded.
type FeedbackListFilter struct {
	From           *time.Time
	To             *time.Time
	NeedsAttention bool
	ContactID      string
}

// FeedbackListQuery is the list endpoint's query: from/to as YYYY-MM-DD, organization-local, to inclusive.
type FeedbackListQuery struct {
	From           string
	To             string
	NeedsAttention bool
}

type FeedbackListResponse struct {
	Feedback []*Feedback `json:"feedback"`
	Total    int         `json:"total"`
}

// FeedbackSettings: the review link is shared with every guest or with none.
type FeedbackSettings struct {
	ReviewLinkEnabled bool   `json:"review_link_enabled"`
	ReviewURL         string `json:"review_url"`
}

type FeedbackSettingsPatch struct {
	ReviewLinkEnabled *bool   `json:"review_link_enabled"`
	ReviewURL         *string `json:"review_url"`
}

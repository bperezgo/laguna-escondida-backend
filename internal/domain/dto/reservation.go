package dto

import "time"

type ReservationStatus string

const (
	ReservationStatusRequested ReservationStatus = "requested"
	ReservationStatusConfirmed ReservationStatus = "confirmed"
	ReservationStatusVisited   ReservationStatus = "visited"
	ReservationStatusDeclined  ReservationStatus = "declined"
	ReservationStatusCancelled ReservationStatus = "cancelled"
	ReservationStatusNoShow    ReservationStatus = "no_show"
)

type ReservationSource string

const (
	ReservationSourceWhatsApp ReservationSource = "whatsapp"
	ReservationSourcePhone    ReservationSource = "phone"
	ReservationSourceWalkIn   ReservationSource = "walk_in"
	ReservationSourceStaff    ReservationSource = "staff"
)

type ReservationCancelledBy string

const (
	CancelledByCustomer ReservationCancelledBy = "customer"
	CancelledByStaff    ReservationCancelledBy = "staff"
)

type Reservation struct {
	ID            string                  `json:"id"`
	ContactID     string                  `json:"contact_id"`
	ContactPhone  string                  `json:"contact_phone"`
	Name          string                  `json:"name"`
	PartySize     int                     `json:"party_size"`
	StartsAt      time.Time               `json:"starts_at"`
	Notes         *string                 `json:"notes,omitempty"`
	Source        ReservationSource       `json:"source"`
	Status        ReservationStatus       `json:"status"`
	DeclineReason *string                 `json:"decline_reason,omitempty"`
	CancelledBy   *ReservationCancelledBy `json:"cancelled_by,omitempty"`
	VisitedAt     *time.Time              `json:"visited_at,omitempty"`
	// FeedbackRequested is read-only: set by the repository from feedback_requests.
	FeedbackRequested bool      `json:"feedback_requested"`
	Origin            Origin    `json:"origin"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateReservationRequest: StartsAt is RFC 3339 with an offset, kept a string so a bad value
// is a validation error naming starts_at rather than an unreadable body.
type CreateReservationRequest struct {
	CustomerPhone string            `json:"customer_phone"`
	Name          string            `json:"name"`
	PartySize     int               `json:"party_size"`
	StartsAt      string            `json:"starts_at"`
	Notes         *string           `json:"notes,omitempty"`
	Source        ReservationSource `json:"source"`
	Origin        *OriginRequest    `json:"origin,omitempty"`
}

type DeclineReservationRequest struct {
	Reason *string `json:"reason,omitempty"`
}

// CancelReservationRequest: CustomerPhone is required when the caller cancels for a guest.
type CancelReservationRequest struct {
	CustomerPhone *string `json:"customer_phone,omitempty"`
}

// ReservationListFilter: zero values mean no filter; From is inclusive and To exclusive on starts_at.
type ReservationListFilter struct {
	From      *time.Time
	To        *time.Time
	Statuses  []ReservationStatus
	Phone     string
	ContactID string
}

// ReservationListQuery is the list endpoint's query: Date (YYYY-MM-DD, organization-local),
// or Phone with an optional From (RFC 3339), or From alone for every day from then on. None means today.
type ReservationListQuery struct {
	Date   string
	Status string
	Phone  string
	From   string
}

type ReservationListResponse struct {
	Reservations []*Reservation `json:"reservations"`
	Total        int            `json:"total"`
}

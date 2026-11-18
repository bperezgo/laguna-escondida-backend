package dto

import "time"

type LeadStatus string

const (
	LeadStatusNew       LeadStatus = "new"
	LeadStatusContacted LeadStatus = "contacted"
	LeadStatusWon       LeadStatus = "won"
	LeadStatusLost      LeadStatus = "lost"
)

func (s LeadStatus) Valid() bool {
	switch s {
	case LeadStatusNew, LeadStatusContacted, LeadStatusWon, LeadStatusLost:
		return true
	}
	return false
}

type LeadAlertStatus string

const (
	LeadAlertPending LeadAlertStatus = "pending"
	LeadAlertSent    LeadAlertStatus = "sent"
	LeadAlertFailed  LeadAlertStatus = "failed"
	LeadAlertSkipped LeadAlertStatus = "skipped"
)

// LeadAttribution is where a prospect came from; an empty field was absent or invalid.
type LeadAttribution struct {
	UTMSource   string `json:"utm_source"`
	UTMMedium   string `json:"utm_medium"`
	UTMCampaign string `json:"utm_campaign"`
	UTMTerm     string `json:"utm_term"`
	UTMContent  string `json:"utm_content"`
	FBCLID      string `json:"fbclid"`
	GCLID       string `json:"gclid"`
	LandingURL  string `json:"landing_url"`
	Referrer    string `json:"referrer"`
	EventID     string `json:"event_id"`
}

// Lead is a prospect's demo request. It belongs to the platform, not to any organization.
type Lead struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	RestaurantName string     `json:"restaurant_name"`
	Phone          string     `json:"phone"`
	Status         LeadStatus `json:"status"`
	Notes          string     `json:"notes"`
	ConsentAt      time.Time  `json:"consent_at"`
	LeadAttribution
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	AlertStatus        LeadAlertStatus `json:"-"`
	AlertAttempts      int             `json:"-"`
	AlertNextAttemptAt *time.Time      `json:"-"`
	AlertLastError     string          `json:"-"`
}

// SubmitLeadRequest is the public form's body. Website is the honeypot: a person never fills it.
type SubmitLeadRequest struct {
	Name           string `json:"name"`
	RestaurantName string `json:"restaurant_name"`
	Phone          string `json:"phone"`
	Consent        bool   `json:"consent"`
	Website        string `json:"website"`
	LeadAttribution
}

// UpdateLeadRequest: nil leaves a field unchanged.
type UpdateLeadRequest struct {
	Status *string `json:"status"`
	Notes  *string `json:"notes"`
}

// ListLeadsFilter: an empty Status lists every status; zero Page and PageSize take the defaults.
type ListLeadsFilter struct {
	Status   string
	Page     int
	PageSize int
}

type LeadPage struct {
	Leads    []*Lead `json:"leads"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

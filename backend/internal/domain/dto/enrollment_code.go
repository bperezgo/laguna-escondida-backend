package dto

import "time"

type EnrollmentCode struct {
	ID             string
	OrganizationID string
	CreatedBy      string
	ExpiresAt      time.Time
	UsedAt         *time.Time
	CreatedAt      time.Time
}

// IssuedEnrollmentCode is the only place a code's plaintext appears.
type IssuedEnrollmentCode struct {
	Code      string    `json:"code"`
	ExpiresAt time.Time `json:"expires_at"`
}

type EnrollWithCodeRequest struct {
	Code    string `json:"code"`
	Name    string `json:"name"`
	Replace bool   `json:"replace"`
}

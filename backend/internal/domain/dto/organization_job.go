package dto

// OrganizationJobFailure is one organization's failed run of a job that runs for each of them.
type OrganizationJobFailure struct {
	OrganizationID string
	Err            error
}

// OrganizationTargetRequest names the organization an admin-key operation acts for, by slug.
type OrganizationTargetRequest struct {
	Organization string `json:"organization"`
}

// EventOrganization is embedded in every event: handlers run on another goroutine with no
// request behind them, so the event itself says which organization it was raised in.
type EventOrganization struct {
	OrganizationID string `json:"organization_id"`
}

func (e EventOrganization) Organization() string {
	return e.OrganizationID
}

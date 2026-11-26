package dto

import "time"

// Invitation is a pending invitation to the cloud app; its token is never stored, only its hash.
type Invitation struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"-"`
	Email          string     `json:"email"`
	Name           string     `json:"name"`
	RoleIDs        []int      `json:"role_ids"`
	Roles          []*Role    `json:"roles"`
	InvitedBy      string     `json:"invited_by"`
	InvitedByName  string     `json:"invited_by_name"`
	ExpiresAt      time.Time  `json:"expires_at"`
	CreatedAt      time.Time  `json:"created_at"`
	AcceptedAt     *time.Time `json:"-"`
	RevokedAt      *time.Time `json:"-"`
}

type CreateInvitationRequest struct {
	Email   string `json:"email"`
	Name    string `json:"name"`
	RoleIDs []int  `json:"role_ids"`
}

// InvitationWithLink is the only place an invitation's link ever appears.
type InvitationWithLink struct {
	Invitation *Invitation `json:"invitation"`
	Link       string      `json:"link"`
}

type InvitationsListResponse struct {
	Invitations []*Invitation `json:"invitations"`
}

// PublicInvitation is what the acceptance page shows to whoever holds the link.
type PublicInvitation struct {
	OrganizationName string    `json:"organization_name"`
	Email            string    `json:"email"`
	Name             string    `json:"name"`
	Roles            []*Role   `json:"roles"`
	ExpiresAt        time.Time `json:"expires_at"`
}

// AcceptInvitationRequest carries a password or a Google ID token, not both.
type AcceptInvitationRequest struct {
	Password string `json:"password"`
	IDToken  string `json:"id_token"`
}

// InviterPrincipal is who creates an invitation, as far as role rules care.
type InviterPrincipal struct {
	UserID  string
	RoleIDs []int
}

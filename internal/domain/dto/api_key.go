package dto

import "time"

// APIKeyPrefix starts every organization API key's plaintext, so a bearer can be told from a JWT.
const APIKeyPrefix = "lgk_"

type IssueAPIKeyRequest struct {
	UserID string `json:"user_id"`
	Name   string `json:"name"`
}

// APIKey is a key as listed: never its plaintext or hash.
type APIKey struct {
	ID             string     `json:"id"`
	OrganizationID string     `json:"organization_id"`
	UserID         string     `json:"user_id"`
	Name           string     `json:"name"`
	Prefix         string     `json:"prefix"`
	CreatedAt      time.Time  `json:"created_at"`
	LastUsedAt     *time.Time `json:"last_used_at"`
	RevokedAt      *time.Time `json:"revoked_at"`
}

// IssuedAPIKey is the only response that carries the plaintext.
type IssuedAPIKey struct {
	APIKey
	Key string `json:"key"`
}

type APIKeyListResponse struct {
	APIKeys []*APIKey `json:"api_keys"`
}

// APIKeyRecord is what a key lookup finds: the key, its user and its organization as they are now.
type APIKeyRecord struct {
	KeyID              string
	KeyOrganizationID  string
	RevokedAt          *time.Time
	UserID             string
	Username           string
	UserOrganizationID string
	UserActive         bool
	UserDeleted        bool
	OrganizationStatus string
	RoleIDs            []int
}

// APIKeyPrincipal is who a valid key authenticates as.
type APIKeyPrincipal struct {
	KeyID          string
	UserID         string
	Username       string
	OrganizationID string
	RoleIDs        []int
}

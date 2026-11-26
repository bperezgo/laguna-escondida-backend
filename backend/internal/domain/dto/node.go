package dto

import "time"

// NodeKeyPrefix starts every edge box key's plaintext, so it can be told from an API key (lgk_).
const NodeKeyPrefix = "lgn_"

// Node is an install registered in the cloud's sync topology, and the organization it syncs for.
type Node struct {
	ID             string
	Name           string
	Kind           string
	OrganizationID string
	HasKey         bool
	EnrolledAt     *time.Time
	RevokedAt      *time.Time
	LastSeenAt     *time.Time
}

// NodeAuthRecord is what authenticating a node's request needs in one read.
type NodeAuthRecord struct {
	Node
	KeyHash            string
	OrganizationStatus string
	TrialEndsAt        *time.Time
}

type EnrollEdgeBoxRequest struct {
	Name    string `json:"name"`
	Replace bool   `json:"replace"`
}

// EnrolledEdgeBox is the only response that carries the plaintext node key.
type EnrolledEdgeBox struct {
	NodeID           string `json:"node_id"`
	OrganizationID   string `json:"organization_id"`
	OrganizationName string `json:"organization_name"`
	NodeKey          string `json:"node_key"`
	CloudSyncURL     string `json:"cloud_sync_url"`
}

type ActiveBox struct {
	Name       string     `json:"name"`
	EnrolledAt *time.Time `json:"enrolled_at"`
	LastSeenAt *time.Time `json:"last_seen_at"`
}

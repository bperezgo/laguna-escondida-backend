package dto

// Node is an install registered in the cloud's sync topology, and the organization it syncs for.
type Node struct {
	ID             string
	Name           string
	Kind           string
	OrganizationID string
}

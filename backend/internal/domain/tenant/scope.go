package tenant

// Scope is which organizations an install serves: any of them (the cloud) or one (an edge box).
type Scope interface {
	Admits(orgID string) bool
	Pinned() (orgID string, ok bool)
}

type anyOrganization struct{}

func (anyOrganization) Admits(string) bool     { return true }
func (anyOrganization) Pinned() (string, bool) { return "", false }

type pinnedOrganization struct{ id string }

func (p pinnedOrganization) Admits(orgID string) bool { return orgID == p.id }
func (p pinnedOrganization) Pinned() (string, bool)   { return p.id, true }

// AnyOrganization is the cloud's scope: each caller brings its own organization.
func AnyOrganization() Scope { return anyOrganization{} }

// PinnedTo is an edge box's scope: it serves orgID and refuses every other.
func PinnedTo(orgID string) Scope { return pinnedOrganization{id: orgID} }

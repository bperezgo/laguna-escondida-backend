package dto

// GoogleIdentity is a verified Google account; Email is as Google sent it, not yet normalized.
type GoogleIdentity struct {
	Email   string
	Name    string
	Subject string
}

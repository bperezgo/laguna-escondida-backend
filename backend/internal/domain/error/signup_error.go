package error

import "errors"

var (
	// ErrGoogleTokenInvalid covers a bad signature, audience, issuer, expiry or an unverified email alike.
	ErrGoogleTokenInvalid = errors.New("invalid Google ID token")
	// ErrGoogleSignInFailed is the opaque answer for every Google sign-in failure but an expired trial.
	ErrGoogleSignInFailed = errors.New("google sign-in failed")
	ErrSignupDisabled     = errors.New("signup is disabled")
	ErrSlugUnavailable    = errors.New("no free slug for this restaurant name")
	ErrUserHasEmail       = errors.New("user has an email; only users without one get a generated password")
)

var (
	ErrInvitationNotFound    = errors.New("invitation not found")
	ErrInvitationNameInvalid = errors.New("name is required and must be at most 255 characters")
	// ErrInvitationInvalidOrExpired covers an unknown, expired, revoked or accepted link alike.
	ErrInvitationInvalidOrExpired = errors.New("invalid or expired link")
	ErrInvitationPending          = errors.New("email already has a pending invitation")
	ErrInvitationRoleNotAllowed   = errors.New("invitations are for admin, manager, accountant or courier roles only")
	ErrInvitationAdminOnly        = errors.New("only an admin can invite an admin")
	ErrInvitationEmailMismatch    = errors.New("google account email does not match the invitation")
	ErrInvitationCredentials      = errors.New("either a password of at least 8 characters or a Google ID token is required")
	ErrGoogleNotConfigured        = errors.New("google sign-in is not configured")
)

// ErrEnrollmentCodeInvalid covers an unknown, expired or used code alike.
var ErrEnrollmentCodeInvalid = errors.New("invalid or expired code")

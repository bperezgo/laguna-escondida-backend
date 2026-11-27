package dto

type GoogleSignInRequest struct {
	IDToken string `json:"id_token"`
}

type GoogleSignupRequest struct {
	IDToken        string `json:"id_token"`
	RestaurantName string `json:"restaurant_name"`
	Plan           string `json:"plan"`
}

// GoogleSignupResult tells a new account from a Google account that was already registered.
// A new account carries Created and no SignIn: it waits as pending until its trial is started.
type GoogleSignupResult struct {
	SignIn  *SignInResponse
	Created *GoogleSignupCreated
}

type GoogleSignupCreated struct {
	Status           string `json:"status"`
	OrganizationName string `json:"organization_name"`
	Plan             string `json:"plan"`
	// SetupToken is accepted only by the billing setup routes; it is not a sign-in.
	SetupToken string `json:"setup_token"`
}

type GeneratedPasswordResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

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
type GoogleSignupResult struct {
	SignIn  *SignInResponse
	Created bool
}

type GeneratedPasswordResponse struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

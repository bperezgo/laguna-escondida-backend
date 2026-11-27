package dto

import "time"

type PlanPrice struct {
	ID      string `json:"id"`
	Monthly int    `json:"monthly"`
	Annual  int    `json:"annual"`
}

type PlanPriceList struct {
	Currency  string      `json:"currency"`
	TrialDays int         `json:"trial_days"`
	Plans     []PlanPrice `json:"plans"`
}

const BillingProviderWompi = "wompi"

// OrganizationBilling is cloud-only and never synced to a box.
type OrganizationBilling struct {
	OrganizationID  string
	Period          string
	Provider        *string
	PaymentSourceID *string
	CardBrand       *string
	CardLastFour    *string
	TermsAcceptedAt *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (b *OrganizationBilling) HasCard() bool {
	return b != nil && b.PaymentSourceID != nil && *b.PaymentSourceID != ""
}

// BillingCard is what a saved card leaves behind: the provider's reference and what to show of it.
type BillingCard struct {
	Provider        string
	PaymentSourceID string
	Brand           string
	LastFour        string
	TermsAcceptedAt time.Time
}

// BillingSetupState never carries a private key.
type BillingSetupState struct {
	OrganizationName string             `json:"organization_name"`
	Plan             string             `json:"plan"`
	Period           string             `json:"period"`
	CardEntry        bool               `json:"card_entry"`
	Wompi            *BillingSetupWompi `json:"wompi,omitempty"`
}

type BillingSetupWompi struct {
	PublicKey                   string `json:"public_key"`
	APIURL                      string `json:"api_url"`
	TermsURL                    string `json:"terms_url"`
	PersonalDataURL             string `json:"personal_data_url"`
	AcceptanceToken             string `json:"acceptance_token"`
	PersonalDataAcceptanceToken string `json:"personal_data_acceptance_token"`
}

type BillingSelectionRequest struct {
	Plan   string `json:"plan"`
	Period string `json:"period"`
}

type BillingSelection struct {
	Plan     string `json:"plan"`
	Period   string `json:"period"`
	Amount   int    `json:"amount"`
	Currency string `json:"currency"`
}

// SaveCardRequest carries Wompi's token for the card, never the card itself.
type SaveCardRequest struct {
	CardToken                   string `json:"card_token"`
	AcceptanceToken             string `json:"acceptance_token"`
	PersonalDataAcceptanceToken string `json:"personal_data_acceptance_token"`
}

// BillingProviderPublic is what the browser needs to send a card to the provider.
type BillingProviderPublic struct {
	PublicKey string
	APIURL    string
}

type PaymentSourceRequest struct {
	CardToken         string
	CustomerEmail     string
	AcceptanceToken   string
	PersonalAuthToken string
}

type PaymentSource struct {
	ID       string
	Brand    string
	LastFour string
}

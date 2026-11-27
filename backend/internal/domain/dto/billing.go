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
	// PaidUntil is nil until the first approved charge.
	PaidUntil   *time.Time
	CancelledAt *time.Time
	// Attempts counts the scheduled charges tried for the current due date.
	Attempts  int
	CreatedAt time.Time
	UpdatedAt time.Time
}

func (b *OrganizationBilling) HasCard() bool {
	return b != nil && b.PaymentSourceID != nil && *b.PaymentSourceID != ""
}

func (b *OrganizationBilling) Cancelled() bool {
	return b != nil && b.CancelledAt != nil
}

type BillingChargeStatus string

const (
	// BillingChargeCreated is stored before the provider is called, so a crash can't lose a charge it accepted.
	BillingChargeCreated  BillingChargeStatus = "created"
	BillingChargePending  BillingChargeStatus = "pending"
	BillingChargeApproved BillingChargeStatus = "approved"
	BillingChargeDeclined BillingChargeStatus = "declined"
	BillingChargeVoided   BillingChargeStatus = "voided"
	BillingChargeError    BillingChargeStatus = "error"
)

func (s BillingChargeStatus) Open() bool {
	return s == BillingChargeCreated || s == BillingChargePending
}

// BillingCharge is one charge asked of the provider. Amount is whole pesos, as the price table states it.
type BillingCharge struct {
	ID             string
	OrganizationID string
	// DueOn is the due date the charge pays for.
	DueOn time.Time
	// Attempt is 1 to 3 for a scheduled try, and 0 for a recovery or new-card charge.
	Attempt       int
	Reference     string
	Plan          string
	Period        string
	Amount        int
	Currency      string
	Status        BillingChargeStatus
	TransactionID *string
	CoversFrom    *time.Time
	CoversUntil   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// DueSubscription is an organization whose due date has passed, read with its billing row locked.
type DueSubscription struct {
	OrganizationID     string
	OrganizationStatus string
	Plan               string
	Period             string
	PaymentSourceID    string
	// CustomerEmail is the organization's first admin with an email; the provider requires one.
	CustomerEmail string
	DueOn         time.Time
	Attempts      int
	CancelledAt   *time.Time
	HasOpenCharge bool
}

// BillingChargeRequest is what the provider needs to charge a saved payment source.
type BillingChargeRequest struct {
	PaymentSourceID string
	AmountInCents   int64
	Currency        string
	Reference       string
	CustomerEmail   string
}

// BillingTransaction is a transaction as the provider reports it, in an event or a lookup.
type BillingTransaction struct {
	ID            string
	Reference     string
	Status        BillingChargeStatus
	AmountInCents int64
	Currency      string
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

// BillingCardSetup is what the browser needs to send a card to the provider; Wompi is nil while card entry is off.
type BillingCardSetup struct {
	CardEntry bool               `json:"card_entry"`
	Wompi     *BillingSetupWompi `json:"wompi,omitempty"`
}

// SavedCard is all that is ever shown of a card: never the payment source.
type SavedCard struct {
	Brand    string `json:"brand"`
	LastFour string `json:"last_four"`
}

// SubscriptionCharge is one charge as its organization's admin sees it.
type SubscriptionCharge struct {
	ID          string     `json:"id"`
	Date        time.Time  `json:"date"`
	Amount      int        `json:"amount"`
	Currency    string     `json:"currency"`
	Status      string     `json:"status"`
	CoversFrom  *time.Time `json:"covers_from"`
	CoversUntil *time.Time `json:"covers_until"`
}

// Subscription is what an admin reads of their organization's billing. Without a saved card it has
// no card, no next charge and no charges.
type Subscription struct {
	Plan        string     `json:"plan"`
	Period      string     `json:"period"`
	Status      string     `json:"status"`
	TrialEndsAt *time.Time `json:"trial_ends_at"`
	PaidUntil   *time.Time `json:"paid_until"`
	// DueDate is the trial's end during the trial and the end of the paid period after.
	DueDate *time.Time `json:"due_date"`
	// NextChargeAmount is nil when nothing will be charged: no card, cancelled, or activated by hand.
	NextChargeAmount *int   `json:"next_charge_amount"`
	Currency         string `json:"currency"`
	Cancelled        bool   `json:"cancelled"`
	// PaymentFailed: the latest charge for the current due date failed and the organization still works.
	PaymentFailed bool `json:"payment_failed"`
	// AccessEndsAt is when the organization stops unless a charge is approved; nil when it never does.
	AccessEndsAt *time.Time           `json:"access_ends_at"`
	Card         *SavedCard           `json:"card"`
	LatestCharge *SubscriptionCharge  `json:"latest_charge"`
	Charges      []SubscriptionCharge `json:"charges"`
	CardEntry    bool                 `json:"card_entry"`
}

// BillingSummary is what the app's notice needs on every page, for an admin only.
type BillingSummary struct {
	Period           string     `json:"period"`
	HasCard          bool       `json:"has_card"`
	CardEntry        bool       `json:"card_entry"`
	Cancelled        bool       `json:"cancelled"`
	DueDate          *time.Time `json:"due_date"`
	NextChargeAmount *int       `json:"next_charge_amount"`
	PaymentFailed    bool       `json:"payment_failed"`
	AccessEndsAt     *time.Time `json:"access_ends_at"`
}

// RecoveryState is what a stopped organization's admin sees before paying their way back.
type RecoveryState struct {
	OrganizationName string `json:"organization_name"`
	// Reason is trial_expired or payment_required.
	Reason       string              `json:"reason"`
	Plan         string              `json:"plan"`
	Period       string              `json:"period"`
	AmountDue    int                 `json:"amount_due"`
	Currency     string              `json:"currency"`
	Card         *SavedCard          `json:"card"`
	LatestCharge *SubscriptionCharge `json:"latest_charge"`
	CardEntry    bool                `json:"card_entry"`
	Wompi        *BillingSetupWompi  `json:"wompi,omitempty"`
}

// RecoveryPayRequest carries a new card only when the person does not pay with the saved one.
type RecoveryPayRequest struct {
	CardToken                   string `json:"card_token"`
	AcceptanceToken             string `json:"acceptance_token"`
	PersonalDataAcceptanceToken string `json:"personal_data_acceptance_token"`
}

type RecoveryPayResponse struct {
	Charge SubscriptionCharge `json:"charge"`
}

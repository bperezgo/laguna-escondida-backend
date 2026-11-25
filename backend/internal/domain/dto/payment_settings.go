package dto

import "time"

type PaymentStrategy string

const (
	PaymentStrategyManual PaymentStrategy = "manual"
	PaymentStrategyWompi  PaymentStrategy = "wompi"
)

const (
	DefaultManualWindowMinutes = 60
	MinManualWindowMinutes     = 10
	MaxManualWindowMinutes     = 1440
	MaxManualInstructionsChars = 1000
)

// PaymentSettings is how an organization's agent orders are paid. An organization that never
// saved any reads as wompi with the default window.
type PaymentSettings struct {
	Strategy            PaymentStrategy `json:"strategy"`
	ManualInstructions  string          `json:"manual_instructions"`
	ManualWindowMinutes int             `json:"manual_window_minutes"`
}

// PaymentSettingsRequest is the PUT body; a missing window means the default.
type PaymentSettingsRequest struct {
	Strategy            string `json:"strategy"`
	ManualInstructions  string `json:"manual_instructions"`
	ManualWindowMinutes *int   `json:"manual_window_minutes"`
}

type PaymentQRStatus string

const (
	PaymentQRActive  PaymentQRStatus = "active"
	PaymentQRRetired PaymentQRStatus = "retired"
)

// PaymentQRTransaction is one Wompi BANCOLOMBIA_QR transaction an agent order had. URLToken is
// the random part of the public image URL; SVG is what Wompi returned, rendered to PNG on request.
type PaymentQRTransaction struct {
	ID                 string
	OrderID            string
	WompiTransactionID string
	Reference          string
	AmountInCents      int64
	Currency           string
	SVG                string
	URLToken           string
	Status             PaymentQRStatus
	CreatedAt          time.Time
	// OrderOpen is read with the QR: its order is neither paid nor voided.
	OrderOpen bool
}

// WompiMerchantTerms is what Wompi's merchant endpoint returns: the two documents the payer must accept
// and the single-use tokens that record the acceptance when a transaction is created.
type WompiMerchantTerms struct {
	TermsURL        string
	PersonalDataURL string
	// Email is the merchant's own: a QR transaction needs one and the WhatsApp customer gives none.
	Email string
	// AcceptanceToken and PersonalAuthToken are single use: read them fresh for every transaction.
	AcceptanceToken   string
	PersonalAuthToken string
}

// PaymentTerms are the two documents shown to the customer before a QR is created.
type PaymentTerms struct {
	TermsURL        string `json:"terms_url"`
	PersonalDataURL string `json:"personal_data_url"`
}

// QRTransactionRequest is what Wompi needs to create a BANCOLOMBIA_QR transaction.
type QRTransactionRequest struct {
	AcceptanceToken   string
	PersonalAuthToken string
	AmountInCents     int64
	Currency          string
	Reference         string
	CustomerEmail     string
	Signature         string
	ExpiresAt         *time.Time
}

// WompiTimestamp is the format Wompi's expiration_time takes, and its signature must repeat.
func WompiTimestamp(t time.Time) string {
	return t.UTC().Format("2006-01-02T15:04:05.000Z")
}

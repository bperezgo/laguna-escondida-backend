package httpclient

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

const wompiSourceAvailable = "AVAILABLE"

// WompiBilling is Wompi with the platform's own keys: the account restaurants pay their plan to.
type WompiBilling struct {
	wompi      *WompiClient
	publicKey  string
	privateKey string
	// integritySecret is empty while charging is off; Charge then refuses.
	integritySecret string
}

func NewWompiBilling(client *Client, baseURL, publicKey, privateKey, integritySecret string) ports.BillingProvider {
	return &WompiBilling{wompi: &WompiClient{client: client, baseURL: strings.TrimRight(baseURL, "/")},
		publicKey: publicKey, privateKey: privateKey, integritySecret: integritySecret}
}

func (b *WompiBilling) Terms(ctx context.Context) (dto.WompiMerchantTerms, error) {
	return b.wompi.MerchantTerms(ctx, b.publicKey)
}

type wompiCreatePaymentSource struct {
	Type               string `json:"type"`
	Token              string `json:"token"`
	CustomerEmail      string `json:"customer_email"`
	AcceptanceToken    string `json:"acceptance_token"`
	AcceptPersonalAuth string `json:"accept_personal_auth"`
}

type wompiPaymentSourceResponse struct {
	Data struct {
		// A number today; kept raw so a string id would work too.
		ID         json.RawMessage `json:"id"`
		Status     string          `json:"status"`
		PublicData struct {
			Brand    string `json:"brand"`
			Bin      string `json:"bin"`
			LastFour string `json:"last_four"`
		} `json:"public_data"`
	} `json:"data"`
}

// SavePaymentSource charges nothing. A 4xx other than a key problem is Wompi refusing the card token.
func (b *WompiBilling) SavePaymentSource(ctx context.Context, req dto.PaymentSourceRequest) (dto.PaymentSource, error) {
	body, err := json.Marshal(wompiCreatePaymentSource{
		Type: "CARD", Token: req.CardToken, CustomerEmail: req.CustomerEmail,
		AcceptanceToken: req.AcceptanceToken, AcceptPersonalAuth: req.PersonalAuthToken,
	})
	if err != nil {
		return dto.PaymentSource{}, fmt.Errorf("encode Wompi payment source: %w", err)
	}
	raw, err := b.wompi.call(ctx, http.MethodPost, "/payment_sources", b.privateKey, body)
	if err != nil {
		var statusErr *wompiStatusError
		if errors.As(err, &statusErr) && cardRefusalStatus(statusErr.status) {
			return dto.PaymentSource{}, fmt.Errorf("%w: create Wompi payment source: %w", domainError.ErrCardRefused, err)
		}
		return dto.PaymentSource{}, fmt.Errorf("%w: create Wompi payment source: %w", domainError.ErrPaymentProviderUnavailable, err)
	}
	var created wompiPaymentSourceResponse
	id := ""
	if err := json.Unmarshal(raw, &created); err == nil {
		id = strings.Trim(string(created.Data.ID), `"`)
	}
	if id == "" || id == "null" {
		return dto.PaymentSource{}, fmt.Errorf("%w: create Wompi payment source: unexpected answer %q", domainError.ErrPaymentProviderUnavailable, truncate(raw))
	}
	if created.Data.Status != wompiSourceAvailable {
		return dto.PaymentSource{}, fmt.Errorf("%w: Wompi payment source %s is %s", domainError.ErrCardRefused, id, created.Data.Status)
	}
	public := created.Data.PublicData
	brand := strings.ToUpper(public.Brand)
	if brand == "" {
		brand = cardBrandFromBin(public.Bin)
	}
	return dto.PaymentSource{ID: id, Brand: brand, LastFour: lastFourDigits(public.LastFour)}, nil
}

// 401 and 403 are our keys, not the card; 404 and 422 are how Wompi refuses a token.
func cardRefusalStatus(status int) bool {
	return status == http.StatusNotFound || status == http.StatusUnprocessableEntity || status == http.StatusBadRequest
}

func cardBrandFromBin(bin string) string {
	switch {
	case strings.HasPrefix(bin, "4"):
		return "VISA"
	case len(bin) >= 2 && bin[:2] >= "51" && bin[:2] <= "55", len(bin) >= 4 && bin[:4] >= "2221" && bin[:4] <= "2720":
		return "MASTERCARD"
	case strings.HasPrefix(bin, "34"), strings.HasPrefix(bin, "37"):
		return "AMEX"
	default:
		return ""
	}
}

// lastFourDigits keeps the column's four digits and drops anything else Wompi might send.
func lastFourDigits(value string) string {
	if len(value) != 4 {
		return ""
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return ""
		}
	}
	return value
}

type wompiChargeSource struct {
	AmountInCents int64  `json:"amount_in_cents"`
	Currency      string `json:"currency"`
	Signature     string `json:"signature"`
	CustomerEmail string `json:"customer_email"`
	// Wompi asks for the installments of a card source; a subscription is always one.
	PaymentMethod   wompiInstallments `json:"payment_method"`
	Reference       string            `json:"reference"`
	PaymentSourceID json.RawMessage   `json:"payment_source_id"`
}

type wompiInstallments struct {
	Installments int `json:"installments"`
}

type wompiBillingTransaction struct {
	ID            string `json:"id"`
	Reference     string `json:"reference"`
	Status        string `json:"status"`
	AmountInCents int64  `json:"amount_in_cents"`
	Currency      string `json:"currency"`
}

// Charge signs the request with the integrity secret: reference, amount in cents and currency.
func (b *WompiBilling) Charge(ctx context.Context, req dto.BillingChargeRequest) (dto.BillingTransaction, error) {
	if b.integritySecret == "" {
		return dto.BillingTransaction{}, errors.New("charge Wompi payment source: no integrity secret")
	}
	sum := sha256.Sum256([]byte(req.Reference + strconv.FormatInt(req.AmountInCents, 10) + req.Currency + b.integritySecret))
	body, err := json.Marshal(wompiChargeSource{
		AmountInCents: req.AmountInCents, Currency: req.Currency, Signature: hex.EncodeToString(sum[:]),
		CustomerEmail: req.CustomerEmail, PaymentMethod: wompiInstallments{Installments: 1},
		Reference: req.Reference, PaymentSourceID: paymentSourceJSON(req.PaymentSourceID),
	})
	if err != nil {
		return dto.BillingTransaction{}, fmt.Errorf("encode Wompi charge: %w", err)
	}
	raw, err := b.wompi.call(ctx, http.MethodPost, "/transactions", b.privateKey, body)
	if err != nil {
		var statusErr *wompiStatusError
		if errors.As(err, &statusErr) && cardRefusalStatus(statusErr.status) {
			return dto.BillingTransaction{}, fmt.Errorf("%w: charge Wompi payment source: %w", domainError.ErrCardRefused, err)
		}
		return dto.BillingTransaction{}, fmt.Errorf("%w: charge Wompi payment source: %w", domainError.ErrPaymentProviderUnavailable, err)
	}
	var created struct {
		Data wompiBillingTransaction `json:"data"`
	}
	if err := json.Unmarshal(raw, &created); err != nil || created.Data.ID == "" {
		return dto.BillingTransaction{}, fmt.Errorf("%w: charge Wompi payment source: unexpected answer %q", domainError.ErrPaymentProviderUnavailable, truncate(raw))
	}
	tx := created.Data.toDTO()
	if tx.Reference == "" {
		tx.Reference = req.Reference
	}
	return tx, nil
}

// paymentSourceJSON sends the id as the number Wompi issued it as, and as a string otherwise.
func paymentSourceJSON(id string) json.RawMessage {
	if _, err := strconv.ParseInt(id, 10, 64); err == nil {
		return json.RawMessage(id)
	}
	quoted, _ := json.Marshal(id)
	return quoted
}

func (b *WompiBilling) Transaction(ctx context.Context, id string) (dto.BillingTransaction, error) {
	raw, err := b.wompi.call(ctx, http.MethodGet, "/transactions/"+url.PathEscape(id), b.privateKey, nil)
	if err != nil {
		var statusErr *wompiStatusError
		if errors.As(err, &statusErr) && statusErr.status == http.StatusNotFound {
			return dto.BillingTransaction{}, fmt.Errorf("%w: %s", domainError.ErrBillingTransactionNotFound, id)
		}
		return dto.BillingTransaction{}, fmt.Errorf("%w: read Wompi transaction %s: %w", domainError.ErrPaymentProviderUnavailable, id, err)
	}
	var found struct {
		Data wompiBillingTransaction `json:"data"`
	}
	if err := json.Unmarshal(raw, &found); err != nil || found.Data.ID == "" {
		return dto.BillingTransaction{}, fmt.Errorf("%w: read Wompi transaction %s: unexpected answer %q", domainError.ErrPaymentProviderUnavailable, id, truncate(raw))
	}
	return found.Data.toDTO(), nil
}

// TransactionByReference takes the transaction whose reference is exactly the one asked for; a
// reference is ours and unique, so there is at most one.
func (b *WompiBilling) TransactionByReference(ctx context.Context, reference string) (dto.BillingTransaction, error) {
	raw, err := b.wompi.call(ctx, http.MethodGet, "/transactions?reference="+url.QueryEscape(reference), b.privateKey, nil)
	if err != nil {
		// Even a 404: only an answered, empty list says the transaction does not exist, since
		// "not found" frees the charge for another try.
		return dto.BillingTransaction{}, fmt.Errorf("%w: find Wompi transaction by reference: %w", domainError.ErrPaymentProviderUnavailable, err)
	}
	var found struct {
		Data []wompiBillingTransaction `json:"data"`
	}
	if err := json.Unmarshal(raw, &found); err != nil {
		return dto.BillingTransaction{}, fmt.Errorf("%w: find Wompi transaction by reference: unexpected answer %q", domainError.ErrPaymentProviderUnavailable, truncate(raw))
	}
	for _, tx := range found.Data {
		if tx.Reference == reference && tx.ID != "" {
			return tx.toDTO(), nil
		}
	}
	return dto.BillingTransaction{}, fmt.Errorf("%w: reference %s", domainError.ErrBillingTransactionNotFound, reference)
}

func (t wompiBillingTransaction) toDTO() dto.BillingTransaction {
	return dto.BillingTransaction{ID: t.ID, Reference: t.Reference, Status: billingChargeStatus(t.Status),
		AmountInCents: t.AmountInCents, Currency: t.Currency}
}

// billingChargeStatus keeps an unknown status pending, so the lookup asks again instead of failing a charge.
func billingChargeStatus(wompiStatus string) dto.BillingChargeStatus {
	switch strings.ToUpper(wompiStatus) {
	case "APPROVED":
		return dto.BillingChargeApproved
	case "DECLINED":
		return dto.BillingChargeDeclined
	case "VOIDED":
		return dto.BillingChargeVoided
	case "ERROR":
		return dto.BillingChargeError
	default:
		return dto.BillingChargePending
	}
}

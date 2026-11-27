package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
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
}

func NewWompiBilling(client *Client, baseURL, publicKey, privateKey string) ports.BillingProvider {
	return &WompiBilling{wompi: &WompiClient{client: client, baseURL: strings.TrimRight(baseURL, "/")}, publicKey: publicKey, privateKey: privateKey}
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

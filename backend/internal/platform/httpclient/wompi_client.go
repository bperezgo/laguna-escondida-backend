package httpclient

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

const (
	wompiTimeout = 10 * time.Second
	// A QR transaction answers with the QR's SVG in base64, about 150 KB.
	wompiMaxAnswer = 1 << 20
	// Sandbox and production links both open on this checkout.
	wompiCheckoutURL = "https://checkout.wompi.co/l/"
)

// WompiClient is Wompi's payment links API; each call carries the organization's private key.
type WompiClient struct {
	client  *Client
	baseURL string
}

func NewWompiClient(client *Client, baseURL string) ports.PaymentLinkProvider {
	return &WompiClient{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

// NewWompiQRClient is the same client as the links', for Wompi's QR transactions.
func NewWompiQRClient(client *Client, baseURL string) ports.PaymentQRProvider {
	return &WompiClient{client: client, baseURL: strings.TrimRight(baseURL, "/")}
}

var (
	_ ports.PaymentLinkProvider = (*WompiClient)(nil)
	_ ports.PaymentQRProvider   = (*WompiClient)(nil)
)

type wompiCreateLink struct {
	Name            string `json:"name"`
	Description     string `json:"description"`
	SingleUse       bool   `json:"single_use"`
	CollectShipping bool   `json:"collect_shipping"`
	Currency        string `json:"currency"`
	AmountInCents   int64  `json:"amount_in_cents"`
	ExpiresAt       string `json:"expires_at"`
	SKU             string `json:"sku"`
}

type wompiLinkResponse struct {
	Data struct {
		ID string `json:"id"`
	} `json:"data"`
}

func (c *WompiClient) CreateLink(ctx context.Context, privateKey string, req dto.PaymentLinkRequest) (string, error) {
	body, err := json.Marshal(wompiCreateLink{
		Name:          req.Name,
		Description:   req.Description,
		SingleUse:     true,
		Currency:      req.Currency,
		AmountInCents: req.AmountInCents,
		ExpiresAt:     req.ExpiresAt.UTC().Format(time.RFC3339),
		SKU:           req.SKU,
	})
	if err != nil {
		return "", fmt.Errorf("encode Wompi payment link: %w", err)
	}
	raw, err := c.call(ctx, http.MethodPost, "/payment_links", privateKey, body)
	if err != nil {
		return "", fmt.Errorf("create Wompi payment link: %w", err)
	}
	var created wompiLinkResponse
	if err := json.Unmarshal(raw, &created); err != nil || created.Data.ID == "" {
		return "", fmt.Errorf("create Wompi payment link: unexpected answer %q", truncate(raw))
	}
	return created.Data.ID, nil
}

func (c *WompiClient) DeactivateLink(ctx context.Context, privateKey, linkID string) error {
	if _, err := c.call(ctx, http.MethodPatch, "/payment_links/"+url.PathEscape(linkID), privateKey, []byte(`{"active":false}`)); err != nil {
		return fmt.Errorf("deactivate Wompi payment link %s: %w", linkID, err)
	}
	return nil
}

func (c *WompiClient) CheckoutURL(linkID string) string {
	return wompiCheckoutURL + linkID
}

type wompiMerchantResponse struct {
	Data struct {
		Email        string          `json:"email"`
		Acceptance   wompiAcceptance `json:"presigned_acceptance"`
		PersonalData wompiAcceptance `json:"presigned_personal_data_auth"`
	} `json:"data"`
}

type wompiAcceptance struct {
	AcceptanceToken string `json:"acceptance_token"`
	Permalink       string `json:"permalink"`
}

// MerchantTerms reads the two documents and fresh acceptance tokens; the public key is the merchant's id.
func (c *WompiClient) MerchantTerms(ctx context.Context, publicKey string) (dto.WompiMerchantTerms, error) {
	raw, err := c.call(ctx, http.MethodGet, "/merchants/"+url.PathEscape(publicKey), "", nil)
	if err != nil {
		return dto.WompiMerchantTerms{}, fmt.Errorf("read Wompi merchant: %w", err)
	}
	var merchant wompiMerchantResponse
	if err := json.Unmarshal(raw, &merchant); err != nil {
		return dto.WompiMerchantTerms{}, fmt.Errorf("read Wompi merchant: unexpected answer %q", truncate(raw))
	}
	data := merchant.Data
	if data.Acceptance.AcceptanceToken == "" || data.PersonalData.AcceptanceToken == "" ||
		data.Acceptance.Permalink == "" || data.PersonalData.Permalink == "" {
		return dto.WompiMerchantTerms{}, errors.New("read Wompi merchant: the terms or their tokens are missing")
	}
	return dto.WompiMerchantTerms{
		TermsURL: data.Acceptance.Permalink, PersonalDataURL: data.PersonalData.Permalink, Email: data.Email,
		AcceptanceToken: data.Acceptance.AcceptanceToken, PersonalAuthToken: data.PersonalData.AcceptanceToken,
	}, nil
}

type wompiQRPaymentMethod struct {
	Type               string `json:"type"`
	PaymentDescription string `json:"payment_description"`
	SandboxStatus      string `json:"sandbox_status,omitempty"`
}

type wompiCreateTransaction struct {
	AcceptanceToken   string               `json:"acceptance_token"`
	AcceptPersonalAut string               `json:"accept_personal_auth"`
	AmountInCents     int64                `json:"amount_in_cents"`
	Currency          string               `json:"currency"`
	Signature         string               `json:"signature"`
	Reference         string               `json:"reference"`
	CustomerEmail     string               `json:"customer_email"`
	ExpirationTime    string               `json:"expiration_time,omitempty"`
	PaymentMethod     wompiQRPaymentMethod `json:"payment_method"`
}

type wompiTransactionResponse struct {
	Data struct {
		ID            string `json:"id"`
		PaymentMethod struct {
			Extra struct {
				QRImage string `json:"qr_image"`
			} `json:"extra"`
		} `json:"payment_method"`
	} `json:"data"`
}

func (c *WompiClient) CreateQRTransaction(ctx context.Context, privateKey string, req dto.QRTransactionRequest) (string, error) {
	create := wompiCreateTransaction{
		AcceptanceToken: req.AcceptanceToken, AcceptPersonalAut: req.PersonalAuthToken, AmountInCents: req.AmountInCents,
		Currency: req.Currency, Signature: req.Signature, Reference: req.Reference, CustomerEmail: req.CustomerEmail,
		PaymentMethod: wompiQRPaymentMethod{Type: "BANCOLOMBIA_QR", PaymentDescription: "Pedido " + req.Reference, SandboxStatus: c.sandboxStatus()},
	}
	if req.ExpiresAt != nil {
		create.ExpirationTime = dto.WompiTimestamp(*req.ExpiresAt)
	}
	body, err := json.Marshal(create)
	if err != nil {
		return "", fmt.Errorf("encode Wompi QR transaction: %w", err)
	}
	raw, err := c.call(ctx, http.MethodPost, "/transactions", privateKey, body)
	if err != nil {
		return "", fmt.Errorf("create Wompi QR transaction: %w", err)
	}
	var created wompiTransactionResponse
	if err := json.Unmarshal(raw, &created); err != nil || created.Data.ID == "" {
		return "", fmt.Errorf("create Wompi QR transaction: unexpected answer %q", truncate(raw))
	}
	return created.Data.ID, nil
}

// sandboxStatus: Wompi's sandbox refuses a transaction without the outcome to simulate.
func (c *WompiClient) sandboxStatus() string {
	if strings.Contains(c.baseURL, "sandbox.wompi.co") {
		return "APPROVED"
	}
	return ""
}

func (c *WompiClient) QRImage(ctx context.Context, privateKey, transactionID string) (string, bool, error) {
	raw, err := c.call(ctx, http.MethodGet, "/transactions/"+url.PathEscape(transactionID), privateKey, nil)
	if err != nil {
		return "", false, fmt.Errorf("read Wompi transaction %s: %w", transactionID, err)
	}
	var tx wompiTransactionResponse
	if json.Unmarshal(raw, &tx) != nil {
		return "", false, fmt.Errorf("read Wompi transaction %s: unexpected answer %q", transactionID, truncate(raw))
	}
	encoded := tx.Data.PaymentMethod.Extra.QRImage
	if encoded == "" {
		return "", false, nil
	}
	svg, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", false, fmt.Errorf("read Wompi transaction %s: the QR is not base64: %w", transactionID, err)
	}
	return string(svg), true, nil
}

func (c *WompiClient) call(ctx context.Context, method, path, privateKey string, body []byte) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, wompiTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if privateKey != "" {
		req.Header.Set("Authorization", "Bearer "+privateKey)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, wompiMaxAnswer))
	if err != nil {
		return nil, fmt.Errorf("read answer: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, &wompiStatusError{status: resp.StatusCode, body: truncate(raw)}
	}
	return raw, nil
}

// wompiStatusError keeps the status so a caller can tell a refusal from an outage.
type wompiStatusError struct {
	status int
	body   string
}

func (e *wompiStatusError) Error() string { return fmt.Sprintf("status %d: %s", e.status, e.body) }

func truncate(raw []byte) string {
	const limit = 512
	if len(raw) > limit {
		return string(raw[:limit]) + "…"
	}
	return string(raw)
}

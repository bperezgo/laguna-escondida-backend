package service

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/google/uuid"
)

const (
	qrPollInterval  = 500 * time.Millisecond
	qrTermsCacheTTL = time.Hour
	qrTokenBytes    = 32
	qrPathPrefix    = "/api/payments/qr/"
)

type cachedTerms struct {
	terms dto.PaymentTerms
	at    time.Time
}

// PaymentQRService gives a wompi agent order a Bancolombia QR beside its link. It is best effort
// by design: any failure leaves the order with its link, and a payment is matched by the
// transaction's reference, so a QR whose image never arrived can still be paid.
type PaymentQRService struct {
	qrs       ports.PaymentQRRepository
	provider  ports.PaymentQRProvider
	config    ports.PaymentLinkConfig
	orgs      ports.OrganizationRepository
	publicURL string
	wait      time.Duration
	pollEvery time.Duration
	logger    *slog.Logger
	now       func() time.Time
	newToken  func() (string, error)

	mu    sync.Mutex
	terms map[string]cachedTerms
}

func NewPaymentQRService(
	qrs ports.PaymentQRRepository,
	provider ports.PaymentQRProvider,
	config ports.PaymentLinkConfig,
	orgs ports.OrganizationRepository,
	publicURL string,
	wait time.Duration,
	logger *slog.Logger,
	now func() time.Time,
) *PaymentQRService {
	if now == nil {
		now = time.Now
	}
	return &PaymentQRService{
		qrs: qrs, provider: provider, config: config, orgs: orgs, publicURL: strings.TrimRight(publicURL, "/"), wait: wait,
		pollEvery: qrPollInterval, logger: logger, now: now, newToken: randomToken, terms: map[string]cachedTerms{},
	}
}

func randomToken() (string, error) {
	raw := make([]byte, qrTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

// Terms are Wompi's two documents when the context's organization can offer a QR; nil otherwise,
// and when Wompi does not answer. Cached for an hour: the agent asks before every order.
func (s *PaymentQRService) Terms(ctx context.Context) *dto.PaymentTerms {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil
	}
	s.mu.Lock()
	cached, ok := s.terms[orgID]
	s.mu.Unlock()
	if ok && s.now().Sub(cached.at) < qrTermsCacheTTL {
		return &cached.terms
	}
	creds, offered := s.credentials(ctx, orgID)
	if !offered {
		return nil
	}
	merchant, err := s.provider.MerchantTerms(ctx, creds.PublicKey)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment terms not read from Wompi", slog.String("organization_id", orgID), slog.Any("error", err))
		return nil
	}
	terms := dto.PaymentTerms{TermsURL: merchant.TermsURL, PersonalDataURL: merchant.PersonalDataURL}
	s.mu.Lock()
	s.terms[orgID] = cachedTerms{terms: terms, at: s.now()}
	s.mu.Unlock()
	return &terms
}

// credentials are the Wompi keys when the organization holds the QR pair.
func (s *PaymentQRService) credentials(ctx context.Context, orgID string) (dto.WompiCredentials, bool) {
	creds, configured, err := s.config.For(ctx, orgID)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment QR skipped: read the Wompi integration", slog.String("organization_id", orgID), slog.Any("error", err))
		return dto.WompiCredentials{}, false
	}
	return creds, configured && creds.QREnabled()
}

// CreateForOrder makes the order's first QR and returns its public URL, or nil: the customer did not
// accept the terms, the organization cannot offer a QR, Wompi refused it or it has not arrived yet.
func (s *PaymentQRService) CreateForOrder(ctx context.Context, order *dto.OpenBill, termsAccepted bool, expiresAt time.Time) *string {
	if !termsAccepted {
		return nil
	}
	return s.create(ctx, order, expiresAt)
}

// ReplaceForOrder retires the order's active QR and makes one for its new total; an order that had
// none gets none, so the customer is never asked to accept the terms again.
func (s *PaymentQRService) ReplaceForOrder(ctx context.Context, order *dto.OpenBill, expiresAt time.Time) *string {
	_, found, err := s.qrs.FindActiveByOrder(ctx, order.ID)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment QR not replaced: read the active QR", slog.String("order_id", order.ID), slog.Any("error", err))
		return nil
	}
	if !found {
		return nil
	}
	s.Retire(ctx, order.ID)
	if !s.now().Before(expiresAt) {
		return nil
	}
	return s.create(ctx, order, expiresAt)
}

// Retire stops the order's QR from paying it and from being served. Wompi cannot cancel a pending
// QR transaction, so a payment made on a retired QR is stored unmatched and refunded by staff.
func (s *PaymentQRService) Retire(ctx context.Context, orderID string) {
	if err := s.qrs.RetireByOrder(ctx, orderID); err != nil {
		s.logger.ErrorContext(ctx, "payment QR not retired", slog.String("order_id", orderID), slog.Any("error", err))
	}
}

// ActiveURL is the public image URL of the order's active QR. A QR that had not arrived is read from
// Wompi once now; nil while it is still missing.
func (s *PaymentQRService) ActiveURL(ctx context.Context, orderID string) *string {
	qr, found, err := s.qrs.FindActiveByOrder(ctx, orderID)
	if err != nil {
		s.logger.ErrorContext(ctx, "payment QR not read", slog.String("order_id", orderID), slog.Any("error", err))
		return nil
	}
	if !found {
		return nil
	}
	if qr.SVG == "" && !s.fetchOnce(ctx, qr) {
		return nil
	}
	return s.url(qr)
}

// ServeSVG is the QR behind a public token; found=false for an unknown or retired token and for a QR not arrived.
func (s *PaymentQRService) ServeSVG(ctx context.Context, token string) (svg string, found bool, err error) {
	qr, found, err := s.qrs.FindActiveByToken(ctx, token)
	if err != nil || !found || qr.SVG == "" {
		return "", false, err
	}
	return qr.SVG, true, nil
}

func (s *PaymentQRService) url(qr *dto.PaymentQRTransaction) *string {
	url := s.publicURL + qrPathPrefix + qr.URLToken
	return &url
}

func (s *PaymentQRService) create(ctx context.Context, order *dto.OpenBill, expiresAt time.Time) *string {
	log := s.logger.With(slog.String("order_id", order.ID))
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil
	}
	creds, offered := s.credentials(ctx, orgID)
	if !offered {
		return nil
	}
	org, err := s.orgs.Get(ctx, orgID)
	if err != nil {
		log.ErrorContext(ctx, "payment QR not created: read the organization", slog.Any("error", err))
		return nil
	}
	merchant, err := s.provider.MerchantTerms(ctx, creds.PublicKey)
	if err != nil {
		log.ErrorContext(ctx, "payment QR not created: Wompi's terms not read", slog.Any("error", err))
		return nil
	}
	token, err := s.newToken()
	if err != nil {
		log.ErrorContext(ctx, "payment QR not created: no random token", slog.Any("error", err))
		return nil
	}

	amount := amountInCents(order.TotalAmount)
	reference := qrReference(order.ID)
	transactionID, err := s.provider.CreateQRTransaction(ctx, creds.PrivateKey, dto.QRTransactionRequest{
		AcceptanceToken: merchant.AcceptanceToken, PersonalAuthToken: merchant.PersonalAuthToken,
		AmountInCents: amount, Currency: org.Currency, Reference: reference, CustomerEmail: merchant.Email,
		Signature: qrSignature(reference, amount, org.Currency, expiresAt, creds.IntegritySecret), ExpiresAt: &expiresAt,
	})
	if err != nil {
		log.ErrorContext(ctx, "payment QR not created: Wompi refused it", slog.Any("error", err))
		return nil
	}
	qr := &dto.PaymentQRTransaction{
		ID: uuid.Must(uuid.NewV7()).String(), OrderID: order.ID, WompiTransactionID: transactionID, Reference: reference,
		AmountInCents: amount, Currency: org.Currency, URLToken: token, Status: dto.PaymentQRActive, CreatedAt: s.now(),
	}
	svg, arrived := s.waitForImage(ctx, creds.PrivateKey, transactionID)
	if arrived {
		qr.SVG = svg
	}
	if err := s.qrs.Create(ctx, qr); err != nil {
		log.ErrorContext(ctx, "payment QR created at Wompi but not stored: nobody received it",
			slog.String("wompi_transaction_id", transactionID), slog.Any("error", err))
		return nil
	}
	if !arrived {
		log.WarnContext(ctx, "payment QR not ready in time: the order's next read gets it", slog.String("wompi_transaction_id", transactionID))
		return nil
	}
	return s.url(qr)
}

// waitForImage polls Wompi until it returns the image or the wait ends; a failed read counts as not arrived.
func (s *PaymentQRService) waitForImage(ctx context.Context, privateKey, transactionID string) (string, bool) {
	deadline := time.Now().Add(s.wait)
	for {
		svg, ready, err := s.provider.QRImage(ctx, privateKey, transactionID)
		if err == nil && ready {
			return svg, true
		}
		if err != nil {
			s.logger.WarnContext(ctx, "payment QR image not read yet", slog.String("wompi_transaction_id", transactionID), slog.Any("error", err))
		}
		if !time.Now().Add(s.pollEvery).Before(deadline) {
			return "", false
		}
		select {
		case <-ctx.Done():
			return "", false
		case <-time.After(s.pollEvery):
		}
	}
}

// fetchOnce reads a late QR from Wompi and stores it; false while it is still missing.
func (s *PaymentQRService) fetchOnce(ctx context.Context, qr *dto.PaymentQRTransaction) bool {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return false
	}
	creds, offered := s.credentials(ctx, orgID)
	if !offered {
		return false
	}
	svg, ready, err := s.provider.QRImage(ctx, creds.PrivateKey, qr.WompiTransactionID)
	if err != nil {
		s.logger.ErrorContext(ctx, "late payment QR not read", slog.String("order_id", qr.OrderID), slog.Any("error", err))
		return false
	}
	if !ready {
		return false
	}
	if err := s.qrs.SetSVG(ctx, qr.ID, svg); err != nil {
		s.logger.ErrorContext(ctx, "late payment QR not stored", slog.String("order_id", qr.OrderID), slog.Any("error", err))
		return false
	}
	qr.SVG = svg
	return true
}

// qrReference is unique per QR: Wompi never lets a reference be reused, and a changed order needs a new one.
func qrReference(orderID string) string {
	return "qr-" + strings.ReplaceAll(orderID, "-", "") + "-" + strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
}

// qrSignature is Wompi's integrity signature: SHA-256 of reference, amount, currency, the expiry when
// there is one, and the organization's integrity secret.
func qrSignature(reference string, amountInCents int64, currency string, expiresAt time.Time, integritySecret string) string {
	sum := sha256.Sum256([]byte(reference + strconv.FormatInt(amountInCents, 10) + currency + dto.WompiTimestamp(expiresAt) + integritySecret))
	return hex.EncodeToString(sum[:])
}

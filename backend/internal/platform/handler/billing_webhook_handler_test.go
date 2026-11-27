package handler

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const billingEventsSecret = "test_events_platform"

type recordingSettler struct {
	settled []dto.BillingTransaction
	err     error
}

func (r *recordingSettler) Settle(_ context.Context, tx dto.BillingTransaction) error {
	r.settled = append(r.settled, tx)
	return r.err
}

func billingWebhookRouter(settler *recordingSettler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.DiscardHandler)
	router := gin.New()
	router.POST("/api/webhooks/wompi-billing",
		NewBillingWebhookHandler(service.NewBillingWebhookService(settler, billingEventsSecret, logger), logger).WompiHandler)
	return router
}

func postBillingEvent(router *gin.Engine, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/webhooks/wompi-billing", bytes.NewBufferString(body)))
	return rec
}

// billingEvent is a transaction.updated event signed as Wompi signs it, under secret.
func billingEvent(status string, amountInCents int64, secret string) string {
	const id, timestamp = "1234-1610641025-49201", 1530291411
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s%s%d%d%s", id, status, amountInCents, timestamp, secret)))
	return fmt.Sprintf(`{"event":"transaction.updated","data":{"transaction":{"id":%q,"status":%q,"amount_in_cents":%d,`+
		`"currency":"COP","reference":"kb-org-20261215-1","payment_method_type":"CARD","payment_source_id":3891}},`+
		`"signature":{"properties":["transaction.id","transaction.status","transaction.amount_in_cents"],"checksum":%q},"timestamp":%d}`,
		id, status, amountInCents, strings.ToUpper(hex.EncodeToString(sum[:])), timestamp)
}

func TestBillingWebhookHandler_ASignedApprovalSettles(t *testing.T) {
	settler := &recordingSettler{}

	rec := postBillingEvent(billingWebhookRouter(settler), billingEvent("APPROVED", 4000000, billingEventsSecret))

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Len(t, settler.settled, 1)
	assert.Equal(t, dto.BillingTransaction{ID: "1234-1610641025-49201", Reference: "kb-org-20261215-1",
		Status: dto.BillingChargeApproved, AmountInCents: 4000000, Currency: "COP"}, settler.settled[0])
}

func TestBillingWebhookHandler_StatusesReachSettleAsChargeStatuses(t *testing.T) {
	cases := map[string]dto.BillingChargeStatus{
		"DECLINED": dto.BillingChargeDeclined, "VOIDED": dto.BillingChargeVoided, "ERROR": dto.BillingChargeError, "PENDING": dto.BillingChargePending,
	}
	for wompi, want := range cases {
		t.Run(wompi, func(t *testing.T) {
			settler := &recordingSettler{}

			rec := postBillingEvent(billingWebhookRouter(settler), billingEvent(wompi, 4000000, billingEventsSecret))

			require.Equal(t, http.StatusOK, rec.Code)
			require.Len(t, settler.settled, 1)
			assert.Equal(t, want, settler.settled[0].Status)
		})
	}
}

func TestBillingWebhookHandler_Refused(t *testing.T) {
	signed := billingEvent("APPROVED", 4000000, billingEventsSecret)
	cases := []struct {
		name string
		body string
	}{
		{"a wrong checksum", billingEvent("APPROVED", 4000000, "test_events_someone_else")},
		{"an amount changed after signing", strings.Replace(signed, `"amount_in_cents":4000000`, `"amount_in_cents":100`, 1)},
		{"a status changed after signing", strings.Replace(signed, `"status":"APPROVED"`, `"status":"DECLINED"`, 1)},
		{"no signature", `{"event":"transaction.updated","data":{"transaction":{"id":"tx_1","status":"APPROVED","reference":"kb-org-20261215-1"}},"timestamp":1530291411}`},
		{"a signature that names no property", `{"event":"transaction.updated","data":{"transaction":{"id":"tx_1","status":"APPROVED"}},` +
			`"signature":{"properties":[],"checksum":"00"},"timestamp":1530291411}`},
		{"not JSON", `checksum=1`},
		{"an empty body", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			settler := &recordingSettler{}

			rec := postBillingEvent(billingWebhookRouter(settler), tc.body)

			assert.Equal(t, http.StatusUnauthorized, rec.Code)
			assert.Empty(t, settler.settled, "no charge changes")
		})
	}
}

func TestBillingWebhookHandler_AnotherEventTypeIs200AndChangesNothing(t *testing.T) {
	sum := sha256.Sum256([]byte("tok_1" + "1530291411" + billingEventsSecret))
	body := fmt.Sprintf(`{"event":"nequi_token.updated","data":{"nequi_token":{"id":"tok_1"}},`+
		`"signature":{"properties":["nequi_token.id"],"checksum":%q},"timestamp":1530291411}`, hex.EncodeToString(sum[:]))
	settler := &recordingSettler{}

	rec := postBillingEvent(billingWebhookRouter(settler), body)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Empty(t, settler.settled)
}

func TestBillingWebhookHandler_AnUnknownReferenceIs200(t *testing.T) {
	settler := &recordingSettler{err: domainError.ErrChargeNotFound}

	rec := postBillingEvent(billingWebhookRouter(settler), billingEvent("APPROVED", 4000000, billingEventsSecret))

	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestBillingWebhookHandler_AVerifiedEventThatFailsToApplyIs500(t *testing.T) {
	settler := &recordingSettler{err: errors.New("database down")}

	rec := postBillingEvent(billingWebhookRouter(settler), billingEvent("APPROVED", 4000000, billingEventsSecret))

	assert.Equal(t, http.StatusInternalServerError, rec.Code, "so Wompi sends it again")
}

func TestBillingWebhookHandler_AnOversizedBodyIsRefused(t *testing.T) {
	settler := &recordingSettler{}

	rec := postBillingEvent(billingWebhookRouter(settler), `{"event":"`+strings.Repeat("a", wompiEventMaxBytes+10)+`"}`)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
	assert.Empty(t, settler.settled)
}

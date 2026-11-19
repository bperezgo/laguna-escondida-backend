package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

const webhookOrgID = "550e8400-e29b-41d4-a716-446655440030"

func webhookRouter(t *testing.T, orgs *mocks.MockOrganizationRepository, config *mocks.MockPaymentLinkConfig) *gin.Engine {
	gin.SetMode(gin.TestMode)
	webhooks := service.NewPaymentWebhookService(orgs, nil, config, nil, nil, nil, nil, slog.New(slog.DiscardHandler))
	router := gin.New()
	router.POST("/api/webhooks/wompi/:organization_id", NewPaymentWebhookHandler(webhooks, slog.New(slog.DiscardHandler)).WompiHandler)
	return router
}

func postWompi(router *gin.Engine, orgID, body string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/api/webhooks/wompi/"+orgID, bytes.NewBufferString(body)))
	return rec
}

func TestPaymentWebhookHandler_Wompi(t *testing.T) {
	sum := sha256.Sum256([]byte("tok_1" + "1530291411" + "s3cret"))
	signedOther := fmt.Sprintf(`{"event":"nequi_token.updated","data":{"nequi_token":{"id":"tok_1"}},`+
		`"signature":{"properties":["nequi_token.id"],"checksum":%q},"timestamp":1530291411}`, hex.EncodeToString(sum[:]))

	t.Run("a path that names no organization is 401", func(t *testing.T) {
		rec := postWompi(webhookRouter(t, mocks.NewMockOrganizationRepository(t), mocks.NewMockPaymentLinkConfig(t)), "laguna", signedOther)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
	t.Run("an unknown organization is 401", func(t *testing.T) {
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, webhookOrgID).Return(nil, domainError.ErrOrganizationNotFound).Once()

		rec := postWompi(webhookRouter(t, orgs, mocks.NewMockPaymentLinkConfig(t)), webhookOrgID, signedOther)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
	t.Run("a forged event is 401", func(t *testing.T) {
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, webhookOrgID).Return(&dto.Organization{ID: webhookOrgID, Slug: "laguna-escondida"}, nil).Once()
		config := mocks.NewMockPaymentLinkConfig(t)
		config.EXPECT().For(mock.Anything, webhookOrgID).Return(dto.WompiCredentials{PrivateKey: "prv_test_x", EventsSecret: "another-secret"}, true, nil).Once()

		rec := postWompi(webhookRouter(t, orgs, config), webhookOrgID, signedOther)

		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})
	t.Run("a signed event of another type is 200", func(t *testing.T) {
		orgs := mocks.NewMockOrganizationRepository(t)
		orgs.EXPECT().Get(mock.Anything, webhookOrgID).Return(&dto.Organization{ID: webhookOrgID, Slug: "laguna-escondida"}, nil).Once()
		config := mocks.NewMockPaymentLinkConfig(t)
		config.EXPECT().For(mock.Anything, webhookOrgID).Return(dto.WompiCredentials{PrivateKey: "prv_test_x", EventsSecret: "s3cret"}, true, nil).Once()

		rec := postWompi(webhookRouter(t, orgs, config), webhookOrgID, signedOther)

		assert.Equal(t, http.StatusOK, rec.Code)
	})
}

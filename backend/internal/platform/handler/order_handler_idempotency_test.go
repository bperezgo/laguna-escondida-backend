package handler

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const (
	idemUserID = "550e8400-e29b-41d4-a716-446655440020"
	idemBody   = `{"open_bill_id":"550e8400-e29b-41d4-a716-446655440010","temporal_identifier":"550e8400-e29b-41d4-a716-446655440011","products":[]}`
)

func idemBodyHash(t *testing.T) string {
	t.Helper()
	var req dto.CreateOrderRequest
	require.NoError(t, json.Unmarshal([]byte(idemBody), &req))
	body, err := json.Marshal(&req)
	require.NoError(t, err)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

// The order service is nil: none of these paths reach it.
func serveIdempotentCreate(t *testing.T, keys *mocks.MockIdempotencyKeyRepository, key string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	h := NewOrderHandler(nil, service.NewOrderIdempotencyService(nil, keys))

	router := gin.New()
	router.POST("/api/orders", func(c *gin.Context) { c.Set("user_id", idemUserID) }, h.CreateOrderHandler)

	req := httptest.NewRequest(http.MethodPost, "/api/orders", bytes.NewBufferString(idemBody))
	req.Header.Set(IdempotencyKeyHeader, key)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestOrderHandler_CreateOrder_IdempotentReplay_Returns201WithHeader(t *testing.T) {
	keys := mocks.NewMockIdempotencyKeyRepository(t)
	keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
	keys.EXPECT().Get(mock.Anything, "order.create", "key-1").Return(&dto.IdempotencyKey{
		UserID: idemUserID, RequestHash: idemBodyHash(t),
		Response: json.RawMessage(`{"id":"550e8400-e29b-41d4-a716-446655440010","total_amount":"0","status":"pending"}`),
	}, nil)

	rec := serveIdempotentCreate(t, keys, "key-1")

	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, "true", rec.Header().Get(IdempotentReplayedHeader))
	var body dto.OpenBill
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "550e8400-e29b-41d4-a716-446655440010", body.ID)
}

func TestOrderHandler_CreateOrder_IdempotencyErrors(t *testing.T) {
	tests := []struct {
		name   string
		key    string
		stored *dto.IdempotencyKey
		status int
		code   string
	}{
		{name: "reused for another body", key: "key-1", stored: &dto.IdempotencyKey{UserID: idemUserID, RequestHash: "other"}, status: http.StatusUnprocessableEntity, code: "IDEMPOTENCY_KEY_REUSED"},
		{name: "still in progress", key: "key-1", stored: &dto.IdempotencyKey{UserID: idemUserID}, status: http.StatusConflict, code: "IDEMPOTENCY_REQUEST_IN_PROGRESS"},
		{name: "key too long", key: strings.Repeat("k", 256), status: http.StatusBadRequest, code: "INVALID_IDEMPOTENCY_KEY"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			keys := mocks.NewMockIdempotencyKeyRepository(t)
			if tt.stored != nil {
				if tt.stored.RequestHash == "" {
					tt.stored.RequestHash = idemBodyHash(t)
					keys.EXPECT().ReleaseStale(mock.Anything, "order.create", tt.key, mock.Anything).Return(false, nil)
				}
				keys.EXPECT().Claim(mock.Anything, mock.Anything).Return(false, nil)
				keys.EXPECT().Get(mock.Anything, "order.create", tt.key).Return(tt.stored, nil)
			}

			rec := serveIdempotentCreate(t, keys, tt.key)

			assert.Equal(t, tt.status, rec.Code)
			var body map[string]string
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tt.code, body["code"])
		})
	}
}

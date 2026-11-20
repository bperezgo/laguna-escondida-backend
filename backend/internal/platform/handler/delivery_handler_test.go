package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	domainError "laguna-escondida/backend/internal/domain/error"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRespondDeliveryError_MapsDomainErrors(t *testing.T) {
	gin.SetMode(gin.TestMode)
	tests := []struct {
		name     string
		err      error
		status   int
		code     string
		expected map[string]any
	}{
		{name: "transition", err: fmt.Errorf("wrapped: %w", &domainError.DeliveryTransitionError{Current: "ready", Action: "deliver"}),
			status: http.StatusConflict, code: "INVALID_DELIVERY_TRANSITION", expected: map[string]any{"current_status": "ready"}},
		{name: "transition without a status", err: &domainError.DeliveryTransitionError{Action: "dispatch"},
			status: http.StatusConflict, code: "INVALID_DELIVERY_TRANSITION", expected: map[string]any{"current_status": nil}},
		{name: "forbidden", err: domainError.ErrDeliveryForbidden, status: http.StatusForbidden, code: "DELIVERY_FORBIDDEN"},
		{name: "validation", err: &domainError.ValidationError{Field: "reason", Reason: "required"},
			status: http.StatusBadRequest, code: "VALIDATION_ERROR", expected: map[string]any{"field": "reason"}},
		{name: "not found", err: domainError.ErrDeliveryOrderNotFound, status: http.StatusNotFound, code: "ORDER_NOT_FOUND"},
		{name: "anything else", err: errors.New("db down"), status: http.StatusInternalServerError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(rec)
			respondDeliveryError(c, "testing", tt.err)

			assert.Equal(t, tt.status, rec.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			if tt.code != "" {
				assert.Equal(t, tt.code, body["code"])
			}
			for k, v := range tt.expected {
				assert.Contains(t, body, k)
				assert.Equal(t, v, body[k], k)
			}
		})
	}
}

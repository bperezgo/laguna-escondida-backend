package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	baseError "laguna-escondida/backend/internal/platform/shared/domain/error"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func decodeErrorBody(t *testing.T, body []byte) map[string]interface{} {
	t.Helper()
	var parsed map[string]interface{}
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("failed to decode response body: %v", err)
	}
	return parsed
}

func TestRespondError_BaseError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	err := baseError.NewBaseError(baseError.ErrorCode("USER_MISSING_NAME"), "name is required")

	RespondError(c, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeErrorBody(t, w.Body.Bytes())
	assert.Equal(t, "name is required", body["error"])
	assert.Equal(t, "USER_MISSING_NAME", body["code"])
}

func TestRespondError_WrappedBaseError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	base := baseError.NewBaseError(baseError.ErrorCode("USER_MISSING_NAME"), "name is required")
	err := fmt.Errorf("failed to create user aggregate: %w", base)

	RespondError(c, err)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	body := decodeErrorBody(t, w.Body.Bytes())
	assert.Equal(t, "name is required", body["error"])
	assert.Equal(t, "USER_MISSING_NAME", body["code"])
}

func TestRespondError_GenericError(t *testing.T) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)

	RespondError(c, errors.New("connection refused"))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	body := decodeErrorBody(t, w.Body.Bytes())
	assert.Equal(t, "Internal server error", body["error"])
	assert.Nil(t, body["code"])
}

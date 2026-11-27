package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestPlansHandler_List_SpecBody(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/api/plans", NewPlansHandler().ListHandler)

	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/plans", nil))

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.JSONEq(t, `{
		"currency": "COP",
		"trial_days": 14,
		"plans": [
			{"id": "starter", "monthly": 25000, "annual": 250000},
			{"id": "einvoicing", "monthly": 40000, "annual": 400000},
			{"id": "full", "monthly": 100000, "annual": 1000000}
		]
	}`, rec.Body.String())
}

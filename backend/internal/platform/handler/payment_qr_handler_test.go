package handler

import (
	"bytes"
	"errors"
	"image/png"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func newQRRouter(t *testing.T) (*gin.Engine, *mocks.MockPaymentQRRepository) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	repo := mocks.NewMockPaymentQRRepository(t)
	qrs := service.NewPaymentQRService(repo, nil, nil, nil, "https://api.example.com", 0, slog.New(slog.DiscardHandler), nil)
	router := gin.New()
	router.GET("/api/payments/qr/:token", NewPaymentQRHandler(qrs).ImageHandler)
	return router, repo
}

func getQR(router *gin.Engine, token string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/payments/qr/"+token, nil))
	return rec
}

func TestPaymentQRHandler_ServesThePNG(t *testing.T) {
	svg, err := os.ReadFile("../qrimage/testdata/wompi_sandbox_qr.svg")
	require.NoError(t, err)
	router, repo := newQRRouter(t)
	repo.EXPECT().FindActiveByToken(mock.Anything, "tok-A").Return(&dto.PaymentQRTransaction{SVG: string(svg)}, true, nil)

	rec := getQR(router, "tok-A")

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "image/png", rec.Header().Get("Content-Type"))
	assert.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	_, err = png.Decode(bytes.NewReader(rec.Body.Bytes()))
	require.NoError(t, err, "a real PNG")
}

func TestPaymentQRHandler_NotFound(t *testing.T) {
	tests := []struct {
		name string
		qr   *dto.PaymentQRTransaction
		ok   bool
	}{
		{"an unknown or retired token", nil, false},
		{"a QR that has not arrived", &dto.PaymentQRTransaction{}, true},
		{"an SVG that is not Wompi's", &dto.PaymentQRTransaction{SVG: "<svg/>"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			router, repo := newQRRouter(t)
			repo.EXPECT().FindActiveByToken(mock.Anything, "tok-A").Return(tt.qr, tt.ok, nil)

			assert.Equal(t, http.StatusNotFound, getQR(router, "tok-A").Code)
		})
	}
}

func TestPaymentQRHandler_ReadFailure(t *testing.T) {
	router, repo := newQRRouter(t)
	repo.EXPECT().FindActiveByToken(mock.Anything, "tok-A").Return(nil, false, errors.New("db down"))

	assert.Equal(t, http.StatusInternalServerError, getQR(router, "tok-A").Code)
}

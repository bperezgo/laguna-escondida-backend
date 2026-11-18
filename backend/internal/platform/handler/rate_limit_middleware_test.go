package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func serveFrom(router *gin.Engine, addr string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/x", nil)
	req.RemoteAddr = addr + ":1234"
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestRateLimiter_SixthRequestInAWindowIsLimited(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(5, time.Hour, func() time.Time { return now })

	for i := 0; i < 5; i++ {
		ok, _ := limiter.allow("1.1.1.1")
		assert.True(t, ok)
	}
	now = now.Add(10 * time.Minute)
	ok, retryAfter := limiter.allow("1.1.1.1")

	assert.False(t, ok)
	assert.Equal(t, 50*time.Minute, retryAfter)
}

func TestRateLimiter_NewWindowResets(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(5, time.Hour, func() time.Time { return now })
	for i := 0; i < 6; i++ {
		limiter.allow("1.1.1.1")
	}

	now = now.Add(time.Hour)
	ok, _ := limiter.allow("1.1.1.1")

	assert.True(t, ok)
}

func TestRateLimiter_AddressesDoNotShareACount(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(5, time.Hour, func() time.Time { return now })
	for i := 0; i < 5; i++ {
		limiter.allow("1.1.1.1")
	}

	ok, _ := limiter.allow("2.2.2.2")

	assert.True(t, ok)
}

func TestRateLimiter_SweepsExpiredKeys(t *testing.T) {
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	limiter := newRateLimiter(5, time.Hour, func() time.Time { return now })
	limiter.allow("1.1.1.1")
	limiter.allow("2.2.2.2")

	now = now.Add(2 * time.Hour)
	limiter.allow("3.3.3.3")

	assert.Len(t, limiter.windows, 1)
}

func TestRateLimitMiddleware_SixthRequestIs429WithRetryAfter(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.POST("/x", RateLimitMiddleware(5, time.Hour), func(c *gin.Context) { c.Status(http.StatusCreated) })

	for i := 0; i < 5; i++ {
		assert.Equal(t, http.StatusCreated, serveFrom(router, "9.9.9.9").Code)
	}
	limited := serveFrom(router, "9.9.9.9")
	other := serveFrom(router, "8.8.8.8")

	assert.Equal(t, http.StatusTooManyRequests, limited.Code)
	assert.Equal(t, "3600", limited.Header().Get("Retry-After"))
	assert.Equal(t, http.StatusCreated, other.Code)
}

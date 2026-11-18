package handler

import (
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

type rateWindow struct {
	start time.Time
	count int
}

// rateLimiter is a fixed window per key, in memory. It is per process: more instances only loosen it.
type rateLimiter struct {
	mu        sync.Mutex
	limit     int
	window    time.Duration
	now       func() time.Time
	windows   map[string]*rateWindow
	lastSweep time.Time
}

func newRateLimiter(limit int, window time.Duration, now func() time.Time) *rateLimiter {
	return &rateLimiter{limit: limit, window: window, now: now, windows: map[string]*rateWindow{}, lastSweep: now()}
}

// allow counts a request for key; when it is over the limit it returns how long until the window ends.
func (l *rateLimiter) allow(key string) (bool, time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	l.sweep(now)

	w, ok := l.windows[key]
	if !ok || !now.Before(w.start.Add(l.window)) {
		l.windows[key] = &rateWindow{start: now, count: 1}
		return true, 0
	}
	if w.count >= l.limit {
		return false, w.start.Add(l.window).Sub(now)
	}
	w.count++
	return true, 0
}

// sweep drops expired windows at most once per window, so the map stays as big as the active addresses.
func (l *rateLimiter) sweep(now time.Time) {
	if now.Sub(l.lastSweep) < l.window {
		return
	}
	for key, w := range l.windows {
		if !now.Before(w.start.Add(l.window)) {
			delete(l.windows, key)
		}
	}
	l.lastSweep = now
}

// RateLimitMiddleware allows limit requests per window for each client address.
func RateLimitMiddleware(limit int, window time.Duration) gin.HandlerFunc {
	limiter := newRateLimiter(limit, window, time.Now)
	return func(c *gin.Context) {
		ok, retryAfter := limiter.allow(c.ClientIP())
		if ok {
			c.Next()
			return
		}
		c.Header("Retry-After", strconv.Itoa(int(math.Ceil(retryAfter.Seconds()))))
		c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "Too many requests"})
	}
}

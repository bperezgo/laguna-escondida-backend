package handler

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"
	"laguna-escondida/backend/internal/platform/sse"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reservationStreamServer(t *testing.T, organizationID string, heartbeat time.Duration) (*sse.ReservationHub, *httptest.Server) {
	t.Helper()
	gin.SetMode(gin.TestMode)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	hub := sse.NewReservationHub(logger)
	stream := NewReservationStreamHandler(hub, logger)
	stream.heartbeat = heartbeat
	router := gin.New()
	router.GET("/api/sse/reservations", func(c *gin.Context) {
		if organizationID != "" {
			c.Request = c.Request.WithContext(tenant.WithOrg(c.Request.Context(), organizationID))
		}
		c.Next()
	}, SSEMiddleware(), stream.StreamHandler)
	server := httptest.NewServer(router)
	t.Cleanup(server.Close)
	return hub, server
}

func openReservationStream(t *testing.T, server *httptest.Server) *http.Response {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/sse/reservations", nil)
	require.NoError(t, err)
	res, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	return res
}

// nextFrame reads one SSE frame (up to the blank line), failing after a second.
func nextFrame(t *testing.T, r *bufio.Reader) string {
	t.Helper()
	frame := make(chan string, 1)
	go func() {
		var b strings.Builder
		for {
			line, err := r.ReadString('\n')
			if err != nil {
				frame <- b.String()
				return
			}
			if line == "\n" {
				frame <- b.String()
				return
			}
			b.WriteString(line)
		}
	}()
	select {
	case f := <-frame:
		return f
	case <-time.After(time.Second):
		t.Fatal("no SSE frame within a second")
		return ""
	}
}

func TestReservationStream_SendsEachChangeOfTheCallersOrganization(t *testing.T) {
	hub, server := reservationStreamServer(t, "org-a", time.Hour)
	res := openReservationStream(t, server)
	defer func() { _ = res.Body.Close() }()
	reader := bufio.NewReader(res.Body)
	require.Equal(t, http.StatusOK, res.StatusCode)
	assert.Equal(t, "text/event-stream", res.Header.Get("Content-Type"))
	assert.Equal(t, ": connected\n", nextFrame(t, reader))

	hub.ReservationChanged(tenant.WithOrg(context.Background(), "org-b"), &dto.Reservation{ID: "elsewhere"})
	hub.ReservationChanged(tenant.WithOrg(context.Background(), "org-a"), &dto.Reservation{
		ID: "r1", Name: "Ana", PartySize: 6, Status: dto.ReservationStatusConfirmed,
	})

	frame := nextFrame(t, reader)
	require.True(t, strings.HasPrefix(frame, "event: reservation\ndata: "), frame)
	var got dto.Reservation
	require.NoError(t, json.Unmarshal([]byte(strings.TrimSpace(strings.TrimPrefix(frame, "event: reservation\ndata: "))), &got))
	assert.Equal(t, "r1", got.ID)
	assert.Equal(t, dto.ReservationStatusConfirmed, got.Status)
}

func TestReservationStream_KeepsAQuietStreamAlive(t *testing.T) {
	_, server := reservationStreamServer(t, "org-a", 20*time.Millisecond)
	res := openReservationStream(t, server)
	defer func() { _ = res.Body.Close() }()
	reader := bufio.NewReader(res.Body)
	nextFrame(t, reader)

	assert.Equal(t, ": heartbeat\n", nextFrame(t, reader))
}

func TestReservationStream_RefusedWithoutAnOrganization(t *testing.T) {
	_, server := reservationStreamServer(t, "", time.Hour)
	res := openReservationStream(t, server)
	defer func() { _ = res.Body.Close() }()

	assert.Equal(t, http.StatusUnauthorized, res.StatusCode)
}

package sse

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/tenant"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func discardHubLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestReservationHub_AChangeReachesOnlyItsOrganizationsScreens(t *testing.T) {
	hub := NewReservationHub(discardHubLogger())
	laguna, lagunaBar, other := hub.Subscribe("org-a"), hub.Subscribe("org-a"), hub.Subscribe("org-b")

	hub.ReservationChanged(tenant.WithOrg(context.Background(), "org-a"), &dto.Reservation{ID: "r1"})

	assert.Equal(t, "r1", (<-laguna.Events).ID)
	assert.Equal(t, "r1", (<-lagunaBar.Events).ID)
	assert.Empty(t, other.Events)
}

func TestReservationHub_AChangeWithoutAnOrganizationGoesNowhere(t *testing.T) {
	hub := NewReservationHub(discardHubLogger())
	client := hub.Subscribe("org-a")

	hub.ReservationChanged(context.Background(), &dto.Reservation{ID: "r1"})

	assert.Empty(t, client.Events)
}

func TestReservationHub_ASlowScreenNeverBlocksTheChange(t *testing.T) {
	hub := NewReservationHub(discardHubLogger())
	client := hub.Subscribe("org-a")
	ctx := tenant.WithOrg(context.Background(), "org-a")

	for range reservationClientBuffer + 5 {
		hub.ReservationChanged(ctx, &dto.Reservation{ID: "r1"})
	}

	assert.Len(t, client.Events, reservationClientBuffer)
}

func TestReservationHub_UnsubscribeAndCloseEndTheStreams(t *testing.T) {
	hub := NewReservationHub(discardHubLogger())
	gone, open := hub.Subscribe("org-a"), hub.Subscribe("org-b")

	hub.Unsubscribe(gone)
	hub.Unsubscribe(gone)
	hub.ReservationChanged(tenant.WithOrg(context.Background(), "org-a"), &dto.Reservation{ID: "r1"})
	_, stillOpen := <-gone.Events
	require.False(t, stillOpen)

	hub.Close()
	_, stillOpen = <-open.Events
	assert.False(t, stillOpen)
}

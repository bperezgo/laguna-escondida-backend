package sse

import (
	"context"
	"log/slog"
	"sync"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/domain/tenant"
)

const reservationClientBuffer = 16

type ReservationClient struct {
	organizationID string
	Events         chan *dto.Reservation
	closeOnce      sync.Once
}

func (c *ReservationClient) close() {
	c.closeOnce.Do(func() { close(c.Events) })
}

// ReservationHub fans reservation changes out to the staff screens of the same organization.
type ReservationHub struct {
	mu      sync.RWMutex
	clients map[string]map[*ReservationClient]struct{}
	logger  *slog.Logger
}

func NewReservationHub(logger *slog.Logger) *ReservationHub {
	return &ReservationHub{clients: make(map[string]map[*ReservationClient]struct{}), logger: logger}
}

func (h *ReservationHub) Subscribe(organizationID string) *ReservationClient {
	client := &ReservationClient{organizationID: organizationID, Events: make(chan *dto.Reservation, reservationClientBuffer)}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.clients[organizationID] == nil {
		h.clients[organizationID] = make(map[*ReservationClient]struct{})
	}
	h.clients[organizationID][client] = struct{}{}
	return client
}

func (h *ReservationHub) Unsubscribe(client *ReservationClient) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if clients, ok := h.clients[client.organizationID]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.organizationID)
		}
	}
	client.close()
}

func (h *ReservationHub) ReservationChanged(ctx context.Context, reservation *dto.Reservation) {
	organizationID, ok := tenant.OrgFrom(ctx)
	if !ok {
		h.logger.WarnContext(ctx, "reservation change without an organization, not streamed", slog.String("reservation_id", reservation.ID))
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients[organizationID] {
		select {
		case client.Events <- reservation:
		default:
			// A screen too slow to keep up misses the change; it reloads the day when it reconnects.
			h.logger.WarnContext(ctx, "reservation stream buffer full, change dropped", slog.String("reservation_id", reservation.ID))
		}
	}
}

func (h *ReservationHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for organizationID, clients := range h.clients {
		for client := range clients {
			client.close()
		}
		delete(h.clients, organizationID)
	}
}

var _ ports.ReservationNotifier = (*ReservationHub)(nil)

package sse

import (
	"context"
	"sync"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"go.uber.org/zap"
)

type OpenBillProductEvent struct {
	Type string                  `json:"type"`
	Data *dto.OpenBillProductSSE `json:"data"`
}

type OpenBillProductClient struct {
	Area      string
	Events    chan OpenBillProductEvent
	closeOnce sync.Once
}

func (c *OpenBillProductClient) Close() {
	c.closeOnce.Do(func() {
		close(c.Events)
	})
}

type OpenBillProductHub struct {
	mu      sync.RWMutex
	clients map[string]map[*OpenBillProductClient]struct{}
	logger  *zap.Logger
}

func NewOpenBillProductHub(logger *zap.Logger) *OpenBillProductHub {
	return &OpenBillProductHub{
		clients: make(map[string]map[*OpenBillProductClient]struct{}),
		logger:  logger,
	}
}

func (h *OpenBillProductHub) Register(client *OpenBillProductClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.Area] == nil {
		h.clients[client.Area] = make(map[*OpenBillProductClient]struct{})
	}
	h.clients[client.Area][client] = struct{}{}

	totalClients := len(h.clients[client.Area])
	h.logger.Info("SSE client registered",
		zap.String("area", client.Area),
		zap.Int("total_clients_in_area", totalClients),
	)
}

func (h *OpenBillProductHub) Unregister(client *OpenBillProductClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.Area]; ok {
		delete(clients, client)
		remainingClients := len(clients)
		h.logger.Info("SSE client unregistered",
			zap.String("area", client.Area),
			zap.Int("remaining_clients_in_area", remainingClients),
		)
		if len(clients) == 0 {
			delete(h.clients, client.Area)
			h.logger.Info("No more SSE clients in area, removing area",
				zap.String("area", client.Area),
			)
		}
	}
	client.Close()
}

func (h *OpenBillProductHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for area, clients := range h.clients {
		for client := range clients {
			client.Close()
		}
		delete(h.clients, area)
	}
}

func (h *OpenBillProductHub) Broadcast(area string, event OpenBillProductEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	clients, ok := h.clients[area]
	if !ok {
		h.logger.Warn("No SSE clients registered for area, event not broadcasted",
			zap.String("area", area),
			zap.String("event_type", event.Type),
		)
		return
	}

	clientCount := len(clients)
	sentCount := 0
	droppedCount := 0

	h.logger.Info("Broadcasting SSE event",
		zap.String("event_type", event.Type),
		zap.String("area", area),
		zap.Int("client_count", clientCount),
	)

	for client := range clients {
		select {
		case client.Events <- event:
			sentCount++
		default:
			droppedCount++
			h.logger.Warn("SSE client buffer full, event dropped",
				zap.String("area", area),
				zap.String("event_type", event.Type),
			)
		}
	}

	h.logger.Info("SSE broadcast complete",
		zap.String("area", area),
		zap.Int("sent", sentCount),
		zap.Int("dropped", droppedCount),
		zap.Int("total_clients", clientCount),
	)
}

func (h *OpenBillProductHub) BroadcastAll(event OpenBillProductEvent) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, clients := range h.clients {
		for client := range clients {
			select {
			case client.Events <- event:
			default:
				// Client buffer full, skip this event
			}
		}
	}
}

func (h *OpenBillProductHub) NotifyArea(ctx context.Context, area string, eventType string, data *dto.OpenBillProductSSE) error {
	h.logger.Debug("NotifyArea called",
		zap.String("area", area),
		zap.String("event_type", eventType),
		zap.String("product_name", data.ProductName),
	)

	event := OpenBillProductEvent{
		Type: eventType,
		Data: data,
	}
	h.Broadcast(area, event)
	return nil
}

func NewOpenBillProductClient(area string) *OpenBillProductClient {
	return &OpenBillProductClient{
		Area:   area,
		Events: make(chan OpenBillProductEvent, 10),
	}
}

var _ ports.OpenBillProductSSENotifier = (*OpenBillProductHub)(nil)

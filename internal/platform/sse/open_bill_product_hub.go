package sse

import (
	"context"
	"sync"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
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
}

func NewOpenBillProductHub() *OpenBillProductHub {
	return &OpenBillProductHub{
		clients: make(map[string]map[*OpenBillProductClient]struct{}),
	}
}

func (h *OpenBillProductHub) Register(client *OpenBillProductClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.Area] == nil {
		h.clients[client.Area] = make(map[*OpenBillProductClient]struct{})
	}
	h.clients[client.Area][client] = struct{}{}
}

func (h *OpenBillProductHub) Unregister(client *OpenBillProductClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.Area]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.Area)
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

	if clients, ok := h.clients[area]; ok {
		for client := range clients {
			select {
			case client.Events <- event:
			default:
				// Client buffer full, skip this event
			}
		}
	}
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

package sse

import (
	"context"
	"encoding/json"
	"sync"

	"laguna-escondida/backend/internal/domain/ports"
)

type Event struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

type Client struct {
	Area   string
	Events chan Event
}

type Hub struct {
	mu      sync.RWMutex
	clients map[string]map[*Client]struct{}
}

func NewHub() *Hub {
	return &Hub{
		clients: make(map[string]map[*Client]struct{}),
	}
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.Area] == nil {
		h.clients[client.Area] = make(map[*Client]struct{})
	}
	h.clients[client.Area][client] = struct{}{}
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if clients, ok := h.clients[client.Area]; ok {
		delete(clients, client)
		if len(clients) == 0 {
			delete(h.clients, client.Area)
		}
	}
	close(client.Events)
}

func (h *Hub) Broadcast(area string, event Event) {
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

func (h *Hub) NotifyArea(ctx context.Context, area string, eventType string, data interface{}) error {
	event := Event{
		Type: eventType,
		Data: data,
	}
	h.Broadcast(area, event)
	return nil
}

func (h *Hub) GetClientCount(area string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if clients, ok := h.clients[area]; ok {
		return len(clients)
	}
	return 0
}

func NewClient(area string) *Client {
	return &Client{
		Area:   area,
		Events: make(chan Event, 10),
	}
}

func (e Event) ToSSEFormat() ([]byte, error) {
	data, err := json.Marshal(e.Data)
	if err != nil {
		return nil, err
	}
	return data, nil
}

var _ ports.SSENotifier = (*Hub)(nil)

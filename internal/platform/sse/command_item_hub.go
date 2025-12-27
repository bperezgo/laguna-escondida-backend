package sse

import (
	"context"
	"sync"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

type CommandItemEvent struct {
	Type string              `json:"type"`
	Data *dto.CommandItemSSE `json:"data"`
}

type CommandItemClient struct {
	Area      string
	Events    chan CommandItemEvent
	closeOnce sync.Once
}

func (c *CommandItemClient) Close() {
	c.closeOnce.Do(func() {
		close(c.Events)
	})
}

type CommandItemHub struct {
	mu      sync.RWMutex
	clients map[string]map[*CommandItemClient]struct{}
}

func NewCommandItemHub() *CommandItemHub {
	return &CommandItemHub{
		clients: make(map[string]map[*CommandItemClient]struct{}),
	}
}

func (h *CommandItemHub) Register(client *CommandItemClient) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.clients[client.Area] == nil {
		h.clients[client.Area] = make(map[*CommandItemClient]struct{})
	}
	h.clients[client.Area][client] = struct{}{}
}

func (h *CommandItemHub) Unregister(client *CommandItemClient) {
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

func (h *CommandItemHub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()

	for area, clients := range h.clients {
		for client := range clients {
			client.Close()
		}
		delete(h.clients, area)
	}
}

func (h *CommandItemHub) Broadcast(area string, event CommandItemEvent) {
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

func (h *CommandItemHub) NotifyArea(ctx context.Context, area string, eventType string, data *dto.CommandItemSSE) error {
	event := CommandItemEvent{
		Type: eventType,
		Data: data,
	}
	h.Broadcast(area, event)
	return nil
}

func NewCommandItemClient(area string) *CommandItemClient {
	return &CommandItemClient{
		Area:   area,
		Events: make(chan CommandItemEvent, 10),
	}
}

var _ ports.CommandItemSSENotifier = (*CommandItemHub)(nil)

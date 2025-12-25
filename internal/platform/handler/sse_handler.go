package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/sse"

	"github.com/gin-gonic/gin"
)

type SSEHandler struct {
	hub            *sse.Hub
	commandService *service.CommandService
}

func NewSSEHandler(hub *sse.Hub, commandService *service.CommandService) *SSEHandler {
	return &SSEHandler{
		hub:            hub,
		commandService: commandService,
	}
}

func (h *SSEHandler) StreamCommandsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Access-Control-Allow-Origin", "*")
	c.Header("X-Accel-Buffering", "no")

	client := sse.NewClient(area)
	h.hub.Register(client)
	defer h.hub.Unregister(client)

	ctx := c.Request.Context()

	pendingCommands, err := h.commandService.GetPendingCommandsByArea(ctx, area)
	if err == nil && len(pendingCommands) > 0 {
		for _, cmd := range pendingCommands {
			data, err := json.Marshal(cmd)
			if err != nil {
				continue
			}
			fmt.Fprintf(c.Writer, "event: command.pending\ndata: %s\n\n", data)
			c.Writer.Flush()
		}
	}

	c.Stream(func(w io.Writer) bool {
		select {
		case event, ok := <-client.Events:
			if !ok {
				return false
			}
			data, err := event.ToSSEFormat()
			if err != nil {
				return true
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Type, data)
			return true
		case <-ctx.Done():
			return false
		}
	})
}

func (h *SSEHandler) GetPendingCommandsHandler(c *gin.Context) {
	area := c.Param("area")
	if area == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Area is required"})
		return
	}

	ctx := c.Request.Context()
	commands, err := h.commandService.GetPendingCommandsByArea(ctx, area)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get pending commands"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"commands": commands})
}

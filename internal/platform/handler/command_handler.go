package handler

import (
	"errors"
	"net/http"

	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type CommandHandler struct {
	commandService *service.CommandService
}

func NewCommandHandler(commandService *service.CommandService) *CommandHandler {
	return &CommandHandler{
		commandService: commandService,
	}
}

func (h *CommandHandler) CompleteCommandHandler(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Command ID is required"})
		return
	}

	command, err := h.commandService.CompleteCommand(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, domainError.ErrCommandNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "Command not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to complete command"})
		return
	}

	c.JSON(http.StatusOK, command)
}

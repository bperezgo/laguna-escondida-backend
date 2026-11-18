package handler

import (
	"github.com/gin-gonic/gin"
)

type HealthCheckResponse struct {
	Status string `json:"status"`
}

func HealthCheckHandler(c *gin.Context) {
	c.JSON(200, HealthCheckResponse{
		Status: "ok",
	})
}

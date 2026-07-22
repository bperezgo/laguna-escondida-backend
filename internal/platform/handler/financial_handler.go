package handler

import (
	"log"
	"net/http"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"

	"github.com/gin-gonic/gin"
)

type FinancialHandler struct {
	financialService *service.FinancialService
}

func NewFinancialHandler(financialService *service.FinancialService) *FinancialHandler {
	return &FinancialHandler{
		financialService: financialService,
	}
}

func (h *FinancialHandler) GetFinancialSummaryHandler(c *gin.Context) {
	startDateStr := c.Query("start_date")
	endDateStr := c.Query("end_date")

	if startDateStr == "" || endDateStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "start_date and end_date are required"})
		return
	}

	startDate, err := time.Parse(time.RFC3339, startDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid start_date format. Use RFC3339 (e.g., 2024-01-01T00:00:00Z)"})
		return
	}

	endDate, err := time.Parse(time.RFC3339, endDateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid end_date format. Use RFC3339 (e.g., 2024-12-31T23:59:59Z)"})
		return
	}

	req := &dto.FinancialSummaryRequest{
		StartDate: startDate,
		EndDate:   endDate,
	}

	summary, err := h.financialService.GetFinancialSummary(c.Request.Context(), req)
	if err != nil {
		log.Printf("Error getting financial summary: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get financial summary"})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetDailyCloseHandler returns the read-only end-of-day money reconciliation for one
// business day (America/Bogota). ?date=YYYY-MM-DD selects the day; it defaults to today.
func (h *FinancialHandler) GetDailyCloseHandler(c *gin.Context) {
	dateStr := c.Query("date")
	if dateStr == "" {
		dateStr = time.Now().In(utcMinus5).Format(simpleDateLayout)
	}

	from, err := parseStartDate(dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}
	to, err := parseEndDate(dateStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid date format. Use YYYY-MM-DD"})
		return
	}

	report, err := h.financialService.GetDailyClose(c.Request.Context(), from, to)
	if err != nil {
		log.Printf("Error getting daily close: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get daily close"})
		return
	}

	c.JSON(http.StatusOK, report)
}

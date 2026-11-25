package handler

import (
	"errors"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/service"
	"laguna-escondida/backend/internal/platform/qrimage"

	"github.com/gin-gonic/gin"
)

// PaymentQRHandler serves an order's QR as a PNG at its public, unguessable URL: WhatsApp fetches it
// without credentials. No image is stored; it is drawn from the SVG on each request.
type PaymentQRHandler struct {
	qrs *service.PaymentQRService
}

func NewPaymentQRHandler(qrs *service.PaymentQRService) *PaymentQRHandler {
	return &PaymentQRHandler{qrs: qrs}
}

func (h *PaymentQRHandler) ImageHandler(c *gin.Context) {
	svg, found, err := h.qrs.ServeSVG(c.Request.Context(), c.Param("token"))
	if err != nil {
		log.Printf("Error reading a payment QR: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read the QR"})
		return
	}
	if !found {
		c.JSON(http.StatusNotFound, gin.H{"error": "QR not found"})
		return
	}
	image, err := qrimage.RenderPNG(svg)
	if err != nil {
		if errors.Is(err, qrimage.ErrUnreadableQR) {
			log.Printf("Payment QR not drawn: %v", err)
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "QR not found"})
		return
	}
	// A retired QR must stop answering at once, so nothing caches the image.
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/png", image)
}

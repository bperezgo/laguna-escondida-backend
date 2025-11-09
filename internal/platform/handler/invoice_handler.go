package handler

import (
	"encoding/json"
	"log"
	"net/http"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/service"
)

type InvoiceHandler struct {
	invoiceService *service.InvoiceService
}

func NewInvoiceHandler(invoiceService *service.InvoiceService) *InvoiceHandler {
	return &InvoiceHandler{
		invoiceService: invoiceService,
	}
}

func (h *InvoiceHandler) CreateElectronicInvoiceHandler(w http.ResponseWriter, r *http.Request) {
	var invoice dto.ElectronicInvoice
	if err := json.NewDecoder(r.Body).Decode(&invoice); err != nil {
		log.Printf("Error decoding request: %v", err)
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.invoiceService.CreateElectronicInvoice(r.Context(), &invoice); err != nil {
		log.Printf("Error creating electronic invoice: %v", err)
		http.Error(w, "Failed to create electronic invoice", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

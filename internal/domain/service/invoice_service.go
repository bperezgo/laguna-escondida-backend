package service

import (
	"context"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
)

type InvoiceService struct {
	electronicInvoiceClient ports.ElectronicInvoiceClient
}

func NewInvoiceService(electronicInvoiceClient ports.ElectronicInvoiceClient) *InvoiceService {
	return &InvoiceService{
		electronicInvoiceClient: electronicInvoiceClient,
	}
}

func (s *InvoiceService) CreateElectronicInvoice(ctx context.Context, bill *dto.Bill) error {
	return s.electronicInvoiceClient.Create(ctx, bill)
}

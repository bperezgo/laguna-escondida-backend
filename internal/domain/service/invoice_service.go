package service

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/samber/lo"
)

type InvoiceService struct {
	electronicInvoiceClient ports.ElectronicInvoiceClient
	productRepo             ports.ProductRepository
	billRepo                ports.BillRepository
}

func NewInvoiceService(
	electronicInvoiceClient ports.ElectronicInvoiceClient,
	productRepo ports.ProductRepository,
	billRepo ports.BillRepository,
) *InvoiceService {
	return &InvoiceService{
		electronicInvoiceClient: electronicInvoiceClient,
		productRepo:             productRepo,
		billRepo:                billRepo,
	}
}

func (s *InvoiceService) CreateElectronicInvoice(ctx context.Context, invoice *dto.ElectronicInvoice) error {
	products, err := s.productRepo.FindByIDs(ctx, lo.Map(invoice.Items, func(item dto.InvoiceItem, _ int) string {
		return item.ProductID
	}))

	if err != nil {
		return err
	}

	bill, err := bill.NewBillFromCreateElectronicInvoiceRequest(invoice, lo.Map(invoice.Items, func(item dto.InvoiceItem, idx int) *bill.BillProduct {
		product := products[idx]

		return bill.NewBillProduct(
			item.ProductID,
			item.Quantity,
			product.UnitPrice,
			product.Description,
			product.Brand,
			product.Model,
			product.SKU,
			item.Allowance,
			product.VAT,
			product.ICO,
		)
	}))

	if err != nil {
		return err
	}

	return s.billRepo.Create(ctx, bill, products)
}

package service

import (
	"context"
	"laguna-escondida/backend/internal/domain/aggregate/bill"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
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

	if len(products) != len(invoice.Items) {
		return domainError.ErrProductNotFound
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

func (s *InvoiceService) ListInvoices(ctx context.Context, req *dto.ListInvoicesRequest) (*dto.ListInvoicesResponse, error) {
	criteria := dto.NewBillCriteria().
		WithPage(req.Page).
		WithPageSize(req.PageSize).
		WithCreatedAtRange(req.CreatedAtStart, req.CreatedAtEnd).
		WithNationalIdentification(req.NationalIdentification)

	if criteria.Page == 0 {
		criteria.Page = 1
	}
	if criteria.PageSize == 0 {
		criteria.PageSize = 20
	}

	invoices, totalCount, err := s.billRepo.FindByCriteria(ctx, criteria)
	if err != nil {
		return nil, err
	}

	totalPages := int(totalCount) / criteria.PageSize
	if int(totalCount)%criteria.PageSize != 0 {
		totalPages++
	}

	return &dto.ListInvoicesResponse{
		Invoices:   invoices,
		TotalCount: totalCount,
		Page:       criteria.Page,
		PageSize:   criteria.PageSize,
		TotalPages: totalPages,
	}, nil
}

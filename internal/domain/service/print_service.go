package service

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/shopspring/decimal"
)

// PrintService orchestrates printing a receipt ("cuenta") for an open bill. It
// loads the authoritative bill, maps it to a transport-agnostic dto.Ticket, and
// hands it to the ReceiptPrinter. It holds no ESC/POS or byte knowledge.
type PrintService struct {
	openBillRepo ports.OpenBillRepository
	printer      ports.ReceiptPrinter
	business     dto.TicketBusinessInfo
}

func NewPrintService(
	openBillRepo ports.OpenBillRepository,
	printer ports.ReceiptPrinter,
	business dto.TicketBusinessInfo,
) *PrintService {
	return &PrintService{
		openBillRepo: openBillRepo,
		printer:      printer,
		business:     business,
	}
}

// PrintTicket loads the open bill, builds the ticket and prints it Copies times
// (default 1). A missing bill is reported as ErrOpenBillNotFound and a printer
// failure as ErrTicketPrintFailed, both wrapping the underlying error.
func (s *PrintService) PrintTicket(ctx context.Context, req *dto.PrintTicketRequest) error {
	copies := req.Copies
	if copies < 1 {
		copies = 1
	}

	bill, err := s.openBillRepo.FindByIDWithProducts(ctx, req.OpenBillID)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrOpenBillNotFound, err)
	}

	ticket := s.buildTicket(bill)

	for i := 0; i < copies; i++ {
		if err := s.printer.Print(ctx, ticket); err != nil {
			return fmt.Errorf("%w: %w", domainError.ErrTicketPrintFailed, err)
		}
	}

	return nil
}

// buildTicket maps an authoritative open bill to the structured ticket. Money
// columns are computed from the products so the printed totals are the backend's,
// not the client's: the line uses the tax-inclusive unit price, and the
// subtotal/VAT/ICO breakdown is derived from each product's tax amounts.
func (s *PrintService) buildTicket(bill *dto.OpenBillWithProducts) *dto.Ticket {
	items := make([]dto.TicketItem, 0, len(bill.Products))
	subtotal := decimal.Zero
	vat := decimal.Zero
	ico := decimal.Zero

	for _, detail := range bill.Products {
		quantity := decimal.NewFromInt(int64(detail.Quantity))
		product := detail.Product

		subtotal = subtotal.Add(product.UnitPrice.Mul(quantity))
		vat = vat.Add(product.VATAmount.Mul(quantity))
		ico = ico.Add(product.ICOAmount.Mul(quantity))

		notes := ""
		if detail.Notes != nil {
			notes = *detail.Notes
		}

		items = append(items, dto.TicketItem{
			Name:      product.Name,
			Quantity:  detail.Quantity,
			UnitPrice: product.TotalPriceWithTaxes,
			LineTotal: product.TotalPriceWithTaxes.Mul(quantity),
			Notes:     notes,
		})
	}

	descriptor := ""
	if bill.Descriptor != nil {
		descriptor = *bill.Descriptor
	}

	return &dto.Ticket{
		BusinessName:       s.business.Name,
		BusinessNIT:        s.business.NIT,
		BusinessAddress:    s.business.Address,
		TemporalIdentifier: bill.TemporalIdentifier,
		ServerName:         bill.CreatedBy.Name,
		Descriptor:         descriptor,
		IssuedAt:           bill.CreatedAt,
		Items:              items,
		Subtotal:           subtotal,
		VAT:                vat,
		ICO:                ico,
		Tip:                decimal.Zero,
		Total:              bill.TotalAmount,
		Footer:             s.business.Footer,
		LegalNotice:        s.business.LegalNotice,
	}
}

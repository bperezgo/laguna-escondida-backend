package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const printOpenBillID = "550e8400-e29b-41d4-a716-446655440099"

// Test helpers (print-prefixed to avoid clashing with order_service_test.go).

func printTestBusinessInfo() dto.TicketBusinessInfo {
	return dto.TicketBusinessInfo{
		Name:        "Laguna Escondida",
		NIT:         "900.123.456-7",
		Address:     "Vereda La Laguna",
		Footer:      "¡Gracias por su visita!",
		LegalNotice: "Documento equivalente - no es factura",
	}
}

func printTestProductDetail(name string, quantity int, unitPrice, vatAmount, icoAmount float64, notes *string) dto.OpenBillProductDetail {
	unit := decimal.NewFromFloat(unitPrice)
	vat := decimal.NewFromFloat(vatAmount)
	ico := decimal.NewFromFloat(icoAmount)
	return dto.OpenBillProductDetail{
		OpenBillProductID: "obp-" + name,
		Quantity:          quantity,
		Notes:             notes,
		Product: dto.Product{
			Name:                name,
			UnitPrice:           unit,
			VATAmount:           vat,
			ICOAmount:           ico,
			TotalPriceWithTaxes: unit.Add(vat).Add(ico),
		},
	}
}

func printTestOpenBill(products []dto.OpenBillProductDetail) *dto.OpenBillWithProducts {
	total := decimal.Zero
	for _, p := range products {
		total = total.Add(p.Product.TotalPriceWithTaxes.Mul(decimal.NewFromInt(int64(p.Quantity))))
	}
	descriptor := "MESA 5"
	return &dto.OpenBillWithProducts{
		ID:                 printOpenBillID,
		TemporalIdentifier: "TABLE-05",
		TotalAmount:        total,
		CreatedBy:          dto.OpenBillCreator{Name: "Ana"},
		Descriptor:         &descriptor,
		Products:           products,
		CreatedAt:          time.Date(2026, 6, 22, 12, 0, 0, 0, time.UTC),
	}
}

func newPrintTestService(repo *mocks.MockOpenBillRepository, printer *mocks.MockReceiptPrinter) *PrintService {
	return NewPrintService(repo, printer, printTestBusinessInfo())
}

// Success Cases

func TestPrintTicket_Success(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockOpenBillRepository(t)
	printer := mocks.NewMockReceiptPrinter(t)

	bill := printTestOpenBill([]dto.OpenBillProductDetail{
		printTestProductDetail("Cerveza", 2, 8000, 1520, 0, nil),
	})
	repo.EXPECT().FindByIDWithProducts(mock.Anything, printOpenBillID).Return(bill, nil)
	printer.EXPECT().Print(mock.Anything, mock.AnythingOfType("*dto.Ticket")).Return(nil).Once()

	svc := newPrintTestService(repo, printer)
	err := svc.PrintTicket(ctx, &dto.PrintTicketRequest{OpenBillID: printOpenBillID, Copies: 0})

	require.NoError(t, err)
}

func TestPrintTicket_MultipleCopies(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockOpenBillRepository(t)
	printer := mocks.NewMockReceiptPrinter(t)

	bill := printTestOpenBill([]dto.OpenBillProductDetail{
		printTestProductDetail("Cerveza", 1, 8000, 1520, 0, nil),
	})
	repo.EXPECT().FindByIDWithProducts(mock.Anything, printOpenBillID).Return(bill, nil)
	printer.EXPECT().Print(mock.Anything, mock.Anything).Return(nil).Times(3)

	svc := newPrintTestService(repo, printer)
	err := svc.PrintTicket(ctx, &dto.PrintTicketRequest{OpenBillID: printOpenBillID, Copies: 3})

	require.NoError(t, err)
}

func TestPrintTicket_CopiesCappedAtMax(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockOpenBillRepository(t)
	printer := mocks.NewMockReceiptPrinter(t)

	bill := printTestOpenBill([]dto.OpenBillProductDetail{
		printTestProductDetail("Cerveza", 1, 8000, 1520, 0, nil),
	})
	repo.EXPECT().FindByIDWithProducts(mock.Anything, printOpenBillID).Return(bill, nil)
	printer.EXPECT().Print(mock.Anything, mock.Anything).Return(nil).Times(maxTicketCopies)

	svc := newPrintTestService(repo, printer)
	err := svc.PrintTicket(ctx, &dto.PrintTicketRequest{OpenBillID: printOpenBillID, Copies: 999})

	require.NoError(t, err)
}

// Error Cases

func TestPrintTicket_BillNotFound(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockOpenBillRepository(t)
	printer := mocks.NewMockReceiptPrinter(t)

	repo.EXPECT().FindByIDWithProducts(mock.Anything, printOpenBillID).
		Return(nil, errors.New("sql: no rows in result set"))

	svc := newPrintTestService(repo, printer)
	err := svc.PrintTicket(ctx, &dto.PrintTicketRequest{OpenBillID: printOpenBillID})

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrOpenBillNotFound)
	// printer.Print is never expected; the mock asserts it was not called.
}

func TestPrintTicket_PrinterError(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockOpenBillRepository(t)
	printer := mocks.NewMockReceiptPrinter(t)

	bill := printTestOpenBill([]dto.OpenBillProductDetail{
		printTestProductDetail("Cerveza", 1, 8000, 1520, 0, nil),
	})
	repo.EXPECT().FindByIDWithProducts(mock.Anything, printOpenBillID).Return(bill, nil)
	printer.EXPECT().Print(mock.Anything, mock.Anything).Return(errors.New("out of paper")).Once()

	svc := newPrintTestService(repo, printer)
	err := svc.PrintTicket(ctx, &dto.PrintTicketRequest{OpenBillID: printOpenBillID})

	require.Error(t, err)
	assert.ErrorIs(t, err, domainError.ErrTicketPrintFailed)
}

// Mapping / Calculation Validation

func TestPrintTicket_TicketMapping(t *testing.T) {
	ctx := context.Background()
	repo := mocks.NewMockOpenBillRepository(t)
	printer := mocks.NewMockReceiptPrinter(t)

	notes := "sin hielo"
	bill := printTestOpenBill([]dto.OpenBillProductDetail{
		printTestProductDetail("Cerveza", 2, 8000, 1520, 0, &notes), // unit total 9520
		printTestProductDetail("Limonada", 1, 5000, 950, 100, nil),  // unit total 6050
	})
	repo.EXPECT().FindByIDWithProducts(mock.Anything, printOpenBillID).Return(bill, nil)

	var captured *dto.Ticket
	printer.EXPECT().Print(mock.Anything, mock.Anything).
		Run(func(_ context.Context, ticket *dto.Ticket) { captured = ticket }).
		Return(nil)

	svc := newPrintTestService(repo, printer)
	err := svc.PrintTicket(ctx, &dto.PrintTicketRequest{OpenBillID: printOpenBillID})
	require.NoError(t, err)

	require.NotNil(t, captured)

	// Header: business info + bill fields.
	assert.Equal(t, "Laguna Escondida", captured.BusinessName)
	assert.Equal(t, "900.123.456-7", captured.BusinessNIT)
	assert.Equal(t, "Vereda La Laguna", captured.BusinessAddress)
	assert.Equal(t, "TABLE-05", captured.TemporalIdentifier)
	assert.Equal(t, "Ana", captured.ServerName)
	assert.Equal(t, "MESA 5", captured.Descriptor)
	assert.Equal(t, "¡Gracias por su visita!", captured.Footer)
	assert.Equal(t, "Documento equivalente - no es factura", captured.LegalNotice)

	// Items.
	require.Len(t, captured.Items, 2)
	assert.Equal(t, "Cerveza", captured.Items[0].Name)
	assert.Equal(t, 2, captured.Items[0].Quantity)
	assert.Equal(t, "sin hielo", captured.Items[0].Notes)
	assert.True(t, captured.Items[0].UnitPrice.Equal(decimal.NewFromInt(9520)), "unit price is tax-inclusive")
	assert.True(t, captured.Items[0].LineTotal.Equal(decimal.NewFromInt(19040)), "line total = 2 * 9520")
	assert.Equal(t, "", captured.Items[1].Notes)

	// Totals: subtotal = 2*8000 + 5000 = 21000; vat = 2*1520 + 950 = 3990; ico = 100; total = 25090.
	assert.True(t, captured.Subtotal.Equal(decimal.NewFromInt(21000)), "subtotal")
	assert.True(t, captured.VAT.Equal(decimal.NewFromInt(3990)), "vat")
	assert.True(t, captured.ICO.Equal(decimal.NewFromInt(100)), "ico")
	assert.True(t, captured.Tip.Equal(decimal.Zero), "tip is zero for an open bill")
	assert.True(t, captured.Total.Equal(decimal.NewFromInt(25090)), "total equals the bill total")
	// Breakdown reconciles with the total.
	assert.True(t, captured.Subtotal.Add(captured.VAT).Add(captured.ICO).Equal(captured.Total))
}

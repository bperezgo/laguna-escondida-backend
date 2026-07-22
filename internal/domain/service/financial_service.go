package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/shopspring/decimal"
)

type FinancialService struct {
	billRepo          ports.BillRepository
	expenseRepo       ports.ExpenseRepository
	purchaseEntryRepo ports.PurchaseEntryRepository
}

func NewFinancialService(
	billRepo ports.BillRepository,
	expenseRepo ports.ExpenseRepository,
	purchaseEntryRepo ports.PurchaseEntryRepository,
) *FinancialService {
	return &FinancialService{
		billRepo:          billRepo,
		expenseRepo:       expenseRepo,
		purchaseEntryRepo: purchaseEntryRepo,
	}
}

func (s *FinancialService) GetFinancialSummary(ctx context.Context, req *dto.FinancialSummaryRequest) (*dto.FinancialSummary, error) {
	revenue, err := s.billRepo.GetRevenueSummary(ctx, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue summary: %w", err)
	}

	expenses, err := s.expenseRepo.GetExpenseSummary(ctx, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get expense summary: %w", err)
	}

	purchases, err := s.purchaseEntryRepo.GetPurchaseSummary(ctx, req.StartDate, req.EndDate)
	if err != nil {
		return nil, fmt.Errorf("failed to get purchase summary: %w", err)
	}

	netIncome := revenue.TotalAmount.Sub(expenses.TotalAmount).Sub(purchases.TotalAmount)

	return &dto.FinancialSummary{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		Revenue:   *revenue,
		Expenses:  *expenses,
		Purchases: *purchases,
		NetIncome: netIncome,
	}, nil
}

// GetDailyClose builds the read-only end-of-day money reconciliation for the [from, to]
// business-day range. Totals reuse GetRevenueSummary; the per-method split reuses
// GetSalesByPaymentMethod. TotalCollected is the sum of the gross per method, so the
// breakdown always reconciles to the total.
func (s *FinancialService) GetDailyClose(ctx context.Context, from, to time.Time) (*dto.DailyCloseReport, error) {
	revenue, err := s.billRepo.GetRevenueSummary(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get revenue summary: %w", err)
	}

	breakdown, err := s.billRepo.GetSalesByPaymentMethod(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("failed to get sales by payment method: %w", err)
	}

	totalCollected := decimal.Zero
	for _, b := range breakdown {
		totalCollected = totalCollected.Add(b.Collected)
	}

	return &dto.DailyCloseReport{
		StartDate:       from,
		EndDate:         to,
		TotalOrders:     revenue.Count,
		TotalCollected:  totalCollected,
		TotalNet:        revenue.TotalAmount,
		TotalVAT:        revenue.TotalVAT,
		TotalICO:        revenue.TotalICO,
		TotalDiscount:   revenue.TotalDiscount,
		TotalTip:        revenue.TotalTip,
		ByPaymentMethod: breakdown,
	}, nil
}

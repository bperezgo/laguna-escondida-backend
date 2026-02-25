package service

import (
	"context"
	"fmt"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
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

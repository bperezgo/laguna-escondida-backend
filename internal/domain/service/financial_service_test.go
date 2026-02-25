package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports/mocks"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func createTestFinancialService(t *testing.T) (*FinancialService, *mocks.MockBillRepository, *mocks.MockExpenseRepository, *mocks.MockPurchaseEntryRepository) {
	b := mocks.NewMockBillRepository(t)
	e := mocks.NewMockExpenseRepository(t)
	p := mocks.NewMockPurchaseEntryRepository(t)
	return NewFinancialService(b, e, p), b, e, p
}

func createTestFinancialRequest() *dto.FinancialSummaryRequest {
	return &dto.FinancialSummaryRequest{
		StartDate: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		EndDate:   time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC),
	}
}

func TestGetFinancialSummary_Success(t *testing.T) {
	ctx := context.Background()
	svc, br, er, pr := createTestFinancialService(t)
	req := createTestFinancialRequest()

	rev := &dto.RevenueSummary{
		TotalAmount:   decimal.NewFromFloat(5000000),
		TotalVAT:      decimal.NewFromFloat(950000),
		TotalICO:      decimal.NewFromFloat(400000),
		TotalDiscount: decimal.NewFromFloat(100000),
		TotalTip:      decimal.NewFromFloat(50000),
		Count:         150,
	}
	exp := &dto.ExpenseSummary{
		TotalAmount: decimal.NewFromFloat(1500000),
		ByCategory: []dto.ExpenseCategorySummary{
			{CategoryID: "c1", CategoryName: "Servicios", CategoryCode: "SVC", TotalAmount: decimal.NewFromFloat(800000), Count: 12},
			{CategoryID: "c2", CategoryName: "Arriendo", CategoryCode: "RNT", TotalAmount: decimal.NewFromFloat(700000), Count: 12},
		},
		Count: 24,
	}
	pur := &dto.PurchaseSummary{TotalAmount: decimal.NewFromFloat(2000000), Count: 50}

	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(rev, nil)
	er.On("GetExpenseSummary", ctx, req.StartDate, req.EndDate).Return(exp, nil)
	pr.On("GetPurchaseSummary", ctx, req.StartDate, req.EndDate).Return(pur, nil)

	result, err := svc.GetFinancialSummary(ctx, req)

	require.NoError(t, err)
	assert.Equal(t, req.StartDate, result.StartDate)
	assert.Equal(t, req.EndDate, result.EndDate)
	assert.True(t, result.Revenue.TotalAmount.Equal(decimal.NewFromFloat(5000000)))
	assert.Equal(t, 150, result.Revenue.Count)
	assert.True(t, result.Expenses.TotalAmount.Equal(decimal.NewFromFloat(1500000)))
	assert.Equal(t, 24, result.Expenses.Count)
	assert.Equal(t, 2, len(result.Expenses.ByCategory))
	assert.True(t, result.Purchases.TotalAmount.Equal(decimal.NewFromFloat(2000000)))
	assert.Equal(t, 50, result.Purchases.Count)
	// NetIncome = 5000000 - 1500000 - 2000000 = 1500000
	assert.True(t, result.NetIncome.Equal(decimal.NewFromFloat(1500000)))
}

func TestGetFinancialSummary_ZeroValues(t *testing.T) {
	ctx := context.Background()
	svc, br, er, pr := createTestFinancialService(t)
	req := createTestFinancialRequest()

	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(&dto.RevenueSummary{}, nil)
	er.On("GetExpenseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.ExpenseSummary{ByCategory: []dto.ExpenseCategorySummary{}}, nil)
	pr.On("GetPurchaseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.PurchaseSummary{}, nil)

	result, err := svc.GetFinancialSummary(ctx, req)

	require.NoError(t, err)
	assert.True(t, result.NetIncome.Equal(decimal.Zero))
	assert.Equal(t, 0, result.Revenue.Count)
	assert.Equal(t, 0, result.Expenses.Count)
	assert.Equal(t, 0, result.Purchases.Count)
}

func TestGetFinancialSummary_NegativeNetIncome(t *testing.T) {
	ctx := context.Background()
	svc, br, er, pr := createTestFinancialService(t)
	req := createTestFinancialRequest()

	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(&dto.RevenueSummary{TotalAmount: decimal.NewFromFloat(1000000), Count: 10}, nil)
	er.On("GetExpenseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.ExpenseSummary{TotalAmount: decimal.NewFromFloat(2000000), ByCategory: []dto.ExpenseCategorySummary{}, Count: 20}, nil)
	pr.On("GetPurchaseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.PurchaseSummary{TotalAmount: decimal.NewFromFloat(500000), Count: 5}, nil)

	result, err := svc.GetFinancialSummary(ctx, req)

	require.NoError(t, err)
	// NetIncome = 1000000 - 2000000 - 500000 = -1500000
	assert.True(t, result.NetIncome.Equal(decimal.NewFromFloat(-1500000)))
}

func TestGetFinancialSummary_RevenueError(t *testing.T) {
	ctx := context.Background()
	svc, br, _, _ := createTestFinancialService(t)
	req := createTestFinancialRequest()

	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(nil, errors.New("db error"))

	result, err := svc.GetFinancialSummary(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get revenue summary")
}

func TestGetFinancialSummary_ExpenseError(t *testing.T) {
	ctx := context.Background()
	svc, br, er, _ := createTestFinancialService(t)
	req := createTestFinancialRequest()

	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(&dto.RevenueSummary{TotalAmount: decimal.NewFromFloat(1000000), Count: 10}, nil)
	er.On("GetExpenseSummary", ctx, req.StartDate, req.EndDate).Return(nil, errors.New("db error"))

	result, err := svc.GetFinancialSummary(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get expense summary")
}

func TestGetFinancialSummary_PurchaseError(t *testing.T) {
	ctx := context.Background()
	svc, br, er, pr := createTestFinancialService(t)
	req := createTestFinancialRequest()

	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(&dto.RevenueSummary{TotalAmount: decimal.NewFromFloat(1000000), Count: 10}, nil)
	er.On("GetExpenseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.ExpenseSummary{TotalAmount: decimal.NewFromFloat(500000), ByCategory: []dto.ExpenseCategorySummary{}, Count: 5}, nil)
	pr.On("GetPurchaseSummary", ctx, req.StartDate, req.EndDate).Return(nil, errors.New("db error"))

	result, err := svc.GetFinancialSummary(ctx, req)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get purchase summary")
}

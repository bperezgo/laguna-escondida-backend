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

func dailyCloseRange() (time.Time, time.Time) {
	from := time.Date(2026, 7, 21, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 7, 21, 23, 59, 59, 0, time.UTC)
	return from, to
}

func TestGetDailyClose_Success(t *testing.T) {
	ctx := context.Background()
	svc, br, _, _ := createTestFinancialService(t)
	from, to := dailyCloseRange()

	rev := &dto.RevenueSummary{
		TotalAmount:   decimal.NewFromFloat(2000000), // net of tax
		TotalVAT:      decimal.NewFromFloat(300000),
		TotalICO:      decimal.NewFromFloat(50000),
		TotalDiscount: decimal.NewFromFloat(20000),
		TotalTip:      decimal.Zero,
		Count:         12,
	}
	// One row per payment kind — cards NOT bucketed.
	breakdown := []dto.PaymentMethodBreakdown{
		{PaymentMethod: "cash", Collected: decimal.NewFromFloat(1240000), Net: decimal.NewFromFloat(1050000), Count: 7},
		{PaymentMethod: "credit_card", Collected: decimal.NewFromFloat(420000), Net: decimal.NewFromFloat(360000), Count: 3},
		{PaymentMethod: "debit_card", Collected: decimal.NewFromFloat(260000), Net: decimal.NewFromFloat(220000), Count: 2},
	}

	br.On("GetRevenueSummary", ctx, from, to).Return(rev, nil)
	br.On("GetSalesByPaymentMethod", ctx, from, to).Return(breakdown, nil)

	result, err := svc.GetDailyClose(ctx, from, to)

	require.NoError(t, err)
	assert.Equal(t, from, result.StartDate)
	assert.Equal(t, to, result.EndDate)
	assert.Equal(t, 12, result.TotalOrders)
	// TotalCollected = sum of per-method gross = 1240000 + 420000 + 260000 = 1920000.
	assert.True(t, result.TotalCollected.Equal(decimal.NewFromFloat(1920000)),
		"TotalCollected must reconcile to the breakdown; got %s", result.TotalCollected)
	assert.True(t, result.TotalNet.Equal(decimal.NewFromFloat(2000000)))
	assert.True(t, result.TotalVAT.Equal(decimal.NewFromFloat(300000)))
	assert.True(t, result.TotalICO.Equal(decimal.NewFromFloat(50000)))
	assert.True(t, result.TotalDiscount.Equal(decimal.NewFromFloat(20000)))
	assert.True(t, result.TotalTip.Equal(decimal.Zero))
	assert.Equal(t, 3, len(result.ByPaymentMethod), "one row per payment kind, not bucketed")
	assert.Equal(t, "credit_card", result.ByPaymentMethod[1].PaymentMethod)
}

func TestGetDailyClose_Empty(t *testing.T) {
	ctx := context.Background()
	svc, br, _, _ := createTestFinancialService(t)
	from, to := dailyCloseRange()

	br.On("GetRevenueSummary", ctx, from, to).Return(&dto.RevenueSummary{}, nil)
	br.On("GetSalesByPaymentMethod", ctx, from, to).Return([]dto.PaymentMethodBreakdown{}, nil)

	result, err := svc.GetDailyClose(ctx, from, to)

	require.NoError(t, err)
	assert.Equal(t, 0, result.TotalOrders)
	assert.True(t, result.TotalCollected.Equal(decimal.Zero))
	assert.Equal(t, 0, len(result.ByPaymentMethod))
}

func TestGetDailyClose_RevenueError(t *testing.T) {
	ctx := context.Background()
	svc, br, _, _ := createTestFinancialService(t)
	from, to := dailyCloseRange()

	br.On("GetRevenueSummary", ctx, from, to).Return(nil, errors.New("db error"))

	result, err := svc.GetDailyClose(ctx, from, to)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get revenue summary")
}

func TestGetDailyClose_PaymentMethodError(t *testing.T) {
	ctx := context.Background()
	svc, br, _, _ := createTestFinancialService(t)
	from, to := dailyCloseRange()

	br.On("GetRevenueSummary", ctx, from, to).Return(&dto.RevenueSummary{Count: 1}, nil)
	br.On("GetSalesByPaymentMethod", ctx, from, to).Return(nil, errors.New("db error"))

	result, err := svc.GetDailyClose(ctx, from, to)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to get sales by payment method")
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

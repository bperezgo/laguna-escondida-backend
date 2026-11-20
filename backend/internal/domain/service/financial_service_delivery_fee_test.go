package service

import (
	"context"
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDailyClose_ReportsDeliveryFees(t *testing.T) {
	dec := decimal.RequireFromString
	tests := []struct {
		name           string
		fee, base, ico string
	}{
		{"fees of 5000 and 8000", "13000", "12037.04", "962.96"},
		{"no delivery", "0", "0", "0"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			svc, br, _, _ := createTestFinancialService(t)
			from, to := dailyCloseRange()
			br.On("GetRevenueSummary", ctx, from, to).Return(&dto.RevenueSummary{
				TotalAmount: dec("80000"), TotalICO: dec("6400"), Count: 3,
				TotalDeliveryFee: dec(tc.fee), TotalDeliveryFeeBase: dec(tc.base), TotalDeliveryFeeICO: dec(tc.ico),
			}, nil)
			br.On("GetSalesByPaymentMethod", ctx, from, to).Return([]dto.PaymentMethodBreakdown{
				{PaymentMethod: "cash", Collected: dec("86400"), Net: dec("80000"), Count: 3},
			}, nil)

			result, err := svc.GetDailyClose(ctx, from, to)

			require.NoError(t, err)
			assert.True(t, dec(tc.fee).Equal(result.TotalDeliveryFee))
			assert.True(t, dec(tc.base).Equal(result.TotalDeliveryFeeBase))
			assert.True(t, dec(tc.ico).Equal(result.TotalDeliveryFeeICO))
			assert.True(t, dec("86400").Equal(result.TotalCollected), "the existing totals are unchanged")
			assert.True(t, dec("80000").Equal(result.TotalNet))
		})
	}
}

func TestGetFinancialSummary_ReportsDeliveryFees(t *testing.T) {
	ctx := context.Background()
	svc, br, er, pr := createTestFinancialService(t)
	req := createTestFinancialRequest()
	br.On("GetRevenueSummary", ctx, req.StartDate, req.EndDate).Return(&dto.RevenueSummary{
		TotalAmount: decimal.NewFromInt(80000), TotalDeliveryFee: decimal.NewFromInt(13000),
		TotalDeliveryFeeBase: decimal.RequireFromString("12037.04"), TotalDeliveryFeeICO: decimal.RequireFromString("962.96"),
	}, nil)
	er.On("GetExpenseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.ExpenseSummary{}, nil)
	pr.On("GetPurchaseSummary", ctx, req.StartDate, req.EndDate).Return(&dto.PurchaseSummary{}, nil)

	result, err := svc.GetFinancialSummary(ctx, req)

	require.NoError(t, err)
	assert.True(t, decimal.NewFromInt(13000).Equal(result.Revenue.TotalDeliveryFee))
	assert.True(t, decimal.NewFromInt(80000).Equal(result.NetIncome), "net income is unchanged: the fee is already in the revenue total")
}

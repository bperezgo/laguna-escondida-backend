package bill

import (
	"testing"

	"laguna-escondida/backend/internal/domain/dto"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var dec = decimal.RequireFromString

func TestLineSplit(t *testing.T) {
	tests := []struct {
		name             string
		price            string
		qty              int
		vat, ico         string
		base, vatA, icoA string
	}{
		{"weighed product, 3352 g at $32 with 8% ICO", "32", 3352, "0", "0.08", "99318.52", "0", "7945.48"},
		{"one unit at $8,000 with 8% ICO", "8000", 1, "0", "0.08", "7407.41", "0", "592.59"},
		{"VAT 19% and ICO 8% on one line", "12700", 3, "0.19", "0.08", "30000", "5700", "2400"},
		{"VAT 19% and ICO 8%, uneven", "9999", 7, "0.19", "0.08", "55112.6", "10471.39", "4409.01"},
		{"large quantity", "3500", 7000, "0", "0.08", "22685185.19", "0", "1814814.81"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			line := SplitLine(dec(tc.price), tc.qty, dec(tc.vat), dec(tc.ico))

			assert.Equal(t, tc.base, line.Base.String(), "base")
			assert.Equal(t, tc.vatA, line.VAT.String(), "vat")
			assert.Equal(t, tc.icoA, line.ICO.String(), "ico")
			gross := dec(tc.price).Mul(decimal.NewFromInt(int64(tc.qty)))
			assert.True(t, line.Base.Add(line.VAT).Add(line.ICO).Equal(gross), "base + taxes is what was charged")
			// The provider recomputes each tax as line base × rate.
			cent := dec("0.01")
			assert.True(t, line.Base.Mul(dec(tc.vat)).Round(2).Sub(line.VAT).Abs().LessThanOrEqual(cent), "vat matches provider")
			assert.True(t, line.Base.Mul(dec(tc.ico)).Round(2).Sub(line.ICO).Abs().LessThanOrEqual(cent), "ico matches provider")
		})
	}
}

func TestLineNewBillProduct(t *testing.T) {
	p := NewBillProduct("prod-1", 3352, dec("32"), "Gramo Pescado de Lago", nil, "Pescados", "SKU1",
		[]dto.InvoiceAllowance{}, decimal.Zero, dec("0.08"))

	assert.Equal(t, "99318.52", p.Total().String())
	assert.Equal(t, "29.63", p.UnitPrice().String())
	assert.Equal(t, []dto.InvoiceTax{{TaxCode: dto.TaxCodeICO, TaxAmount: "7945.48", Percent: "8.00"}}, p.Taxes())
	assert.True(t, p.Total().Add(dec(p.Taxes()[0].TaxAmount)).Equal(dec("107264")), "base + taxes = price × qty")

	both := NewBillProduct("prod-2", 7, dec("9999"), "X", nil, "C", "SKU2", nil, dec("0.19"), dec("0.08"))
	require.Len(t, both.Taxes(), 2)
	assert.Equal(t, dto.TaxCodeVAT, both.Taxes()[0].TaxCode)
	assert.Equal(t, "19.00", both.Taxes()[0].Percent)
	sum := both.Total().Add(dec(both.Taxes()[0].TaxAmount)).Add(dec(both.Taxes()[1].TaxAmount))
	assert.True(t, sum.Equal(dec("69993")), "base + both taxes = price × qty")

	assert.Equal(t, "99318.52", p.ToDTO().Total.String(), "the DTO carries the line base")
}

func TestLineBillPaidStoresLineSums(t *testing.T) {
	agg, err := NewBillFromOpenBillWithProducts(&dto.OpenBillWithProducts{
		Products: []dto.OpenBillProductDetail{
			{Product: dto.Product{ID: "prod-1", Name: "Gramo Pescado de Lago", SKU: "SKU1",
				TotalPriceWithTaxes: dec("32"), ICO: dec("0.08"), UnitPrice: dec("29.63"), ICOAmount: dec("2.37")}, Quantity: 3000},
			{Product: dto.Product{ID: "prod-1", Name: "Gramo Pescado de Lago", SKU: "SKU1",
				TotalPriceWithTaxes: dec("32"), ICO: dec("0.08"), UnitPrice: dec("29.63"), ICOAmount: dec("2.37")}, Quantity: 352},
		},
	}, dto.ElectronicInvoicePaymentCodeCash, nil)
	require.NoError(t, err)

	b := agg.ToDTO()
	assert.Equal(t, "7945.48", b.ICO.String())
	assert.Equal(t, "0", b.VAT.String())
	assert.Equal(t, "99318.52", b.TotalAmount.String())
	assert.Equal(t, "107264", b.PayAmount.String())
	assert.True(t, b.TotalAmount.Add(b.VAT).Add(b.ICO).Sub(b.DiscountAmount).Add(b.Tip).Equal(b.PayAmount),
		"net + vat + ico - discount + tip = pay")
}

func TestLineApplyLineTotals(t *testing.T) {
	fee, base, ico, rate := dec("5000"), dec("4629.63"), dec("370.37"), dec("0.08")
	b := &dto.Bill{
		// Stored with per-unit rounding before the fix.
		TotalAmount: dec("99319.76").Add(base), ICO: dec("7944.24").Add(ico), PayAmount: dec("112264"),
		Products:    []dto.BillProduct{NewBillProduct("prod-1", 3352, dec("32"), "P", nil, "C", "S", nil, decimal.Zero, rate).ToDTO()},
		DeliveryFee: &fee, DeliveryFeeBase: &base, DeliveryFeeICO: &ico, DeliveryFeeICORate: &rate,
	}
	item, ok := DeliveryFeeItem(b)
	require.True(t, ok)
	assert.True(t, item.Total.Equal(base), "delivery line keeps its stored base")
	b.Products = append(b.Products, item)

	require.NoError(t, ApplyLineTotals(b))

	assert.Equal(t, "103948.15", b.TotalAmount.String())
	assert.Equal(t, "8315.85", b.TaxAmount.String())
	assert.Equal(t, "8315.85", b.ICO.String())
	assert.Equal(t, "112264", b.PayAmount.String(), "pay amount is what was charged")
	assert.True(t, b.TotalAmount.Add(b.TaxAmount).Equal(b.PayAmount))
}

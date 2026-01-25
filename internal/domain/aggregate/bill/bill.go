package bill

import (
	"time"

	billError "laguna-escondida/backend/internal/domain/aggregate/bill/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/samber/lo"
	"github.com/shopspring/decimal"
)

type Aggregate struct {
	id             string
	totalAmount    decimal.Decimal
	discountAmount decimal.Decimal
	taxAmount      decimal.Decimal
	payAmount      decimal.Decimal
	vat            decimal.Decimal
	ico            decimal.Decimal
	tip            decimal.Decimal
	documentURL    *string
	customer       *dto.Customer
	paymentCode    *PaymentCode
	products       []*BillProduct
	createdAt      time.Time
	updatedAt      time.Time
}

func NewBillFromCreateElectronicInvoiceRequest(invoice *dto.ElectronicInvoice, products []*BillProduct) (*Aggregate, error) {
	if len(products) == 0 {
		return nil, billError.NewProductsCannotBeEmptyError()
	}

	paymentCode, err := NewPaymentCode(invoice.PaymentCode)
	if err != nil {
		return nil, err
	}

	totalAmount := decimal.Zero
	discountAmount := decimal.Zero
	taxAmount := decimal.Zero
	totalVat := decimal.Zero
	totalIco := decimal.Zero
	totalTip := decimal.Zero

	for _, product := range products {
		totalAmount = totalAmount.Add(product.unitPrice.Mul(decimal.NewFromInt(int64(product.quantity))))

		for _, allowance := range product.allowance {
			allowanceAmount, err := decimal.NewFromString(allowance.Amount)
			if err != nil {
				return nil, billError.NewInvalidAllowanceAmountError(allowance.Amount)
			}
			discountAmount = discountAmount.Add(allowanceAmount)
		}

		for _, tax := range product.taxes {
			parsedTaxAmount, err := decimal.NewFromString(tax.TaxAmount)
			if err != nil {
				return nil, billError.NewInvalidTaxAmountError(tax.TaxAmount)
			}

			switch tax.TaxCode {
			case dto.TaxCodeVAT:
				totalVat = totalVat.Add(parsedTaxAmount)
			case dto.TaxCodeICO:
				totalIco = totalIco.Add(parsedTaxAmount)
			}
			taxAmount = taxAmount.Add(parsedTaxAmount)
		}
	}

	payAmount := totalAmount.Add(taxAmount).Sub(discountAmount)

	return &Aggregate{
		id:             uuid.New().String(),
		totalAmount:    totalAmount,
		discountAmount: discountAmount,
		taxAmount:      taxAmount,
		payAmount:      payAmount,
		vat:            totalVat,
		ico:            totalIco,
		tip:            totalTip,
		documentURL:    nil,
		customer:       invoice.Customer,
		paymentCode:    paymentCode,
		products:       products,
		createdAt:      time.Now(),
		updatedAt:      time.Now(),
	}, nil
}

func (a *Aggregate) ToDTO() *dto.Bill {
	return &dto.Bill{
		ID:             a.id,
		TotalAmount:    a.totalAmount,
		DiscountAmount: a.discountAmount,
		TaxAmount:      a.taxAmount,
		PayAmount:      a.payAmount,
		CreatedAt:      a.createdAt,
		UpdatedAt:      a.updatedAt,
		VAT:            a.vat,
		ICO:            a.ico,
		Tip:            a.tip,
		DocumentURL:    a.documentURL,
		Customer:       a.customer,
		Products: lo.Map(a.products, func(product *BillProduct, _ int) dto.BillProduct {
			return dto.BillProduct{
				ProductID:   product.id,
				Quantity:    product.quantity,
				UnitPrice:   product.unitPrice,
				Name:        product.name,
				Description: product.description,
				Category:    product.category,
				Code:        product.code,
				Allowance:   product.allowance,
				Taxes:       product.taxes,
			}
		}),
	}
}

func groupProductsByID(products []dto.OpenBillProductDetail) (map[string]*dto.OpenBillProductDetail, map[string]int) {
	productMap := make(map[string]*dto.OpenBillProductDetail)
	quantityMap := make(map[string]int)

	for _, productDetail := range products {
		productID := productDetail.Product.ID

		if _, exists := productMap[productID]; exists {
			quantityMap[productID] += productDetail.Quantity
			continue
		}

		productMap[productID] = &productDetail
		quantityMap[productID] = productDetail.Quantity
	}

	return productMap, quantityMap
}

func NewBillFromOpenBillWithProducts(openBillWithProducts *dto.OpenBillWithProducts, paymentCode dto.ElectronicInvoicePaymentCode, customer *dto.Customer) (*Aggregate, error) {
	if len(openBillWithProducts.Products) == 0 {
		return nil, billError.NewProductsCannotBeEmptyError()
	}

	productMap, quantityMap := groupProductsByID(openBillWithProducts.Products)

	items := make([]dto.InvoiceItem, 0, len(productMap))
	for productID, totalQuantity := range quantityMap {
		items = append(items, dto.InvoiceItem{
			Quantity:  totalQuantity,
			ProductID: productID,
			Allowance: []dto.InvoiceAllowance{},
		})
	}

	billProducts := make([]*BillProduct, 0, len(productMap))
	for productID, productDetail := range productMap {
		totalQuantity := quantityMap[productID]
		billProducts = append(billProducts, NewBillProduct(
			productDetail.Product.ID,
			totalQuantity,
			productDetail.Product.UnitPrice,
			productDetail.Product.Name,
			productDetail.Product.Description,
			productDetail.Product.Category,
			productDetail.Product.SKU,
			[]dto.InvoiceAllowance{},
			productDetail.Product.VAT,
			productDetail.Product.ICO,
		))
	}

	return NewBillFromCreateElectronicInvoiceRequest(
		&dto.ElectronicInvoice{
			PaymentCode: paymentCode,
			Customer:    customer,
			Items:       items,
		},
		billProducts,
	)
}

func (a *Aggregate) Products() []*BillProduct {
	return a.products
}

func (a *Aggregate) PaymentCode() dto.ElectronicInvoicePaymentCode {
	return a.paymentCode.Value()
}

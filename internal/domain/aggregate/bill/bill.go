package bill

import (
	billError "laguna-escondida/backend/internal/domain/aggregate/bill/error"
	"laguna-escondida/backend/internal/domain/dto"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/samber/lo"
)

type Aggregate struct {
	id             string
	totalAmount    float64
	discountAmount float64
	taxAmount      float64
	payAmount      float64
	vat            float64
	ico            float64
	tip            float64
	documentURL    *string
	customer       *dto.Customer
	paymentCode    dto.ElectronicInvoicePaymentCode
	products       []*BillProduct
	createdAt      time.Time
	updatedAt      time.Time
}

func NewBillFromCreateElectronicInvoiceRequest(invoice *dto.ElectronicInvoice, products []*BillProduct) (*Aggregate, error) {
	if len(products) == 0 {
		return nil, billError.NewProductsCannotBeEmptyError()
	}

	totalAmount := 0.0
	discountAmount := 0.0
	taxAmount := 0.0
	payAmount := 0.0
	totalVat := 0.0
	totalIco := 0.0
	totalTip := 0.0

	for _, product := range products {
		totalAmount += product.unitPrice * float64(product.quantity)

		for _, allowance := range product.allowance {
			allowanceAmount, err := strconv.ParseFloat(allowance.Amount, 64)
			if err != nil {
				return nil, billError.NewInvalidAllowanceAmountError(allowance.Amount)
			}
			discountAmount += allowanceAmount
		}

		for _, tax := range product.taxes {
			parsedTaxAmount, err := strconv.ParseFloat(tax.TaxAmount, 64)
			if err != nil {
				return nil, billError.NewInvalidTaxAmountError(tax.TaxAmount)
			}

			switch tax.TaxCode {
			case dto.TaxCodeVAT:
				totalVat += parsedTaxAmount
			case dto.TaxCodeICO:
				totalIco += parsedTaxAmount
			}
			taxAmount += parsedTaxAmount
		}
	}

	payAmount = totalAmount + taxAmount - discountAmount

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
		paymentCode:    invoice.PaymentCode,
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
				Brand:       product.brand,
				Model:       product.model,
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
			productDetail.Product.Brand,
			productDetail.Product.Model,
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
	return a.paymentCode
}

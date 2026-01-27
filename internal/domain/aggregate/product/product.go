package product

import (
	"time"

	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Aggregate struct {
	id                  string
	name                string
	category            string
	productType         ProductType
	unitOfMeasure       UnitOfMeasure
	version             int
	unitPrice           decimal.Decimal
	vat                 decimal.Decimal
	vatAmount           decimal.Decimal
	ico                 decimal.Decimal
	icoAmount           decimal.Decimal
	description         string
	sku                 *SKU
	totalPriceWithTaxes decimal.Decimal
	createdAt           time.Time
	updatedAt           time.Time
}

type taxCalculationResult struct {
	totalPriceWithTaxes decimal.Decimal
	vatPercentage       decimal.Decimal
	vatAmount           decimal.Decimal
	icoPercentage       decimal.Decimal
	icoAmount           decimal.Decimal
	unitPrice           decimal.Decimal
}

// calculateTaxesAndUnitPrice parses and validates tax values, then calculates tax amounts and unit price
// Uses proportional calculation to ensure: unitPrice + vatAmount + icoAmount = totalPriceWithTaxes (exact)
func calculateTaxesAndUnitPrice(totalPriceWithTaxesStr, vatStr, icoStr, taxesFormat string) (*taxCalculationResult, error) {
	totalPriceWithTaxes, err := decimal.NewFromString(totalPriceWithTaxesStr)
	if err != nil {
		return nil, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be a number", totalPriceWithTaxesStr)
	}
	if totalPriceWithTaxes.LessThanOrEqual(decimal.Zero) {
		return nil, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be greater than 0", totalPriceWithTaxesStr)
	}

	vat, err := decimal.NewFromString(vatStr)
	if err != nil {
		return nil, productError.NewInvalidVATError("vat must be a number", vatStr)
	}
	if vat.LessThan(decimal.Zero) {
		return nil, productError.NewInvalidVATError("vat must be greater than or equal to 0", vatStr)
	}

	ico, err := decimal.NewFromString(icoStr)
	if err != nil {
		return nil, productError.NewInvalidICOError("ico must be a number", icoStr)
	}
	if ico.LessThan(decimal.Zero) {
		return nil, productError.NewInvalidICOError("ico must be greater than or equal to 0", icoStr)
	}

	taxSum := vat.Add(ico)
	if taxSum.IsZero() {
		return nil, productError.NewInvalidTaxCalculationErrorWithField("vat and ico cannot both be 0 (would result in division by zero)", map[string]string{"vat": vatStr, "ico": icoStr})
	}

	if taxesFormat != "percentage" {
		return nil, productError.NewInvalidTaxCalculationErrorWithField("taxes_format must be 'percentage'", taxesFormat)
	}

	hundred := decimal.NewFromInt(100)
	vatPercentage := vat.Div(hundred)
	icoPercentage := ico.Div(hundred)
	taxSumPercentage := vatPercentage.Add(icoPercentage)
	divisor := decimal.NewFromInt(1).Add(taxSumPercentage)

	// Calculate tax amounts directly from total (proportionally)
	// This ensures: unitPrice + vatAmount + icoAmount = totalPriceWithTaxes
	vatAmount := totalPriceWithTaxes.Mul(vatPercentage).Div(divisor).Round(2)
	icoAmount := totalPriceWithTaxes.Mul(icoPercentage).Div(divisor).Round(2)

	// Unit price absorbs any rounding difference to guarantee exact equality
	unitPrice := totalPriceWithTaxes.Sub(vatAmount).Sub(icoAmount)

	return &taxCalculationResult{
		totalPriceWithTaxes: totalPriceWithTaxes,
		vatPercentage:       vatPercentage,
		vatAmount:           vatAmount,
		icoPercentage:       icoPercentage,
		icoAmount:           icoAmount,
		unitPrice:           unitPrice,
	}, nil
}

func NewAggregateFromDTO(dto *dto.Product) (*Aggregate, error) {
	description := ""
	if dto.Description != nil {
		description = *dto.Description
	}
	sku, err := NewSKU(dto.SKU)
	if err != nil {
		return nil, productError.NewInvalidSKUError(dto.SKU)
	}

	productType, err := NewProductType(string(dto.ProductType))
	if err != nil {
		return nil, err
	}

	unitOfMeasure, err := NewUnitOfMeasure(string(dto.UnitOfMeasure))
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                  dto.ID,
		name:                dto.Name,
		category:            dto.Category,
		productType:         productType,
		unitOfMeasure:       unitOfMeasure,
		version:             dto.Version,
		unitPrice:           dto.UnitPrice,
		vat:                 dto.VAT,
		vatAmount:           dto.VATAmount,
		ico:                 dto.ICO,
		icoAmount:           dto.ICOAmount,
		description:         description,
		sku:                 sku,
		totalPriceWithTaxes: dto.TotalPriceWithTaxes,
		createdAt:           dto.CreatedAt,
		updatedAt:           dto.UpdatedAt,
	}, nil
}

func NewFromOpenBillProducts(products []dto.OpenBillProductDetail) ([]*Aggregate, error) {
	productMap := groupProductsByID(products)

	productAggregates := make([]*Aggregate, 0, len(productMap))
	for _, productDetail := range productMap {
		productAggregate, err := NewAggregateFromDTO(&productDetail.Product)
		if err != nil {
			return nil, err
		}
		productAggregates = append(productAggregates, productAggregate)
	}
	return productAggregates, nil
}

func groupProductsByID(products []dto.OpenBillProductDetail) map[string]*dto.OpenBillProductDetail {
	productMap := make(map[string]*dto.OpenBillProductDetail)

	for _, productDetail := range products {
		productID := productDetail.Product.ID
		if _, exists := productMap[productID]; exists {
			continue
		}
		productMap[productID] = &productDetail
	}
	return productMap
}

func NewAggregateFromCreateProductRequest(req *dto.CreateProductRequest) (*Aggregate, error) {
	if req == nil {
		return nil, productError.NewInvalidRequestError("request cannot be nil")
	}

	if req.Name == "" {
		return nil, productError.NewMissingNameError()
	}
	if req.Category == "" {
		return nil, productError.NewMissingCategoryError()
	}

	sku, err := NewSKU(req.SKU)
	if err != nil {
		return nil, err
	}

	productType, err := NewProductType(req.ProductType)
	if err != nil {
		return nil, err
	}

	unitOfMeasure, err := NewUnitOfMeasure(req.UnitOfMeasure)
	if err != nil {
		return nil, err
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	now := time.Now()

	// For INGREDIENT type, price/taxes are optional
	if !productType.IsSellable() {
		return &Aggregate{
			id:                  uuid.New().String(),
			name:                req.Name,
			category:            req.Category,
			productType:         productType,
			unitOfMeasure:       unitOfMeasure,
			version:             1,
			unitPrice:           decimal.Zero,
			vat:                 decimal.Zero,
			vatAmount:           decimal.Zero,
			ico:                 decimal.Zero,
			icoAmount:           decimal.Zero,
			description:         description,
			sku:                 sku,
			totalPriceWithTaxes: decimal.Zero,
			createdAt:           now,
			updatedAt:           now,
		}, nil
	}

	taxResult, err := calculateTaxesAndUnitPrice(
		req.TotalPriceWithTaxes,
		req.VAT,
		req.ICO,
		req.TaxesFormat,
	)
	if err != nil {
		return nil, err
	}

	return &Aggregate{
		id:                  uuid.New().String(),
		name:                req.Name,
		category:            req.Category,
		productType:         productType,
		unitOfMeasure:       unitOfMeasure,
		version:             1,
		unitPrice:           taxResult.unitPrice,
		vat:                 taxResult.vatPercentage,
		vatAmount:           taxResult.vatAmount,
		ico:                 taxResult.icoPercentage,
		icoAmount:           taxResult.icoAmount,
		description:         description,
		sku:                 sku,
		totalPriceWithTaxes: taxResult.totalPriceWithTaxes,
		createdAt:           now,
		updatedAt:           now,
	}, nil
}

func (a *Aggregate) ToDTO() *dto.Product {
	return &dto.Product{
		ID:                  a.id,
		Name:                a.name,
		Category:            a.category,
		ProductType:         dto.ProductType(a.productType),
		UnitOfMeasure:       dto.UnitOfMeasure(a.unitOfMeasure),
		Version:             a.version,
		UnitPrice:           a.unitPrice,
		VAT:                 a.vat,
		VATAmount:           a.vatAmount,
		ICO:                 a.ico,
		ICOAmount:           a.icoAmount,
		Description:         &a.description,
		SKU:                 a.sku.Value(),
		TotalPriceWithTaxes: a.totalPriceWithTaxes,
		CreatedAt:           a.createdAt,
		UpdatedAt:           a.updatedAt,
	}
}

func (a *Aggregate) Update(req *dto.UpdateProductRequest) (*Aggregate, error) {
	sku, err := NewSKU(req.SKU)
	if err != nil {
		return nil, err
	}

	productType, err := NewProductType(req.ProductType)
	if err != nil {
		return nil, err
	}

	unitOfMeasure, err := NewUnitOfMeasure(req.UnitOfMeasure)
	if err != nil {
		return nil, err
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	a.name = req.Name
	a.category = req.Category
	a.productType = productType
	a.unitOfMeasure = unitOfMeasure
	// We'll let the logic of this version for another moment, the idea behind this is to change the version if the price changes
	//  To validate how the system behaves with different prices (Split Tests)
	a.version = 1

	// For INGREDIENT type, price/taxes are optional
	if !productType.IsSellable() {
		a.totalPriceWithTaxes = decimal.Zero
		a.vat = decimal.Zero
		a.vatAmount = decimal.Zero
		a.ico = decimal.Zero
		a.icoAmount = decimal.Zero
		a.unitPrice = decimal.Zero
	} else {
		taxResult, err := calculateTaxesAndUnitPrice(
			req.TotalPriceWithTaxes,
			req.VAT,
			req.ICO,
			req.TaxesFormat,
		)
		if err != nil {
			return nil, err
		}

		a.totalPriceWithTaxes = taxResult.totalPriceWithTaxes
		a.vat = taxResult.vatPercentage
		a.vatAmount = taxResult.vatAmount
		a.ico = taxResult.icoPercentage
		a.icoAmount = taxResult.icoAmount
		a.unitPrice = taxResult.unitPrice
	}

	a.description = description
	a.sku = sku
	a.updatedAt = time.Now()

	return a, nil
}

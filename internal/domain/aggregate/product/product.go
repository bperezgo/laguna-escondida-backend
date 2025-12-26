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
	version             int
	unitPrice           decimal.Decimal
	vat                 decimal.Decimal
	ico                 decimal.Decimal
	description         string
	brand               string
	model               string
	sku                 *SKU
	totalPriceWithTaxes decimal.Decimal
	createdAt           time.Time
	updatedAt           time.Time
}

// calculateTaxesAndUnitPrice parses and validates tax values, then calculates unit price
// Returns: totalPriceWithTaxes, vat (as decimal percentage), ico (as decimal percentage), unitPrice, error
func calculateTaxesAndUnitPrice(totalPriceWithTaxesStr, vatStr, icoStr, taxesFormat string) (decimal.Decimal, decimal.Decimal, decimal.Decimal, decimal.Decimal, error) {
	totalPriceWithTaxes, err := decimal.NewFromString(totalPriceWithTaxesStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be a number", totalPriceWithTaxesStr)
	}
	if totalPriceWithTaxes.LessThanOrEqual(decimal.Zero) {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be greater than 0", totalPriceWithTaxesStr)
	}

	vat, err := decimal.NewFromString(vatStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidVATError("vat must be a number", vatStr)
	}
	if vat.LessThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidVATError("vat must be greater than or equal to 0", vatStr)
	}

	ico, err := decimal.NewFromString(icoStr)
	if err != nil {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidICOError("ico must be a number", icoStr)
	}
	if ico.LessThan(decimal.Zero) {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidICOError("ico must be greater than or equal to 0", icoStr)
	}

	taxSum := vat.Add(ico)
	if taxSum.IsZero() {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidTaxCalculationErrorWithField("vat and ico cannot both be 0 (would result in division by zero)", map[string]string{"vat": vatStr, "ico": icoStr})
	}

	if taxesFormat != "percentage" {
		return decimal.Zero, decimal.Zero, decimal.Zero, decimal.Zero, productError.NewInvalidTaxCalculationErrorWithField("taxes_format must be 'percentage'", taxesFormat)
	}

	hundred := decimal.NewFromInt(100)
	vatPercentage := vat.Div(hundred)
	icoPercentage := ico.Div(hundred)
	taxSumPercentage := vatPercentage.Add(icoPercentage)
	unitPrice := totalPriceWithTaxes.Div(decimal.NewFromInt(1).Add(taxSumPercentage))

	// Round unitPrice to 2 decimal places
	unitPrice = unitPrice.Round(2)

	return totalPriceWithTaxes, vatPercentage, icoPercentage, unitPrice, nil
}

func NewAggregateFromDTO(dto *dto.Product) (*Aggregate, error) {
	description := ""
	if dto.Description != nil {
		description = *dto.Description
	}
	brand := ""
	if dto.Brand != nil {
		brand = *dto.Brand
	}
	model := ""
	if dto.Model != nil {
		model = *dto.Model
	}
	sku, err := NewSKU(dto.SKU)
	if err != nil {
		return nil, productError.NewInvalidSKUError(dto.SKU)
	}
	return &Aggregate{
		id:                  dto.ID,
		name:                dto.Name,
		category:            dto.Category,
		version:             dto.Version,
		unitPrice:           dto.UnitPrice,
		vat:                 dto.VAT,
		ico:                 dto.ICO,
		description:         description,
		brand:               brand,
		model:               model,
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

	// Validate required fields
	if req.Name == "" {
		return nil, productError.NewMissingNameError()
	}
	if req.Category == "" {
		return nil, productError.NewMissingCategoryError()
	}

	// Create and validate SKU value object
	sku, err := NewSKU(req.SKU)
	if err != nil {
		return nil, err
	}

	totalPriceWithTaxes, vatDecimal, icoDecimal, unitPrice, err := calculateTaxesAndUnitPrice(
		req.TotalPriceWithTaxes,
		req.VAT,
		req.ICO,
		req.TaxesFormat,
	)
	if err != nil {
		return nil, err
	}

	// Handle nullable fields (Description, Brand, Model)
	description := ""
	if req.Description != nil {
		description = *req.Description
	}

	brand := "unknown"
	if req.Brand != nil {
		brand = *req.Brand
	}

	model := "unknown"
	if req.Model != nil {
		model = *req.Model
	}

	now := time.Now()
	return &Aggregate{
		id:                  uuid.New().String(),
		name:                req.Name,
		category:            req.Category,
		version:             1,
		unitPrice:           unitPrice,
		vat:                 vatDecimal,
		ico:                 icoDecimal,
		description:         description,
		brand:               brand,
		model:               model,
		sku:                 sku,
		totalPriceWithTaxes: totalPriceWithTaxes,
		createdAt:           now,
		updatedAt:           now,
	}, nil
}

func (a *Aggregate) ToDTO() *dto.Product {
	return &dto.Product{
		ID:                  a.id,
		Name:                a.name,
		Category:            a.category,
		Version:             a.version,
		UnitPrice:           a.unitPrice,
		VAT:                 a.vat,
		ICO:                 a.ico,
		Description:         &a.description,
		Brand:               &a.brand,
		Model:               &a.model,
		SKU:                 a.sku.Value(),
		TotalPriceWithTaxes: a.totalPriceWithTaxes,
		CreatedAt:           a.createdAt,
		UpdatedAt:           a.updatedAt,
	}
}

func (a *Aggregate) Update(req *dto.UpdateProductRequest) (*Aggregate, error) {
	// Create and validate SKU value object
	sku, err := NewSKU(req.SKU)
	if err != nil {
		return nil, err
	}

	description := ""
	if req.Description != nil {
		description = *req.Description
	}
	brand := ""
	if req.Brand != nil {
		brand = *req.Brand
	}
	model := ""
	if req.Model != nil {
		model = *req.Model
	}
	a.name = req.Name
	a.category = req.Category
	// We'll let the logic of this version for another moment, the idea behind this is to change the version if the price changes
	//  To validate how the system behaves with different prices (Split Tests)
	a.version = 1

	totalPriceWithTaxes, vatDecimal, icoDecimal, unitPrice, err := calculateTaxesAndUnitPrice(
		req.TotalPriceWithTaxes,
		req.VAT,
		req.ICO,
		req.TaxesFormat,
	)
	if err != nil {
		return nil, err
	}

	a.totalPriceWithTaxes = totalPriceWithTaxes
	a.vat = vatDecimal
	a.ico = icoDecimal
	a.description = description
	a.brand = brand
	a.model = model
	a.sku = sku
	a.unitPrice = unitPrice
	a.updatedAt = time.Now()

	return a, nil
}

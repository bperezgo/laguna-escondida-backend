package product

import (
	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
	"laguna-escondida/backend/internal/domain/dto"
	"math"
	"strconv"
	"time"

	"github.com/google/uuid"
)

type Aggregate struct {
	id                  string
	name                string
	category            string
	version             int
	unitPrice           float64
	vat                 float64
	ico                 float64
	description         string
	brand               string
	model               string
	sku                 string
	totalPriceWithTaxes float64
	createdAt           time.Time
	updatedAt           time.Time
}

// calculateTaxesAndUnitPrice parses and validates tax values, then calculates unit price
// Returns: totalPriceWithTaxes, vat (as decimal), ico (as decimal), unitPrice, error
func calculateTaxesAndUnitPrice(totalPriceWithTaxesStr, vatStr, icoStr, taxesFormat string) (float64, float64, float64, float64, error) {
	totalPriceWithTaxes, err := strconv.ParseFloat(totalPriceWithTaxesStr, 64)
	if err != nil {
		return 0, 0, 0, 0, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be a number", totalPriceWithTaxesStr)
	}
	if totalPriceWithTaxes <= 0 {
		return 0, 0, 0, 0, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be greater than 0", totalPriceWithTaxesStr)
	}

	vat, err := strconv.ParseFloat(vatStr, 64)
	if err != nil {
		return 0, 0, 0, 0, productError.NewInvalidVATError("vat must be a number", vatStr)
	}
	if vat < 0 {
		return 0, 0, 0, 0, productError.NewInvalidVATError("vat must be greater than or equal to 0", vatStr)
	}

	ico, err := strconv.ParseFloat(icoStr, 64)
	if err != nil {
		return 0, 0, 0, 0, productError.NewInvalidICOError("ico must be a number", icoStr)
	}
	if ico < 0 {
		return 0, 0, 0, 0, productError.NewInvalidICOError("ico must be greater than or equal to 0", icoStr)
	}

	taxSum := vat + ico
	if taxSum == 0 {
		return 0, 0, 0, 0, productError.NewInvalidTaxCalculationErrorWithField("vat and ico cannot both be 0 (would result in division by zero)", map[string]string{"vat": vatStr, "ico": icoStr})
	}

	if taxesFormat != "percentage" {
		return 0, 0, 0, 0, productError.NewInvalidTaxCalculationErrorWithField("taxes_format must be 'percentage'", taxesFormat)
	}

	vatPercentage := vat / 100
	icoPercentage := ico / 100
	taxSumPercentage := (vat + ico) / 100
	unitPrice := totalPriceWithTaxes / (1 + taxSumPercentage)

	// Round unitPrice to 2 decimal places
	unitPrice = math.Round(unitPrice*100) / 100

	return totalPriceWithTaxes, vatPercentage, icoPercentage, unitPrice, nil
}

func NewAggregateFromDTO(dto *dto.Product) *Aggregate {
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
		sku:                 dto.SKU,
		totalPriceWithTaxes: dto.TotalPriceWithTaxes,
		createdAt:           dto.CreatedAt,
		updatedAt:           dto.UpdatedAt,
	}
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
	if req.SKU == "" {
		return nil, productError.NewMissingSKUError()
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
		sku:                 req.SKU,
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
		SKU:                 a.sku,
		TotalPriceWithTaxes: a.totalPriceWithTaxes,
		CreatedAt:           a.createdAt,
		UpdatedAt:           a.updatedAt,
	}
}

func (a *Aggregate) Update(req *dto.UpdateProductRequest) (*Aggregate, error) {
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
	a.sku = req.SKU
	a.unitPrice = unitPrice
	a.updatedAt = time.Now()

	return a, nil
}

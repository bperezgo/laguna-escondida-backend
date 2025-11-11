package product

import (
	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
	"laguna-escondida/backend/internal/domain/dto"
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
	totalPriceWithTaxes, err := strconv.ParseFloat(req.TotalPriceWithTaxes, 64)
	if err != nil {
		return nil, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be a number", req.TotalPriceWithTaxes)
	}
	if totalPriceWithTaxes <= 0 {
		return nil, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be greater than 0", req.TotalPriceWithTaxes)
	}
	vat, err := strconv.ParseFloat(req.VAT, 64)
	if err != nil {
		return nil, productError.NewInvalidVATError("vat must be a number", req.VAT)
	}
	if vat < 0 {
		return nil, productError.NewInvalidVATError("vat must be greater than or equal to 0", req.VAT)
	}
	ico, err := strconv.ParseFloat(req.ICO, 64)
	if err != nil {
		return nil, productError.NewInvalidICOError("ico must be a number", req.ICO)
	}
	if ico < 0 {
		return nil, productError.NewInvalidICOError("ico must be greater than or equal to 0", req.ICO)
	}

	// Calculate unitPrice from totalPriceWithTaxes
	// Formula: unitPrice = TotalPriceWithTaxes / (VAT + ICO)
	// Where totalPriceWithTaxes = unitPrice * (VAT + ICO)
	taxSum := vat + ico
	if taxSum == 0 {
		return nil, productError.NewInvalidTaxCalculationErrorWithField("vat and ico cannot both be 0 (would result in division by zero)", map[string]string{"vat": req.VAT, "ico": req.ICO})
	}
	if req.TaxesFormat != "percentage" {
		return nil, productError.NewInvalidTaxCalculationErrorWithField("taxes_format must be 'percentage'", req.TaxesFormat)
	}
	unitPrice := totalPriceWithTaxes / (vat + ico)

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
		vat:                 vat,
		ico:                 ico,
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

	totalPriceWithTaxes, err := strconv.ParseFloat(req.TotalPriceWithTaxes, 64)
	if err != nil {
		return nil, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be a number", req.TotalPriceWithTaxes)
	}
	vat, err := strconv.ParseFloat(req.VAT, 64)
	if err != nil {
		return nil, productError.NewInvalidVATError("vat must be a number", req.VAT)
	}
	ico, err := strconv.ParseFloat(req.ICO, 64)
	if err != nil {
		return nil, productError.NewInvalidICOError("ico must be a number", req.ICO)
	}
	taxSum := vat + ico
	if taxSum == 0 {
		return nil, productError.NewInvalidTaxCalculationErrorWithField("vat and ico cannot both be 0 (would result in division by zero)", map[string]float64{"vat": vat, "ico": ico})
	}
	unitPrice := totalPriceWithTaxes / taxSum
	a.totalPriceWithTaxes = totalPriceWithTaxes
	a.vat = vat
	a.ico = ico
	a.description = description
	a.brand = brand
	a.model = model
	a.sku = req.SKU
	a.unitPrice = unitPrice
	a.totalPriceWithTaxes = totalPriceWithTaxes
	a.updatedAt = time.Now()

	return a, nil
}

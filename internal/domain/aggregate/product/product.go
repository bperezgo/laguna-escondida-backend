package product

import (
	productError "laguna-escondida/backend/internal/domain/aggregate/product/error"
	"laguna-escondida/backend/internal/domain/dto"

	"github.com/google/uuid"
)

type Aggregate struct {
	ID          string
	Name        string
	Category    string
	Version     int
	UnitPrice   float64
	VAT         float64
	ICO         float64
	Description string
	Brand       string
	Model       string
	SKU         string
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
	if req.TotalPriceWithTaxes <= 0 {
		return nil, productError.NewInvalidPriceErrorWithField("total_price_with_taxes must be greater than 0", req.TotalPriceWithTaxes)
	}
	if req.VAT < 0 {
		return nil, productError.NewInvalidVATError("vat must be greater than or equal to 0", req.VAT)
	}
	if req.ICO < 0 {
		return nil, productError.NewInvalidICOError("ico must be greater than or equal to 0", req.ICO)
	}

	// Calculate unitPrice from totalPriceWithTaxes
	// Formula: unitPrice = TotalPriceWithTaxes / (VAT + ICO)
	// Where totalPriceWithTaxes = unitPrice * (VAT + ICO)
	taxSum := req.VAT + req.ICO
	if taxSum == 0 {
		return nil, productError.NewInvalidTaxCalculationErrorWithField("vat and ico cannot both be 0 (would result in division by zero)", map[string]float64{"vat": req.VAT, "ico": req.ICO})
	}
	unitPrice := req.TotalPriceWithTaxes / taxSum

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

	return &Aggregate{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Category:    req.Category,
		Version:     1,
		UnitPrice:   unitPrice,
		VAT:         req.VAT,
		ICO:         req.ICO,
		Description: description,
		Brand:       brand,
		Model:       model,
		SKU:         req.SKU,
	}, nil
}

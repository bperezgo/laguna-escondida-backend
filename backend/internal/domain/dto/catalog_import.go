package dto

import "github.com/shopspring/decimal"

// CatalogImportDocument is a whole catalog in one document. Entries point at each other by
// a local ref, never by product id; an existing product is reachable only through Existing.
type CatalogImportDocument struct {
	Defaults     *CatalogImportDefaults    `json:"defaults,omitempty"`
	Existing     []CatalogImportExisting   `json:"existing,omitempty"`
	Ingredients  []CatalogImportIngredient `json:"ingredients,omitempty"`
	Sellables    []CatalogImportSellable   `json:"sellables,omitempty"`
	Plates       []CatalogImportPlate      `json:"plates,omitempty"`
	OpeningStock []CatalogImportStockCount `json:"opening_stock,omitempty"`
}

// ApplyCatalogImportRequest is the document plus the digest validation returned for it.
type ApplyCatalogImportRequest struct {
	CatalogImportDocument
	ExpectedDigest string `json:"expected_digest"`
}

type CatalogImportDefaults struct {
	VAT           *decimal.Decimal `json:"vat,omitempty"`
	ICO           *decimal.Decimal `json:"ico,omitempty"`
	TaxesFormat   *string          `json:"taxes_format,omitempty"`
	UnitOfMeasure *string          `json:"unit_of_measure,omitempty"`
	PrepArea      *string          `json:"prep_area,omitempty"`
	PrepPriority  *int             `json:"prep_priority,omitempty"`
}

// CatalogImportExisting names one existing product of the organization by SKU or by name.
type CatalogImportExisting struct {
	Ref  string `json:"ref"`
	SKU  string `json:"sku,omitempty"`
	Name string `json:"name,omitempty"`
}

type CatalogImportPrep struct {
	Area     *string `json:"area,omitempty"`
	Priority *int    `json:"priority,omitempty"`
}

// CatalogImportProductFields are the product fields every entry kind shares.
type CatalogImportProductFields struct {
	Ref                 string             `json:"ref"`
	Name                string             `json:"name"`
	Category            string             `json:"category"`
	SKU                 *string            `json:"sku,omitempty"`
	Description         *string            `json:"description,omitempty"`
	UnitOfMeasure       *string            `json:"unit_of_measure,omitempty"`
	TotalPriceWithTaxes *decimal.Decimal   `json:"total_price_with_taxes,omitempty"`
	VAT                 *decimal.Decimal   `json:"vat,omitempty"`
	ICO                 *decimal.Decimal   `json:"ico,omitempty"`
	TaxesFormat         *string            `json:"taxes_format,omitempty"`
	Prep                *CatalogImportPrep `json:"prep,omitempty"`
}

// CatalogImportIngredient is INGREDIENT (the default) or BOTH.
type CatalogImportIngredient struct {
	CatalogImportProductFields
	ProductType string `json:"product_type,omitempty"`
}

type CatalogImportSellable struct {
	CatalogImportProductFields
}

// CatalogImportPlate is created as COMPOSITE.
type CatalogImportPlate struct {
	CatalogImportProductFields
	Recipe []CatalogImportRecipeLine `json:"recipe,omitempty"`
	Slots  []CatalogImportSlot       `json:"slots,omitempty"`
}

type CatalogImportRecipeLine struct {
	Ingredient string          `json:"ingredient"`
	Quantity   decimal.Decimal `json:"quantity"`
}

type CatalogImportSlot struct {
	Name    string                `json:"name"`
	Choices []CatalogImportChoice `json:"choices"`
}

type CatalogImportChoice struct {
	Name    string                    `json:"name"`
	Default bool                      `json:"default,omitempty"`
	Items   []CatalogImportChoiceItem `json:"items,omitempty"`
}

// CatalogImportChoiceItem's quantity is decoded as a decimal so a fraction is reported as a
// problem instead of failing the whole request.
type CatalogImportChoiceItem struct {
	Ingredient string          `json:"ingredient"`
	Quantity   decimal.Decimal `json:"quantity"`
}

type CatalogImportStockCount struct {
	Product string          `json:"product"`
	Counted decimal.Decimal `json:"counted"`
}

// CatalogImportProblem's Path is a JSON pointer into the document; Code is a stable string.
type CatalogImportProblem struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type CatalogImportGeneratedSKU struct {
	Ref string `json:"ref"`
	SKU string `json:"sku"`
}

type CatalogImportSummary struct {
	Products         int                         `json:"products"`
	Ingredients      int                         `json:"ingredients"`
	Sellables        int                         `json:"sellables"`
	Plates           int                         `json:"plates"`
	Existing         int                         `json:"existing"`
	RecipeLines      int                         `json:"recipe_lines"`
	OptionSlots      int                         `json:"option_slots"`
	OptionChoices    int                         `json:"option_choices"`
	PreparationAreas int                         `json:"preparation_areas"`
	OpeningStock     int                         `json:"opening_stock"`
	GeneratedSKUs    []CatalogImportGeneratedSKU `json:"generated_skus"`
}

type CatalogImportCreatedProduct struct {
	ID  string `json:"id"`
	SKU string `json:"sku"`
}

// CatalogImportResult answers both endpoints. Products is set only after a successful apply.
type CatalogImportResult struct {
	Valid    bool                                   `json:"valid"`
	Applied  bool                                   `json:"applied"`
	Problems []CatalogImportProblem                 `json:"problems"`
	Summary  CatalogImportSummary                   `json:"summary"`
	Digest   string                                 `json:"digest"`
	Products map[string]CatalogImportCreatedProduct `json:"products,omitempty"`
}

package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

const (
	CatalogImportMaxProducts     = 1000
	CatalogImportMaxRecipeLines  = 50
	CatalogImportMaxSlots        = 20
	CatalogImportMaxChoices      = 30
	CatalogImportMaxChoiceItems  = 50
	CatalogImportMaxOpeningStock = 1000
)

const (
	ProblemDuplicateRef       = "duplicate_ref"
	ProblemUnknownRef         = "unknown_ref"
	ProblemExistingNotFound   = "existing_not_found"
	ProblemExistingAmbiguous  = "existing_ambiguous"
	ProblemNameConflict       = "name_conflict"
	ProblemSKUConflict        = "sku_conflict"
	ProblemMissingField       = "missing_field"
	ProblemInvalidField       = "invalid_field"
	ProblemInvalidUnit        = "invalid_unit"
	ProblemInvalidLinkType    = "invalid_link_type"
	ProblemInvalidQuantity    = "invalid_quantity"
	ProblemDuplicateItem      = "duplicate_item"
	ProblemInvalidSlot        = "invalid_slot"
	ProblemSlotDefaultCount   = "slot_default_count"
	ProblemStockOnExisting    = "stock_on_existing"
	ProblemLimitExceeded      = "limit_exceeded"
	ProblemDigestMismatch     = "digest_mismatch"
	catalogImportStockMaxUnit = 1_000_000_000
)

// CatalogImportProblemCodes lists every code the import can report; the API doc must name each.
var CatalogImportProblemCodes = []string{
	ProblemDuplicateRef, ProblemUnknownRef, ProblemExistingNotFound, ProblemExistingAmbiguous,
	ProblemNameConflict, ProblemSKUConflict, ProblemMissingField, ProblemInvalidField,
	ProblemInvalidUnit, ProblemInvalidLinkType, ProblemInvalidQuantity, ProblemDuplicateItem,
	ProblemInvalidSlot, ProblemSlotDefaultCount, ProblemStockOnExisting, ProblemLimitExceeded,
	ProblemDigestMismatch,
}

// CatalogImportService validates and applies catalog import documents. It writes through the
// repositories in one unit of work rather than through the per-entity services, which open
// their own transactions; design D4 lists what those services do on create.
type CatalogImportService struct {
	productRepo    ports.ProductRepository
	ingredientRepo ports.ProductIngredientRepository
	slotRepo       ports.PlateOptionSlotRepository
	stockRepo      ports.StockRepository
	unitOfWork     ports.UnitOfWork
	emitter        ports.StockMovementEmitter
}

func NewCatalogImportService(
	productRepo ports.ProductRepository,
	ingredientRepo ports.ProductIngredientRepository,
	slotRepo ports.PlateOptionSlotRepository,
	stockRepo ports.StockRepository,
	unitOfWork ports.UnitOfWork,
	emitter ports.StockMovementEmitter,
) *CatalogImportService {
	return &CatalogImportService{
		productRepo:    productRepo,
		ingredientRepo: ingredientRepo,
		slotRepo:       slotRepo,
		stockRepo:      stockRepo,
		unitOfWork:     unitOfWork,
		emitter:        emitter,
	}
}

// Validate checks the whole document against the caller's organization and writes nothing.
func (s *CatalogImportService) Validate(ctx context.Context, doc *dto.CatalogImportDocument) (*dto.CatalogImportResult, error) {
	if result := refuseOversizedDecimals(doc); result != nil {
		return result, nil
	}
	catalog, err := s.productRepo.FindAll(ctx, dto.ListProductsRequest{})
	if err != nil {
		return nil, fmt.Errorf("load catalog for import: %w", err)
	}
	plan := planCatalogImport(doc, catalog)
	digest, err := catalogImportDigest(doc)
	if err != nil {
		return nil, err
	}
	return plan.result(digest), nil
}

// Apply writes the whole document or nothing. It returns ErrCatalogImportDigestMismatch or
// ErrCatalogImportInvalid together with a result that carries the problems.
func (s *CatalogImportService) Apply(ctx context.Context, req *dto.ApplyCatalogImportRequest) (*dto.CatalogImportResult, error) {
	doc := &req.CatalogImportDocument
	if result := refuseOversizedDecimals(doc); result != nil {
		return result, domainError.ErrCatalogImportInvalid
	}
	digest, err := catalogImportDigest(doc)
	if err != nil {
		return nil, err
	}
	if req.ExpectedDigest != digest {
		return &dto.CatalogImportResult{
			Problems: []dto.CatalogImportProblem{{
				Path:    "/expected_digest",
				Code:    ProblemDigestMismatch,
				Message: "the document differs from the one validated; validate it again and confirm the new summary",
			}},
			Summary: dto.CatalogImportSummary{GeneratedSKUs: []dto.CatalogImportGeneratedSKU{}},
			Digest:  digest,
		}, domainError.ErrCatalogImportDigestMismatch
	}

	var result *dto.CatalogImportResult
	err = s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		catalog, findErr := s.productRepo.FindAll(ctx, dto.ListProductsRequest{})
		if findErr != nil {
			return fmt.Errorf("load catalog for import: %w", findErr)
		}
		plan := planCatalogImport(doc, catalog)
		result = plan.result(digest)
		if !result.Valid {
			return domainError.ErrCatalogImportInvalid
		}
		products, writeErr := s.write(ctx, plan)
		if writeErr != nil {
			return writeErr
		}
		result.Products = products
		return nil
	})
	if err != nil {
		if errors.Is(err, domainError.ErrCatalogImportInvalid) {
			return result, err
		}
		return nil, fmt.Errorf("apply catalog import: %w", err)
	}
	result.Applied = true
	return result, nil
}

func (s *CatalogImportService) write(ctx context.Context, plan *importPlan) (map[string]dto.CatalogImportCreatedProduct, error) {
	now := time.Now()

	if err := s.productRepo.CreateMany(ctx, plan.aggregates()); err != nil {
		return nil, fmt.Errorf("create products: %w", err)
	}

	recipe := make([]*dto.ProductIngredient, len(plan.recipeLines))
	for i, line := range plan.recipeLines {
		recipe[i] = &dto.ProductIngredient{
			ID:                  uuid.Must(uuid.NewV7()).String(),
			CompositeProductID:  line.plateID,
			IngredientProductID: line.ingredientID,
			DefaultQuantity:     line.quantity,
			CreatedAt:           now,
			UpdatedAt:           now,
		}
	}
	if err := s.ingredientRepo.CreateMany(ctx, recipe); err != nil {
		return nil, fmt.Errorf("create recipe lines: %w", err)
	}

	for _, plate := range plan.plateSlots {
		slots := buildSlots(plate.plateID, plate.slots, nil, now)
		if err := s.slotRepo.ReplaceForCompositeProduct(ctx, plate.plateID, slots); err != nil {
			return nil, fmt.Errorf("create option slots: %w", err)
		}
	}

	responsibilities := make([]*dto.ProductPreparationResponsibility, 0, len(plan.entries))
	for _, e := range plan.entries {
		if !e.hasPrep {
			continue
		}
		responsibilities = append(responsibilities, &dto.ProductPreparationResponsibility{
			ID:        uuid.Must(uuid.NewV7()).String(),
			ProductID: e.id,
			Area:      e.prepArea,
			Priority:  e.prepPriority,
			CreatedAt: now,
			UpdatedAt: now,
		})
	}
	if err := s.productRepo.CreatePreparationResponsibilities(ctx, responsibilities); err != nil {
		return nil, fmt.Errorf("create preparation responsibilities: %w", err)
	}

	if err := s.writeOpeningStock(ctx, plan.stockCounts, now); err != nil {
		return nil, err
	}

	products := make(map[string]dto.CatalogImportCreatedProduct, len(plan.entries))
	for _, e := range plan.entries {
		products[e.ref] = dto.CatalogImportCreatedProduct{ID: e.id, SKU: e.sku}
	}
	return products, nil
}

// writeOpeningStock records each count like any cloud count: the stock row plus a count
// movement of counted - on-hand, where on-hand is 0 for a product created by this import.
func (s *CatalogImportService) writeOpeningStock(ctx context.Context, counts []importStockCount, now time.Time) error {
	if len(counts) == 0 {
		return nil
	}
	stocks := make([]*dto.Stock, len(counts))
	for i, c := range counts {
		stocks[i] = &dto.Stock{
			ProductID:     c.entry.id,
			Version:       c.entry.version,
			Amount:        c.counted,
			UnitOfMeasure: dto.UnitOfMeasure(c.entry.unit),
			CreatedAt:     now,
			UpdatedAt:     now,
		}
	}
	if err := s.stockRepo.BulkCreateOrUpdate(ctx, stocks); err != nil {
		return fmt.Errorf("create opening stock: %w", err)
	}
	for _, c := range counts {
		movement := &dto.HistoricStock{
			ProductID:     c.entry.id,
			UnitOfMeasure: dto.UnitOfMeasure(c.entry.unit),
			CreatedAt:     now,
			Change:        c.counted,
			Kind:          dto.StockMovementKindCount,
		}
		if err := recordMovement(ctx, s.stockRepo, s.emitter, movement); err != nil {
			return err
		}
	}
	return nil
}

// catalogImportDigest hashes the document with defaults folded into each entry. Decoding into
// the typed document and re-encoding it fixes the key order, so the input's order never counts.
func catalogImportDigest(doc *dto.CatalogImportDocument) (string, error) {
	canonical, err := json.Marshal(canonicalCatalogImport(doc))
	if err != nil {
		return "", fmt.Errorf("encode catalog import document: %w", err)
	}
	sum := sha256.Sum256(canonical)
	return hex.EncodeToString(sum[:]), nil
}

func canonicalCatalogImport(doc *dto.CatalogImportDocument) dto.CatalogImportDocument {
	defaults := dto.CatalogImportDefaults{}
	if doc.Defaults != nil {
		defaults = *doc.Defaults
	}
	out := dto.CatalogImportDocument{
		Existing:     doc.Existing,
		OpeningStock: doc.OpeningStock,
	}
	for _, e := range doc.Ingredients {
		e.CatalogImportProductFields = fieldsWithDefaults(e.CatalogImportProductFields, defaults)
		if e.ProductType == "" {
			e.ProductType = string(dto.ProductTypeIngredient)
		}
		out.Ingredients = append(out.Ingredients, e)
	}
	for _, e := range doc.Sellables {
		e.CatalogImportProductFields = fieldsWithDefaults(e.CatalogImportProductFields, defaults)
		out.Sellables = append(out.Sellables, e)
	}
	for _, e := range doc.Plates {
		e.CatalogImportProductFields = fieldsWithDefaults(e.CatalogImportProductFields, defaults)
		out.Plates = append(out.Plates, e)
	}
	return out
}

func fieldsWithDefaults(f dto.CatalogImportProductFields, d dto.CatalogImportDefaults) dto.CatalogImportProductFields {
	if f.UnitOfMeasure == nil {
		f.UnitOfMeasure = d.UnitOfMeasure
	}
	if f.VAT == nil {
		f.VAT = d.VAT
	}
	if f.ICO == nil {
		f.ICO = d.ICO
	}
	if f.TaxesFormat == nil {
		f.TaxesFormat = d.TaxesFormat
	}
	prep := dto.CatalogImportPrep{}
	if f.Prep != nil {
		prep = *f.Prep
	}
	if prep.Area == nil {
		prep.Area = d.PrepArea
	}
	if prep.Priority == nil {
		prep.Priority = d.PrepPriority
	}
	if prep.Area != nil || prep.Priority != nil {
		f.Prep = &prep
	} else {
		f.Prep = nil
	}
	return f
}

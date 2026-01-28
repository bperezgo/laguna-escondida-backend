package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/purchase_entry"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	pkgPorts "laguna-escondida/backend/pkg/domain/ports"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type PurchaseEntryService struct {
	purchaseEntryRepo   ports.PurchaseEntryRepository
	supplierRepo        ports.SupplierRepository
	supplierCatalogRepo ports.SupplierCatalogRepository
	productRepo         ports.ProductRepository
	storageClient       ports.StorageClient
	eventBus            pkgPorts.EventBus
	logger              *slog.Logger
	organizationID      string
}

func NewPurchaseEntryService(
	purchaseEntryRepo ports.PurchaseEntryRepository,
	supplierRepo ports.SupplierRepository,
	supplierCatalogRepo ports.SupplierCatalogRepository,
	productRepo ports.ProductRepository,
	storageClient ports.StorageClient,
	eventBus pkgPorts.EventBus,
	logger *slog.Logger,
	organizationID string,
) *PurchaseEntryService {
	return &PurchaseEntryService{
		purchaseEntryRepo:   purchaseEntryRepo,
		supplierRepo:        supplierRepo,
		supplierCatalogRepo: supplierCatalogRepo,
		productRepo:         productRepo,
		storageClient:       storageClient,
		eventBus:            eventBus,
		logger:              logger,
		organizationID:      organizationID,
	}
}

func (s *PurchaseEntryService) CreatePurchaseEntry(ctx context.Context, req *dto.CreatePurchaseEntryRequest) (*dto.PurchaseEntry, error) {
	if _, err := s.supplierRepo.FindByID(ctx, req.SupplierID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	productIDs := make([]string, len(req.Items))
	for i, item := range req.Items {
		productIDs[i] = item.ProductID
	}

	products, err := s.productRepo.FindByIDs(ctx, productIDs)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	if len(products) != len(productIDs) {
		return nil, fmt.Errorf("%w: some products not found", domainError.ErrProductNotFound)
	}

	entry, err := purchase_entry.NewAggregateFromCreateRequest(req)
	if err != nil {
		return nil, err
	}

	if err := s.purchaseEntryRepo.Create(ctx, entry); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrPurchaseEntryCreationFailed, err)
	}

	// Update supplier catalog with new unit costs
	for _, item := range req.Items {
		s.updateSupplierCatalog(ctx, req.SupplierID, item)
	}

	purchaseEntryDTO := entry.ToDTO()

	// Publish event for stock update
	event := dto.NewPurchaseEntryCreatedEvent(purchaseEntryDTO.ID, purchaseEntryDTO.SupplierID, purchaseEntryDTO.Items)
	if err := s.eventBus.Publish(ctx, event); err != nil {
		s.logger.Error("failed to publish purchase entry created event",
			slog.String("purchase_entry_id", purchaseEntryDTO.ID),
			slog.String("error", err.Error()),
		)
	}

	return purchaseEntryDTO, nil
}

func (s *PurchaseEntryService) updateSupplierCatalog(ctx context.Context, supplierID string, item dto.CreatePurchaseEntryItemRequest) {
	existing, err := s.supplierCatalogRepo.FindBySupplierAndProduct(ctx, supplierID, item.ProductID)

	unitCost, parseErr := decimal.NewFromString(item.UnitCost)
	if parseErr != nil {
		return
	}

	if err != nil {
		// Catalog entry doesn't exist, create it
		now := time.Now()
		catalog := &dto.SupplierCatalog{
			ID:         uuid.New().String(),
			SupplierID: supplierID,
			ProductID:  item.ProductID,
			UnitCost:   unitCost,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		_ = s.supplierCatalogRepo.Create(ctx, catalog)
		return
	}

	// Update existing catalog entry with new unit cost
	existing.UnitCost = unitCost
	existing.UpdatedAt = time.Now()
	_ = s.supplierCatalogRepo.Update(ctx, existing.ID, existing)
}

func (s *PurchaseEntryService) GetPurchaseEntryByID(ctx context.Context, id string) (*dto.PurchaseEntryWithSupplier, error) {
	entry, err := s.purchaseEntryRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrPurchaseEntryNotFound, err)
	}

	return entry, nil
}

func (s *PurchaseEntryService) ListPurchaseEntries(ctx context.Context) ([]*dto.PurchaseEntryWithSupplier, error) {
	entries, err := s.purchaseEntryRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list purchase entries: %w", err)
	}

	return entries, nil
}

func (s *PurchaseEntryService) GetPurchaseEntriesBySupplier(ctx context.Context, supplierID string) ([]*dto.PurchaseEntryWithSupplier, error) {
	if _, err := s.supplierRepo.FindByID(ctx, supplierID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrSupplierNotFound, err)
	}

	entries, err := s.purchaseEntryRepo.FindBySupplierID(ctx, supplierID)
	if err != nil {
		return nil, fmt.Errorf("failed to get supplier purchase entries: %w", err)
	}

	return entries, nil
}

func (s *PurchaseEntryService) UploadPurchaseEntryDocument(ctx context.Context, purchaseEntryID string, fileData []byte, fileType string) (*string, error) {
	if _, err := s.purchaseEntryRepo.FindByID(ctx, purchaseEntryID); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrPurchaseEntryNotFound, err)
	}

	var contentType string
	var extension string

	switch fileType {
	case "pdf":
		contentType = "application/pdf"
		extension = "pdf"
	case "xml":
		contentType = "application/xml"
		extension = "xml"
	default:
		return nil, fmt.Errorf("unsupported file type: %s", fileType)
	}

	storageKey := fmt.Sprintf("%s/purchase-entries/%s.%s", s.organizationID, purchaseEntryID, extension)

	if err := s.storageClient.Upload(ctx, storageKey, fileData, contentType); err != nil {
		return nil, fmt.Errorf("failed to upload document: %w", err)
	}

	var pdfPath, xmlPath *string
	if fileType == "pdf" {
		pdfPath = &storageKey
	} else {
		xmlPath = &storageKey
	}

	if err := s.purchaseEntryRepo.UpdateStoragePaths(ctx, purchaseEntryID, pdfPath, xmlPath); err != nil {
		return nil, fmt.Errorf("failed to update storage paths: %w", err)
	}

	return &storageKey, nil
}

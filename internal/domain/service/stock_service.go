package service

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/google/uuid"
)

type StockService struct {
	stockRepo    ports.StockRepository
	productRepo  ports.ProductRepository
	unitOfWork   ports.UnitOfWork
	outboxRepo   ports.SyncOutboxRepository
	syncIdentity dto.SyncIdentity
}

func NewStockService(
	stockRepo ports.StockRepository,
	productRepo ports.ProductRepository,
	unitOfWork ports.UnitOfWork,
	outboxRepo ports.SyncOutboxRepository,
	syncIdentity dto.SyncIdentity,
) *StockService {
	return &StockService{
		stockRepo:    stockRepo,
		productRepo:  productRepo,
		unitOfWork:   unitOfWork,
		outboxRepo:   outboxRepo,
		syncIdentity: syncIdentity,
	}
}

func (s *StockService) CreateStock(ctx context.Context, req *dto.CreateStockRequest) (*dto.Stock, error) {
	product, err := s.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	existingStock, err := s.stockRepo.FindByProductID(ctx, req.ProductID)
	if err == nil && existingStock != nil {
		return nil, domainError.ErrStockAlreadyExists
	}

	now := time.Now()
	stock := &dto.Stock{
		ProductID:     req.ProductID,
		Version:       product.Version,
		Amount:        req.Amount,
		UnitOfMeasure: product.UnitOfMeasure,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// Persist the stock row, its historic record, and the sync-outbox row that replicates
	// it to the cloud in one transaction (Option A): they commit or roll back together.
	if err := s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.stockRepo.Create(ctx, stock); err != nil {
			return fmt.Errorf("%w: %w", domainError.ErrStockCreationFailed, err)
		}

		historicStock := &dto.HistoricStock{
			ProductID:     req.ProductID,
			UnitOfMeasure: product.UnitOfMeasure,
			CreatedAt:     now,
			Change:        req.Amount,
		}
		if err := createAndSyncHistoric(ctx, s.stockRepo, s.outboxRepo, s.syncIdentity.NodeID, historicStock); err != nil {
			return fmt.Errorf("failed to create historic stock record: %w", err)
		}

		return appendStockOutbox(ctx, s.outboxRepo, s.syncIdentity.NodeID, stock, dto.SyncOperationCreate)
	}); err != nil {
		return nil, err
	}

	return stock, nil
}

func (s *StockService) AddOrDecreaseStock(ctx context.Context, req *dto.AddOrDecreaseStockRequest) error {
	product, err := s.productRepo.FindByID(ctx, req.ProductID)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrProductNotFound, err)
	}

	existingStock, err := s.stockRepo.FindByProductID(ctx, req.ProductID)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrStockNotFound, err)
	}

	if existingStock.Version != product.Version {
		return domainError.ErrProductVersionMismatch
	}

	newAmount := existingStock.Amount + req.Change

	return s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.stockRepo.UpdateAmount(ctx, req.ProductID, newAmount); err != nil {
			return fmt.Errorf("%w: %w", domainError.ErrStockUpdateFailed, err)
		}

		historicStock := &dto.HistoricStock{
			ProductID:     req.ProductID,
			UnitOfMeasure: product.UnitOfMeasure,
			CreatedAt:     time.Now(),
			Change:        req.Change,
		}
		if err := createAndSyncHistoric(ctx, s.stockRepo, s.outboxRepo, s.syncIdentity.NodeID, historicStock); err != nil {
			return fmt.Errorf("failed to create historic stock record: %w", err)
		}

		updatedStock := &dto.Stock{
			ProductID:     existingStock.ProductID,
			Version:       existingStock.Version,
			Amount:        newAmount,
			UnitOfMeasure: existingStock.UnitOfMeasure,
			CreatedAt:     existingStock.CreatedAt,
			UpdatedAt:     time.Now(),
		}
		return appendStockOutbox(ctx, s.outboxRepo, s.syncIdentity.NodeID, updatedStock, dto.SyncOperationUpdate)
	})
}

func (s *StockService) DeleteStock(ctx context.Context, productID string) error {
	if _, err := s.stockRepo.FindByProductID(ctx, productID); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrStockNotFound, err)
	}

	return s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.stockRepo.Delete(ctx, productID); err != nil {
			return fmt.Errorf("%w: %w", domainError.ErrStockDeleteFailed, err)
		}
		return appendStockDeleteOutbox(ctx, s.outboxRepo, s.syncIdentity.NodeID, productID)
	})
}

func (s *StockService) GetAllStocks(ctx context.Context) ([]*dto.Stock, error) {
	stocks, err := s.stockRepo.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get all stocks: %w", err)
	}

	return stocks, nil
}

func (s *StockService) BulkStockCreationOrUpdating(ctx context.Context, req *dto.BulkStockCreationOrUpdatingRequest) error {
	if len(req.Items) == 0 {
		return fmt.Errorf("items cannot be empty")
	}

	productIDs := make([]string, len(req.Items))
	for i, item := range req.Items {
		productIDs[i] = item.ProductID
	}

	products, err := s.productRepo.FindByIDs(ctx, productIDs)
	if err != nil {
		return fmt.Errorf("failed to fetch products: %w", err)
	}

	if len(products) != len(req.Items) {
		return fmt.Errorf("%w: some products not found", domainError.ErrProductNotFound)
	}

	productMap := make(map[string]*dto.Product)
	for _, product := range products {
		productMap[product.ID] = product
	}

	existingStocks, err := s.stockRepo.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to fetch existing stocks: %w", err)
	}

	existingStockMap := make(map[string]*dto.Stock)
	for _, stock := range existingStocks {
		existingStockMap[stock.ProductID] = stock
	}

	stocksToCreateOrUpdate := make([]*dto.Stock, 0, len(req.Items))
	operations := make([]dto.SyncOperation, 0, len(req.Items))
	historicRecords := make([]*dto.HistoricStock, 0, len(req.Items))
	now := time.Now()

	for _, item := range req.Items {
		product := productMap[item.ProductID]

		existingStock, exists := existingStockMap[item.ProductID]
		if !exists {
			stock := &dto.Stock{
				ProductID:     item.ProductID,
				Version:       product.Version,
				Amount:        item.Amount,
				UnitOfMeasure: product.UnitOfMeasure,
				CreatedAt:     now,
				UpdatedAt:     now,
			}
			stocksToCreateOrUpdate = append(stocksToCreateOrUpdate, stock)
			operations = append(operations, dto.SyncOperationCreate)

			historicStock := &dto.HistoricStock{
				ProductID:     item.ProductID,
				UnitOfMeasure: product.UnitOfMeasure,
				CreatedAt:     now,
				Change:        item.Amount,
			}
			historicRecords = append(historicRecords, historicStock)
		} else {
			if existingStock.Version != product.Version {
				return fmt.Errorf("%w: product %s version mismatch", domainError.ErrProductVersionMismatch, item.ProductID)
			}

			change := item.Amount - existingStock.Amount

			stock := &dto.Stock{
				ProductID:     item.ProductID,
				Version:       product.Version,
				Amount:        item.Amount,
				UnitOfMeasure: product.UnitOfMeasure,
				CreatedAt:     existingStock.CreatedAt,
				UpdatedAt:     now,
			}
			stocksToCreateOrUpdate = append(stocksToCreateOrUpdate, stock)
			operations = append(operations, dto.SyncOperationUpdate)

			if change != 0 {
				historicStock := &dto.HistoricStock{
					ProductID:     item.ProductID,
					UnitOfMeasure: product.UnitOfMeasure,
					CreatedAt:     now,
					Change:        change,
				}
				historicRecords = append(historicRecords, historicStock)
			}
		}
	}

	// Bulk write, its historic records, and one outbox op per changed product all commit
	// in a single transaction (Option A), so the cloud mirror converges to exactly what
	// the edge persisted.
	return s.unitOfWork.Do(ctx, func(ctx context.Context) error {
		if err := s.stockRepo.BulkCreateOrUpdate(ctx, stocksToCreateOrUpdate); err != nil {
			return fmt.Errorf("failed to bulk create or update stocks: %w", err)
		}

		for _, historicStock := range historicRecords {
			if err := createAndSyncHistoric(ctx, s.stockRepo, s.outboxRepo, s.syncIdentity.NodeID, historicStock); err != nil {
				return fmt.Errorf("failed to create historic stock record: %w", err)
			}
		}

		for i, stock := range stocksToCreateOrUpdate {
			if err := appendStockOutbox(ctx, s.outboxRepo, s.syncIdentity.NodeID, stock, operations[i]); err != nil {
				return err
			}
		}

		return nil
	})
}

// appendStockOutbox writes one create/update sync_outbox row carrying the current stock
// snapshot. It must be called inside a UnitOfWork transaction (Option A). Shared by the
// manual stock writes (StockService) and the order/purchase-driven writes (StockEventHandler).
func appendStockOutbox(ctx context.Context, outboxRepo ports.SyncOutboxRepository, nodeID string, stock *dto.Stock, operation dto.SyncOperation) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate stock outbox op_id: %w", err)
	}

	payload := dto.StockSyncPayload{
		ProductID:     stock.ProductID,
		Version:       stock.Version,
		Amount:        stock.Amount,
		UnitOfMeasure: string(stock.UnitOfMeasure),
		CreatedAt:     stock.CreatedAt,
		UpdatedAt:     stock.UpdatedAt,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal stock sync payload: %w", err)
	}

	return outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: nodeID,
		EntityType:   dto.SyncEntityStock,
		EntityID:     stock.ProductID,
		Operation:    operation,
		Payload:      payloadBytes,
	})
}

// createAndSyncHistoric persists one historic_stock movement row and appends its append-only
// create sync_outbox op in the same transaction (Option A), so the ledger entry replicates to
// the cloud. It generates the row's op_id, stores it on the row, and reuses it as the sync op
// id (1:1) — the cloud dedups on it. Must be called inside a UnitOfWork transaction. Shared by
// the manual stock writes (StockService) and the order/purchase-driven writes (StockEventHandler).
func createAndSyncHistoric(ctx context.Context, stockRepo ports.StockRepository, outboxRepo ports.SyncOutboxRepository, nodeID string, historic *dto.HistoricStock) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate historic_stock op_id: %w", err)
	}
	historic.OpID = opID.String()

	if createErr := stockRepo.CreateHistoricRecord(ctx, historic); createErr != nil {
		return createErr
	}

	payload := dto.HistoricStockSyncPayload{
		OpID:          historic.OpID,
		ProductID:     historic.ProductID,
		UnitOfMeasure: string(historic.UnitOfMeasure),
		Change:        historic.Change,
		CreatedAt:     historic.CreatedAt,
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal historic_stock sync payload: %w", err)
	}

	return outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         historic.OpID,
		OriginNodeID: nodeID,
		EntityType:   dto.SyncEntityHistoricStock,
		EntityID:     historic.OpID,
		Operation:    dto.SyncOperationCreate,
		Payload:      payloadBytes,
	})
}

// appendStockDeleteOutbox writes a delete (tombstone) sync_outbox row keyed by product_id.
// It must be called inside a UnitOfWork transaction (Option A).
func appendStockDeleteOutbox(ctx context.Context, outboxRepo ports.SyncOutboxRepository, nodeID, productID string) error {
	opID, err := uuid.NewV7()
	if err != nil {
		return fmt.Errorf("generate stock outbox op_id: %w", err)
	}

	payloadBytes, err := json.Marshal(dto.SyncTombstone{ID: productID})
	if err != nil {
		return fmt.Errorf("marshal stock tombstone: %w", err)
	}

	return outboxRepo.Append(ctx, &dto.SyncOutboxEntry{
		OpID:         opID.String(),
		OriginNodeID: nodeID,
		EntityType:   dto.SyncEntityStock,
		EntityID:     productID,
		Operation:    dto.SyncOperationDelete,
		Payload:      payloadBytes,
	})
}

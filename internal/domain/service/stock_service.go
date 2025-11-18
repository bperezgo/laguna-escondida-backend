package service

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
)

type StockService struct {
	stockRepo   ports.StockRepository
	productRepo ports.ProductRepository
}

func NewStockService(stockRepo ports.StockRepository, productRepo ports.ProductRepository) *StockService {
	return &StockService{
		stockRepo:   stockRepo,
		productRepo: productRepo,
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
		ProductID: req.ProductID,
		Version:   product.Version,
		Amount:    req.Amount,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if err := s.stockRepo.Create(ctx, stock); err != nil {
		return nil, fmt.Errorf("%w: %w", domainError.ErrStockCreationFailed, err)
	}

	historicStock := &dto.HistoricStock{
		ProductID: req.ProductID,
		CreatedAt: now,
		Change:    req.Amount,
	}

	if err := s.stockRepo.CreateHistoricRecord(ctx, historicStock); err != nil {
		return nil, fmt.Errorf("failed to create historic stock record: %w", err)
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

	if err := s.stockRepo.UpdateAmount(ctx, req.ProductID, newAmount); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrStockUpdateFailed, err)
	}

	historicStock := &dto.HistoricStock{
		ProductID: req.ProductID,
		CreatedAt: time.Now(),
		Change:    req.Change,
	}

	if err := s.stockRepo.CreateHistoricRecord(ctx, historicStock); err != nil {
		return fmt.Errorf("failed to create historic stock record: %w", err)
	}

	return nil
}

func (s *StockService) DeleteStock(ctx context.Context, productID string) error {
	existingStock, err := s.stockRepo.FindByProductID(ctx, productID)
	if err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrStockNotFound, err)
	}

	if err := s.stockRepo.Delete(ctx, productID); err != nil {
		return fmt.Errorf("%w: %w", domainError.ErrStockDeleteFailed, err)
	}

	_ = existingStock
	return nil
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
	historicRecords := make([]*dto.HistoricStock, 0, len(req.Items))
	now := time.Now()

	for _, item := range req.Items {
		product := productMap[item.ProductID]

		existingStock, exists := existingStockMap[item.ProductID]
		if !exists {
			stock := &dto.Stock{
				ProductID: item.ProductID,
				Version:   product.Version,
				Amount:    item.Amount,
				CreatedAt: now,
				UpdatedAt: now,
			}
			stocksToCreateOrUpdate = append(stocksToCreateOrUpdate, stock)

			historicStock := &dto.HistoricStock{
				ProductID: item.ProductID,
				CreatedAt: now,
				Change:    item.Amount,
			}
			historicRecords = append(historicRecords, historicStock)
		} else {
			if existingStock.Version != product.Version {
				return fmt.Errorf("%w: product %s version mismatch", domainError.ErrProductVersionMismatch, item.ProductID)
			}

			change := item.Amount - existingStock.Amount

			stock := &dto.Stock{
				ProductID: item.ProductID,
				Version:   product.Version,
				Amount:    item.Amount,
				CreatedAt: existingStock.CreatedAt,
				UpdatedAt: now,
			}
			stocksToCreateOrUpdate = append(stocksToCreateOrUpdate, stock)

			if change != 0 {
				historicStock := &dto.HistoricStock{
					ProductID: item.ProductID,
					CreatedAt: now,
					Change:    change,
				}
				historicRecords = append(historicRecords, historicStock)
			}
		}
	}

	if err := s.stockRepo.BulkCreateOrUpdate(ctx, stocksToCreateOrUpdate); err != nil {
		return fmt.Errorf("failed to bulk create or update stocks: %w", err)
	}

	for _, historicStock := range historicRecords {
		if err := s.stockRepo.CreateHistoricRecord(ctx, historicStock); err != nil {
			return fmt.Errorf("failed to create historic stock record: %w", err)
		}
	}

	return nil
}

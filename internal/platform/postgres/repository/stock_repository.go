package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type StockRepository struct {
	db *gorm.DB
}

func NewStockRepository(db *gorm.DB) ports.StockRepository {
	return &StockRepository{db: db}
}

type stockModel struct {
	ProductID     string     `gorm:"type:uuid;primaryKey"`
	Version       int        `gorm:"type:integer;primaryKey"`
	Amount        int        `gorm:"type:integer;not null"`
	UnitOfMeasure string     `gorm:"type:varchar(10);not null;default:'unit'"`
	CreatedAt     time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt     time.Time  `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	DeletedAt     *time.Time `gorm:"type:timestamp"`
}

func (stockModel) TableName() string {
	return "stock"
}

type historicStockModel struct {
	ID            int       `gorm:"type:serial;primaryKey"`
	OpID          *string   `gorm:"type:uuid;column:op_id"`
	ProductID     string    `gorm:"type:uuid;not null"`
	UnitOfMeasure string    `gorm:"type:varchar(10);not null;default:'unit'"`
	CreatedAt     time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	Change        int       `gorm:"type:integer;not null"`
}

func (historicStockModel) TableName() string {
	return "historic_stock"
}

func (r *StockRepository) Create(ctx context.Context, stock *dto.Stock) error {
	model := &stockModel{
		ProductID:     stock.ProductID,
		Version:       stock.Version,
		Amount:        stock.Amount,
		UnitOfMeasure: string(stock.UnitOfMeasure),
		CreatedAt:     stock.CreatedAt,
		UpdatedAt:     stock.UpdatedAt,
	}

	return postgres.GetTxOrDB(ctx, r.db).Create(model).Error
}

func (r *StockRepository) FindByProductID(ctx context.Context, productID string) (*dto.Stock, error) {
	var model stockModel
	if err := postgres.GetTxOrDB(ctx, r.db).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		First(&model).Error; err != nil {
		return nil, err
	}

	return r.toDTO(&model), nil
}

func (r *StockRepository) FindAll(ctx context.Context) ([]*dto.Stock, error) {
	var models []stockModel
	if err := postgres.GetTxOrDB(ctx, r.db).
		Where("deleted_at IS NULL").
		Find(&models).Error; err != nil {
		return nil, err
	}

	stocks := make([]*dto.Stock, len(models))
	for i, model := range models {
		stocks[i] = r.toDTO(&model)
	}

	return stocks, nil
}

func (r *StockRepository) UpdateAmount(ctx context.Context, productID string, amount int) error {
	now := time.Now()
	return postgres.GetTxOrDB(ctx, r.db).
		Model(&stockModel{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Updates(map[string]any{
			"amount":     amount,
			"updated_at": now,
		}).Error
}

func (r *StockRepository) Delete(ctx context.Context, productID string) error {
	now := time.Now()
	return postgres.GetTxOrDB(ctx, r.db).
		Model(&stockModel{}).
		Where("product_id = ? AND deleted_at IS NULL", productID).
		Updates(map[string]any{
			"deleted_at": &now,
			"updated_at": now,
		}).Error
}

// BulkCreateOrUpdate creates or updates each stock row, joining the caller's transaction
// via GetTxOrDB. The stock service always calls it inside a UnitOfWork (Option A), so the
// bulk write and the outbox rows that replicate it commit atomically.
func (r *StockRepository) BulkCreateOrUpdate(ctx context.Context, stocks []*dto.Stock) error {
	if len(stocks) == 0 {
		return nil
	}

	db := postgres.GetTxOrDB(ctx, r.db)
	for _, stock := range stocks {
		model := &stockModel{
			ProductID:     stock.ProductID,
			Version:       stock.Version,
			Amount:        stock.Amount,
			UnitOfMeasure: string(stock.UnitOfMeasure),
			CreatedAt:     stock.CreatedAt,
			UpdatedAt:     stock.UpdatedAt,
		}

		var existing stockModel
		err := db.Where("product_id = ? AND version = ? AND deleted_at IS NULL", stock.ProductID, stock.Version).First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			if err = db.Create(model).Error; err != nil {
				return err
			}
		} else if err != nil {
			return err
		} else {
			if err := db.Model(&stockModel{}).
				Where("product_id = ? AND version = ? AND deleted_at IS NULL", stock.ProductID, stock.Version).
				Updates(map[string]any{
					"amount":     stock.Amount,
					"updated_at": stock.UpdatedAt,
				}).Error; err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *StockRepository) CreateHistoricRecord(ctx context.Context, historicStock *dto.HistoricStock) error {
	model := &historicStockModel{
		ProductID:     historicStock.ProductID,
		UnitOfMeasure: string(historicStock.UnitOfMeasure),
		CreatedAt:     historicStock.CreatedAt,
		Change:        historicStock.Change,
	}
	if historicStock.OpID != "" {
		model.OpID = &historicStock.OpID
	}

	return postgres.GetTxOrDB(ctx, r.db).Create(model).Error
}

func (r *StockRepository) FindHistoricByProductID(ctx context.Context, productID string) ([]*dto.HistoricStock, error) {
	var models []historicStockModel
	if err := postgres.GetTxOrDB(ctx, r.db).
		Where("product_id = ?", productID).
		Order("created_at DESC").
		Find(&models).Error; err != nil {
		return nil, err
	}

	historicStocks := make([]*dto.HistoricStock, len(models))
	for i, model := range models {
		historicStocks[i] = r.historicToDTO(&model)
	}

	return historicStocks, nil
}

func (r *StockRepository) toDTO(model *stockModel) *dto.Stock {
	return &dto.Stock{
		ProductID:     model.ProductID,
		Version:       model.Version,
		Amount:        model.Amount,
		UnitOfMeasure: dto.UnitOfMeasure(model.UnitOfMeasure),
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}
}

func (r *StockRepository) historicToDTO(model *historicStockModel) *dto.HistoricStock {
	opID := ""
	if model.OpID != nil {
		opID = *model.OpID
	}
	return &dto.HistoricStock{
		ID:            model.ID,
		OpID:          opID,
		ProductID:     model.ProductID,
		UnitOfMeasure: dto.UnitOfMeasure(model.UnitOfMeasure),
		CreatedAt:     model.CreatedAt,
		Change:        model.Change,
	}
}

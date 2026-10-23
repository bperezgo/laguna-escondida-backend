package repository

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type DeliveryZoneRepository struct {
	db *gorm.DB
}

func NewDeliveryZoneRepository(db *gorm.DB) ports.DeliveryZoneRepository {
	return &DeliveryZoneRepository{db: db}
}

type deliveryZoneModel struct {
	postgres.TenantOwned
	ID        string          `gorm:"type:uuid;primaryKey"`
	Name      string          `gorm:"type:varchar(100);not null"`
	Boundary  json.RawMessage `gorm:"type:jsonb;not null"`
	Active    bool            `gorm:"type:boolean;not null;default:true"`
	CreatedAt time.Time       `gorm:"type:timestamp;not null"`
	UpdatedAt time.Time       `gorm:"type:timestamp;not null"`
	DeletedAt *time.Time      `gorm:"type:timestamp"`
}

func (deliveryZoneModel) TableName() string {
	return "delivery_zones"
}

func (r *DeliveryZoneRepository) Create(ctx context.Context, zone *dto.DeliveryZone) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Create(&deliveryZoneModel{
		ID:        zone.ID,
		Name:      zone.Name,
		Boundary:  zone.Boundary,
		Active:    zone.Active,
		CreatedAt: zone.CreatedAt,
		UpdatedAt: zone.UpdatedAt,
	}).Error
}

func (r *DeliveryZoneRepository) Update(ctx context.Context, zone *dto.DeliveryZone) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.
		Model(&deliveryZoneModel{}).
		Where("id = ? AND deleted_at IS NULL", zone.ID).
		Updates(map[string]any{
			"name":       zone.Name,
			"boundary":   zone.Boundary,
			"active":     zone.Active,
			"updated_at": zone.UpdatedAt,
		}).Error
}

func (r *DeliveryZoneRepository) Delete(ctx context.Context, id string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	now := time.Now()
	return db.
		Model(&deliveryZoneModel{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error
}

func (r *DeliveryZoneRepository) FindByID(ctx context.Context, id string) (*dto.DeliveryZone, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model deliveryZoneModel
	err = db.Where("id = ? AND deleted_at IS NULL", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrDeliveryZoneNotFound
	}
	if err != nil {
		return nil, err
	}
	return model.toDTO(), nil
}

func (r *DeliveryZoneRepository) FindAll(ctx context.Context) ([]*dto.DeliveryZone, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	return r.find(ctx, db.Where("deleted_at IS NULL"))
}

func (r *DeliveryZoneRepository) ListActive(ctx context.Context) ([]*dto.DeliveryZone, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	return r.find(ctx, db.Where("deleted_at IS NULL AND active = true"))
}

func (r *DeliveryZoneRepository) find(_ context.Context, query *gorm.DB) ([]*dto.DeliveryZone, error) {
	var models []deliveryZoneModel
	if err := query.Order("created_at ASC, id ASC").Find(&models).Error; err != nil {
		return nil, err
	}
	zones := make([]*dto.DeliveryZone, len(models))
	for i := range models {
		zones[i] = models[i].toDTO()
	}
	return zones, nil
}

func (m *deliveryZoneModel) toDTO() *dto.DeliveryZone {
	return &dto.DeliveryZone{
		ID:        m.ID,
		Name:      m.Name,
		Boundary:  m.Boundary,
		Active:    m.Active,
		CreatedAt: m.CreatedAt,
		UpdatedAt: m.UpdatedAt,
	}
}

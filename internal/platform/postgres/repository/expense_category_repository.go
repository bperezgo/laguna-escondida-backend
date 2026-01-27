package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"gorm.io/gorm"
)

type ExpenseCategoryRepository struct {
	db *gorm.DB
}

func NewExpenseCategoryRepository(db *gorm.DB) ports.ExpenseCategoryRepository {
	return &ExpenseCategoryRepository{db: db}
}

type expenseCategoryModel struct {
	ID          string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Code        string    `gorm:"type:varchar(50);not null;uniqueIndex"`
	Name        string    `gorm:"type:varchar(255);not null"`
	Description *string   `gorm:"type:text"`
	IsActive    bool      `gorm:"type:boolean;not null;default:true;column:is_active"`
	CreatedAt   time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP;column:created_at"`
}

func (expenseCategoryModel) TableName() string {
	return "expense_categories"
}

func (r *ExpenseCategoryRepository) Create(ctx context.Context, category *dto.ExpenseCategory) error {
	model := &expenseCategoryModel{
		ID:          category.ID,
		Code:        category.Code,
		Name:        category.Name,
		Description: category.Description,
		IsActive:    category.IsActive,
		CreatedAt:   category.CreatedAt,
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *ExpenseCategoryRepository) Update(ctx context.Context, id string, category *dto.ExpenseCategory) error {
	return r.db.WithContext(ctx).
		Model(&expenseCategoryModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"code":        category.Code,
			"name":        category.Name,
			"description": category.Description,
			"is_active":   category.IsActive,
		}).Error
}

func (r *ExpenseCategoryRepository) FindAll(ctx context.Context) ([]*dto.ExpenseCategory, error) {
	var models []expenseCategoryModel

	err := r.db.WithContext(ctx).
		Order("name ASC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	return r.modelsToDTOs(models), nil
}

func (r *ExpenseCategoryRepository) FindByID(ctx context.Context, id string) (*dto.ExpenseCategory, error) {
	var model expenseCategoryModel

	err := r.db.WithContext(ctx).
		Where("id = ?", id).
		First(&model).Error

	if err != nil {
		return nil, err
	}

	return r.modelToDTO(&model), nil
}

func (r *ExpenseCategoryRepository) FindByCode(ctx context.Context, code string) (*dto.ExpenseCategory, error) {
	var model expenseCategoryModel

	err := r.db.WithContext(ctx).
		Where("code = ?", code).
		First(&model).Error

	if err != nil {
		return nil, err
	}

	return r.modelToDTO(&model), nil
}

func (r *ExpenseCategoryRepository) FindAllActive(ctx context.Context) ([]*dto.ExpenseCategory, error) {
	var models []expenseCategoryModel

	err := r.db.WithContext(ctx).
		Where("is_active = ?", true).
		Order("name ASC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	return r.modelsToDTOs(models), nil
}

func (r *ExpenseCategoryRepository) modelToDTO(model *expenseCategoryModel) *dto.ExpenseCategory {
	return &dto.ExpenseCategory{
		ID:          model.ID,
		Code:        model.Code,
		Name:        model.Name,
		Description: model.Description,
		IsActive:    model.IsActive,
		CreatedAt:   model.CreatedAt,
	}
}

func (r *ExpenseCategoryRepository) modelsToDTOs(models []expenseCategoryModel) []*dto.ExpenseCategory {
	result := make([]*dto.ExpenseCategory, len(models))
	for i, model := range models {
		result[i] = r.modelToDTO(&model)
	}
	return result
}

package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/expense"
	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"

	"github.com/shopspring/decimal"
	"gorm.io/gorm"
)

type ExpenseRepository struct {
	db *gorm.DB
}

func NewExpenseRepository(db *gorm.DB) ports.ExpenseRepository {
	return &ExpenseRepository{db: db}
}

type expenseModel struct {
	ID             string          `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CategoryID     string          `gorm:"type:uuid;not null;column:category_id"`
	SupplierID     *string         `gorm:"type:uuid;column:supplier_id"`
	Amount         decimal.Decimal `gorm:"type:numeric(19,4);not null"`
	Description    string          `gorm:"type:varchar(500);not null"`
	ExpenseDate    time.Time       `gorm:"type:timestamp;not null;column:expense_date"`
	Reference      *string         `gorm:"type:varchar(255)"`
	Notes          *string         `gorm:"type:text"`
	PDFStoragePath *string         `gorm:"type:text;column:pdf_storage_path"`
	XMLStoragePath *string         `gorm:"type:text;column:xml_storage_path"`
	CreatedAt      time.Time       `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP;column:created_at"`
}

func (expenseModel) TableName() string {
	return "expenses"
}

type expenseWithCategoryModel struct {
	ID             string          `gorm:"column:id"`
	CategoryID     string          `gorm:"column:category_id"`
	CategoryCode   string          `gorm:"column:category_code"`
	CategoryName   string          `gorm:"column:category_name"`
	SupplierID     *string         `gorm:"column:supplier_id"`
	SupplierName   *string         `gorm:"column:supplier_name"`
	Amount         decimal.Decimal `gorm:"column:amount"`
	Description    string          `gorm:"column:description"`
	ExpenseDate    time.Time       `gorm:"column:expense_date"`
	Reference      *string         `gorm:"column:reference"`
	Notes          *string         `gorm:"column:notes"`
	PDFStoragePath *string         `gorm:"column:pdf_storage_path"`
	XMLStoragePath *string         `gorm:"column:xml_storage_path"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
}

func (r *ExpenseRepository) Create(ctx context.Context, exp *expense.Aggregate) error {
	model := &expenseModel{
		ID:          exp.ID(),
		CategoryID:  exp.CategoryID(),
		SupplierID:  exp.SupplierID(),
		Amount:      exp.Amount(),
		Description: exp.Description(),
		ExpenseDate: exp.ExpenseDate(),
		Reference:   exp.Reference(),
		Notes:       exp.Notes(),
		CreatedAt:   exp.CreatedAt(),
	}

	return r.db.WithContext(ctx).Create(model).Error
}

func (r *ExpenseRepository) Update(ctx context.Context, id string, exp *expense.Aggregate) error {
	return r.db.WithContext(ctx).
		Model(&expenseModel{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			"category_id":  exp.CategoryID(),
			"supplier_id":  exp.SupplierID(),
			"amount":       exp.Amount(),
			"description":  exp.Description(),
			"expense_date": exp.ExpenseDate(),
			"reference":    exp.Reference(),
			"notes":        exp.Notes(),
		}).Error
}

func (r *ExpenseRepository) Delete(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).
		Where("id = ?", id).
		Delete(&expenseModel{}).Error
}

func (r *ExpenseRepository) FindByID(ctx context.Context, id string) (*dto.ExpenseWithCategory, error) {
	var model expenseWithCategoryModel

	err := r.db.WithContext(ctx).
		Table("expenses e").
		Select(`e.id, e.category_id, ec.code as category_code, ec.name as category_name, 
			e.supplier_id, s.name as supplier_name, e.amount, e.description, 
			e.expense_date, e.reference, e.notes, e.pdf_storage_path, e.xml_storage_path, e.created_at`).
		Joins("JOIN expense_categories ec ON ec.id = e.category_id").
		Joins("LEFT JOIN suppliers s ON s.id = e.supplier_id AND s.deleted_at IS NULL").
		Where("e.id = ?", id).
		First(&model).Error

	if err != nil {
		return nil, err
	}

	return r.modelToDTO(&model), nil
}

func (r *ExpenseRepository) FindAll(ctx context.Context) ([]*dto.ExpenseWithCategory, error) {
	var models []expenseWithCategoryModel

	err := r.db.WithContext(ctx).
		Table("expenses e").
		Select(`e.id, e.category_id, ec.code as category_code, ec.name as category_name, 
			e.supplier_id, s.name as supplier_name, e.amount, e.description, 
			e.expense_date, e.reference, e.notes, e.pdf_storage_path, e.xml_storage_path, e.created_at`).
		Joins("JOIN expense_categories ec ON ec.id = e.category_id").
		Joins("LEFT JOIN suppliers s ON s.id = e.supplier_id AND s.deleted_at IS NULL").
		Order("e.expense_date DESC").
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	return r.modelsToDTOs(models), nil
}

func (r *ExpenseRepository) FindByCriteria(ctx context.Context, criteria *dto.ExpenseListCriteria) ([]*dto.ExpenseWithCategory, error) {
	var models []expenseWithCategoryModel

	query := r.db.WithContext(ctx).
		Table("expenses e").
		Select(`e.id, e.category_id, ec.code as category_code, ec.name as category_name, 
			e.supplier_id, s.name as supplier_name, e.amount, e.description, 
			e.expense_date, e.reference, e.notes, e.pdf_storage_path, e.xml_storage_path, e.created_at`).
		Joins("JOIN expense_categories ec ON ec.id = e.category_id").
		Joins("LEFT JOIN suppliers s ON s.id = e.supplier_id AND s.deleted_at IS NULL")

	if criteria.CategoryID != nil {
		query = query.Where("e.category_id = ?", *criteria.CategoryID)
	}

	if criteria.SupplierID != nil {
		query = query.Where("e.supplier_id = ?", *criteria.SupplierID)
	}

	if criteria.StartDate != nil {
		query = query.Where("e.expense_date >= ?", *criteria.StartDate)
	}

	if criteria.EndDate != nil {
		query = query.Where("e.expense_date <= ?", *criteria.EndDate)
	}

	err := query.Order("e.expense_date DESC").Find(&models).Error

	if err != nil {
		return nil, err
	}

	return r.modelsToDTOs(models), nil
}

func (r *ExpenseRepository) UpdateStoragePaths(ctx context.Context, id string, pdfPath *string, xmlPath *string) error {
	updates := make(map[string]interface{})

	if pdfPath != nil {
		updates["pdf_storage_path"] = *pdfPath
	}

	if xmlPath != nil {
		updates["xml_storage_path"] = *xmlPath
	}

	if len(updates) == 0 {
		return nil
	}

	return r.db.WithContext(ctx).
		Model(&expenseModel{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *ExpenseRepository) modelToDTO(model *expenseWithCategoryModel) *dto.ExpenseWithCategory {
	return &dto.ExpenseWithCategory{
		ID:             model.ID,
		CategoryID:     model.CategoryID,
		CategoryCode:   model.CategoryCode,
		CategoryName:   model.CategoryName,
		SupplierID:     model.SupplierID,
		SupplierName:   model.SupplierName,
		Amount:         model.Amount,
		Description:    model.Description,
		ExpenseDate:    model.ExpenseDate,
		Reference:      model.Reference,
		Notes:          model.Notes,
		PDFStoragePath: model.PDFStoragePath,
		XMLStoragePath: model.XMLStoragePath,
		CreatedAt:      model.CreatedAt,
	}
}

func (r *ExpenseRepository) modelsToDTOs(models []expenseWithCategoryModel) []*dto.ExpenseWithCategory {
	result := make([]*dto.ExpenseWithCategory, len(models))
	for i, model := range models {
		result[i] = r.modelToDTO(&model)
	}
	return result
}

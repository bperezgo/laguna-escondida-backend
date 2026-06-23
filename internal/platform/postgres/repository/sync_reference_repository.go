package repository

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SyncReferenceRepository is both sides of reference-data pull: the cloud reads changed
// rows (FindChanged*) and the edge upserts them (Upsert*). It reuses the per-entity GORM
// models, whose DeletedAt is a plain nullable column (not gorm.DeletedAt), so queries
// see soft-deleted rows and the explicit deleted_at filter controls visibility.
type SyncReferenceRepository struct {
	db *gorm.DB
}

func NewSyncReferenceRepository(db *gorm.DB) *SyncReferenceRepository {
	return &SyncReferenceRepository{db: db}
}

// changedFilter matches rows whose update or soft-delete happened strictly after since.
func changedFilter(db *gorm.DB, since time.Time) *gorm.DB {
	return db.Where("updated_at > ? OR (deleted_at IS NOT NULL AND deleted_at > ?)", since, since).
		Order("updated_at ASC")
}

func (r *SyncReferenceRepository) FindChangedProducts(ctx context.Context, since time.Time) ([]dto.ProductSyncPayload, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []productModel
	if err := changedFilter(db, since).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query changed products: %w", err)
	}

	out := make([]dto.ProductSyncPayload, len(models))
	for i, m := range models {
		out[i] = dto.ProductSyncPayload{
			ID:                  m.ID,
			Name:                m.Name,
			Category:            m.Category,
			ProductType:         m.ProductType,
			UnitOfMeasure:       m.UnitOfMeasure,
			Version:             m.Version,
			UnitPrice:           m.UnitPrice,
			VAT:                 m.VAT,
			VATAmount:           m.VATAmount,
			ICO:                 m.ICO,
			ICOAmount:           m.ICOAmount,
			Description:         m.Description,
			SKU:                 m.SKU,
			TotalPriceWithTaxes: m.TotalPriceWithTaxes,
			CreatedAt:           m.CreatedAt,
			UpdatedAt:           m.UpdatedAt,
			DeletedAt:           m.DeletedAt,
		}
	}
	return out, nil
}

func (r *SyncReferenceRepository) FindChangedUsers(ctx context.Context, since time.Time) ([]dto.UserSyncPayload, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []userModel
	if err := changedFilter(db, since).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query changed users: %w", err)
	}

	userIDs := make([]string, len(models))
	for i, m := range models {
		userIDs[i] = m.ID
	}
	rolesByUser, err := r.roleIDsByUser(db, userIDs)
	if err != nil {
		return nil, err
	}

	out := make([]dto.UserSyncPayload, len(models))
	for i, m := range models {
		out[i] = dto.UserSyncPayload{
			ID:        m.ID,
			Username:  m.Username,
			Name:      m.Name,
			Password:  m.Password,
			RoleIDs:   rolesByUser[m.ID],
			CreatedAt: m.CreatedAt,
			UpdatedAt: m.UpdatedAt,
			DeletedAt: m.DeletedAt,
		}
	}
	return out, nil
}

// roleIDsByUser fetches the role assignments for the given users in one query and groups
// them by user id, so FindChangedUsers can attach each user's roles without an N+1.
func (r *SyncReferenceRepository) roleIDsByUser(db *gorm.DB, userIDs []string) (map[string][]int, error) {
	byUser := make(map[string][]int)
	if len(userIDs) == 0 {
		return byUser, nil
	}

	var rows []userRoleModel
	if err := db.Where("user_id IN ?", userIDs).
		Order("user_id, role_id").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("query user roles: %w", err)
	}
	for _, row := range rows {
		byUser[row.UserID] = append(byUser[row.UserID], row.RoleID)
	}
	return byUser, nil
}

func (r *SyncReferenceRepository) FindChangedSuppliers(ctx context.Context, since time.Time) ([]dto.SupplierSyncPayload, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []supplierModel
	if err := changedFilter(db, since).Find(&models).Error; err != nil {
		return nil, fmt.Errorf("query changed suppliers: %w", err)
	}

	out := make([]dto.SupplierSyncPayload, len(models))
	for i, m := range models {
		out[i] = dto.SupplierSyncPayload{
			ID:                   m.ID,
			Name:                 m.Name,
			IdentificationType:   m.IdentificationType,
			IdentificationNumber: m.IdentificationNumber,
			ContactName:          m.ContactName,
			Phone:                m.Phone,
			Email:                m.Email,
			Notes:                m.Notes,
			CreatedAt:            m.CreatedAt,
			UpdatedAt:            m.UpdatedAt,
			DeletedAt:            m.DeletedAt,
		}
	}
	return out, nil
}

func (r *SyncReferenceRepository) UpsertProducts(ctx context.Context, products []dto.ProductSyncPayload) error {
	if len(products) == 0 {
		return nil
	}
	db := postgres.GetTxOrDB(ctx, r.db)

	models := make([]productModel, len(products))
	for i, p := range products {
		models[i] = productModel{
			ID:                  p.ID,
			Name:                p.Name,
			Category:            p.Category,
			ProductType:         p.ProductType,
			UnitOfMeasure:       p.UnitOfMeasure,
			Version:             p.Version,
			UnitPrice:           p.UnitPrice,
			VAT:                 p.VAT,
			VATAmount:           p.VATAmount,
			ICO:                 p.ICO,
			ICOAmount:           p.ICOAmount,
			Description:         p.Description,
			SKU:                 p.SKU,
			TotalPriceWithTaxes: p.TotalPriceWithTaxes,
			CreatedAt:           p.CreatedAt,
			UpdatedAt:           p.UpdatedAt,
			DeletedAt:           p.DeletedAt,
		}
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "category", "product_type", "unit_of_measure", "version", "unit_price",
			"vat", "vat_amount", "ico", "ico_amount", "description", "sku",
			"total_price_with_taxes", "updated_at", "deleted_at",
		}),
	}).Create(&models).Error; err != nil {
		return fmt.Errorf("upsert products: %w", err)
	}
	return nil
}

func (r *SyncReferenceRepository) UpsertUsers(ctx context.Context, users []dto.UserSyncPayload) error {
	if len(users) == 0 {
		return nil
	}
	db := postgres.GetTxOrDB(ctx, r.db)

	models := make([]userModel, len(users))
	for i, u := range users {
		models[i] = userModel{
			ID:        u.ID,
			Username:  u.Username,
			Name:      u.Name,
			Password:  u.Password,
			CreatedAt: u.CreatedAt,
			UpdatedAt: u.UpdatedAt,
			DeletedAt: u.DeletedAt,
		}
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"username", "name", "password", "updated_at", "deleted_at",
		}),
	}).Create(&models).Error; err != nil {
		return fmt.Errorf("upsert users: %w", err)
	}

	return r.replaceUserRoles(db, users)
}

// replaceUserRoles rewrites the user_roles of every user in the batch to match the cloud
// snapshot: it clears the existing assignments for those users and reinserts the payload's
// RoleIDs. Replace (not insert-only) means a role removed in the cloud disappears on the
// edge too. Runs in the caller's pull transaction, so it commits with the user upsert.
func (r *SyncReferenceRepository) replaceUserRoles(db *gorm.DB, users []dto.UserSyncPayload) error {
	userIDs := make([]string, len(users))
	for i, u := range users {
		userIDs[i] = u.ID
	}

	if err := db.Where("user_id IN ?", userIDs).Delete(&userRoleModel{}).Error; err != nil {
		return fmt.Errorf("clear user roles: %w", err)
	}

	var assignments []userRoleModel
	for _, u := range users {
		for _, roleID := range u.RoleIDs {
			assignments = append(assignments, userRoleModel{UserID: u.ID, RoleID: roleID})
		}
	}
	if len(assignments) == 0 {
		return nil
	}
	if err := db.Create(&assignments).Error; err != nil {
		return fmt.Errorf("insert user roles: %w", err)
	}
	return nil
}

func (r *SyncReferenceRepository) UpsertSuppliers(ctx context.Context, suppliers []dto.SupplierSyncPayload) error {
	if len(suppliers) == 0 {
		return nil
	}
	db := postgres.GetTxOrDB(ctx, r.db)

	models := make([]supplierModel, len(suppliers))
	for i, s := range suppliers {
		models[i] = supplierModel{
			ID:                   s.ID,
			Name:                 s.Name,
			IdentificationType:   s.IdentificationType,
			IdentificationNumber: s.IdentificationNumber,
			ContactName:          s.ContactName,
			Phone:                s.Phone,
			Email:                s.Email,
			Notes:                s.Notes,
			CreatedAt:            s.CreatedAt,
			UpdatedAt:            s.UpdatedAt,
			DeletedAt:            s.DeletedAt,
		}
	}

	if err := db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"name", "identification_type", "identification_number", "contact_name",
			"phone", "email", "notes", "updated_at", "deleted_at",
		}),
	}).Create(&models).Error; err != nil {
		return fmt.Errorf("upsert suppliers: %w", err)
	}
	return nil
}

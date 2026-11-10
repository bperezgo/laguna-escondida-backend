package repository

import (
	"context"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type PlateOptionSlotRepository struct {
	db *gorm.DB
}

func NewPlateOptionSlotRepository(db *gorm.DB) ports.PlateOptionSlotRepository {
	return &PlateOptionSlotRepository{db: db}
}

// The three models keep DeletedAt a plain column so the sync pull can see removed rows.
type plateOptionSlotModel struct {
	postgres.TenantOwned
	ID                 string     `gorm:"type:uuid;primaryKey"`
	CompositeProductID string     `gorm:"type:uuid;not null;column:composite_product_id"`
	Name               string     `gorm:"not null;column:name"`
	Position           int        `gorm:"not null;column:position"`
	CreatedAt          time.Time  `gorm:"type:timestamp;not null"`
	UpdatedAt          time.Time  `gorm:"type:timestamp;not null"`
	DeletedAt          *time.Time `gorm:"type:timestamp;column:deleted_at"`
}

func (plateOptionSlotModel) TableName() string { return "plate_option_slots" }

type plateOptionChoiceModel struct {
	postgres.TenantOwned
	ID        string     `gorm:"type:uuid;primaryKey"`
	SlotID    string     `gorm:"type:uuid;not null;column:slot_id"`
	Name      string     `gorm:"not null;column:name"`
	Position  int        `gorm:"not null;column:position"`
	IsDefault bool       `gorm:"not null;column:is_default"`
	CreatedAt time.Time  `gorm:"type:timestamp;not null"`
	UpdatedAt time.Time  `gorm:"type:timestamp;not null"`
	DeletedAt *time.Time `gorm:"type:timestamp;column:deleted_at"`
}

func (plateOptionChoiceModel) TableName() string { return "plate_option_choices" }

type plateOptionChoiceItemModel struct {
	postgres.TenantOwned
	ID                  string     `gorm:"type:uuid;primaryKey"`
	ChoiceID            string     `gorm:"type:uuid;not null;column:choice_id"`
	IngredientProductID string     `gorm:"type:uuid;not null;column:ingredient_product_id"`
	Quantity            int        `gorm:"not null;column:quantity"`
	CreatedAt           time.Time  `gorm:"type:timestamp;not null"`
	UpdatedAt           time.Time  `gorm:"type:timestamp;not null"`
	DeletedAt           *time.Time `gorm:"type:timestamp;column:deleted_at"`
}

func (plateOptionChoiceItemModel) TableName() string { return "plate_option_choice_items" }

func (r *PlateOptionSlotRepository) FindByCompositeProductID(ctx context.Context, compositeProductID string) ([]dto.PlateOptionSlot, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	byPlate, err := loadPlateOptionSlots(db, []string{compositeProductID})
	if err != nil {
		return nil, err
	}
	return byPlate[compositeProductID], nil
}

func (r *PlateOptionSlotRepository) ReplaceForCompositeProduct(ctx context.Context, compositeProductID string, slots []dto.PlateOptionSlot) error {
	return postgres.TenantTx(ctx, r.db, func(tx *gorm.DB) error {
		return replacePlateOptionSlots(tx, compositeProductID, slots, time.Now())
	})
}

// loadPlateOptionSlots returns the live slots of each plate, choices and items in position
// order, with each item's ingredient name.
func loadPlateOptionSlots(db *gorm.DB, compositeProductIDs []string) (map[string][]dto.PlateOptionSlot, error) {
	result := make(map[string][]dto.PlateOptionSlot, len(compositeProductIDs))
	if len(compositeProductIDs) == 0 {
		return result, nil
	}

	var slotRows []plateOptionSlotModel
	if err := db.Where("composite_product_id IN ? AND deleted_at IS NULL", compositeProductIDs).
		Order("position ASC").Find(&slotRows).Error; err != nil {
		return nil, fmt.Errorf("query plate option slots: %w", err)
	}
	if len(slotRows) == 0 {
		return result, nil
	}
	slotIDs := make([]string, len(slotRows))
	for i, s := range slotRows {
		slotIDs[i] = s.ID
	}

	var choiceRows []plateOptionChoiceModel
	if err := db.Where("slot_id IN ? AND deleted_at IS NULL", slotIDs).
		Order("position ASC").Find(&choiceRows).Error; err != nil {
		return nil, fmt.Errorf("query plate option choices: %w", err)
	}
	choiceIDs := make([]string, len(choiceRows))
	for i, c := range choiceRows {
		choiceIDs[i] = c.ID
	}

	var itemRows []plateOptionChoiceItemModel
	if len(choiceIDs) > 0 {
		if err := db.Where("choice_id IN ? AND deleted_at IS NULL", choiceIDs).
			Order("created_at ASC, id ASC").Find(&itemRows).Error; err != nil {
			return nil, fmt.Errorf("query plate option choice items: %w", err)
		}
	}

	names, err := productNames(db, itemRows)
	if err != nil {
		return nil, err
	}

	itemsByChoice := make(map[string][]dto.PlateOptionChoiceItem, len(choiceRows))
	for _, it := range itemRows {
		itemsByChoice[it.ChoiceID] = append(itemsByChoice[it.ChoiceID], dto.PlateOptionChoiceItem{
			ID:                  it.ID,
			IngredientProductID: it.IngredientProductID,
			IngredientName:      names[it.IngredientProductID],
			Quantity:            it.Quantity,
			CreatedAt:           it.CreatedAt,
			UpdatedAt:           it.UpdatedAt,
		})
	}
	choicesBySlot := make(map[string][]dto.PlateOptionChoice, len(slotRows))
	for _, c := range choiceRows {
		items := itemsByChoice[c.ID]
		if items == nil {
			items = []dto.PlateOptionChoiceItem{}
		}
		choicesBySlot[c.SlotID] = append(choicesBySlot[c.SlotID], dto.PlateOptionChoice{
			ID:        c.ID,
			Name:      c.Name,
			Position:  c.Position,
			IsDefault: c.IsDefault,
			Items:     items,
			CreatedAt: c.CreatedAt,
			UpdatedAt: c.UpdatedAt,
		})
	}
	for _, s := range slotRows {
		result[s.CompositeProductID] = append(result[s.CompositeProductID], dto.PlateOptionSlot{
			ID:                 s.ID,
			CompositeProductID: s.CompositeProductID,
			Name:               s.Name,
			Position:           s.Position,
			Choices:            choicesBySlot[s.ID],
			CreatedAt:          s.CreatedAt,
			UpdatedAt:          s.UpdatedAt,
		})
	}
	return result, nil
}

func productNames(db *gorm.DB, items []plateOptionChoiceItemModel) (map[string]string, error) {
	names := make(map[string]string)
	if len(items) == 0 {
		return names, nil
	}
	ids := make([]string, 0, len(items))
	for _, it := range items {
		ids = append(ids, it.IngredientProductID)
	}
	var rows []struct {
		ID   string
		Name string
	}
	if err := db.Table("products").Select("id, name").Where("id IN ?", ids).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("query bundle ingredient names: %w", err)
	}
	for _, row := range rows {
		names[row.ID] = row.Name
	}
	return names, nil
}

// replacePlateOptionSlots makes slots the plate's whole live list inside tx: it soft-deletes
// every live slot, choice and item left out and upserts the rest by id, reviving any that
// were removed before. Both the cloud's PUT and the box's pull apply go through it.
func replacePlateOptionSlots(tx *gorm.DB, compositeProductID string, slots []dto.PlateOptionSlot, now time.Time) error {
	var liveSlotIDs []string
	if err := tx.Model(&plateOptionSlotModel{}).
		Where("composite_product_id = ? AND deleted_at IS NULL", compositeProductID).
		Pluck("id", &liveSlotIDs).Error; err != nil {
		return fmt.Errorf("query live slots: %w", err)
	}

	var slotRows []plateOptionSlotModel
	var choiceRows []plateOptionChoiceModel
	var itemRows []plateOptionChoiceItemModel
	keepSlots, keepChoices, keepItems := []string{}, []string{}, []string{}
	for _, s := range slots {
		keepSlots = append(keepSlots, s.ID)
		slotRows = append(slotRows, plateOptionSlotModel{
			ID: s.ID, CompositeProductID: compositeProductID, Name: s.Name, Position: s.Position,
			CreatedAt: s.CreatedAt, UpdatedAt: s.UpdatedAt,
		})
		for _, c := range s.Choices {
			keepChoices = append(keepChoices, c.ID)
			choiceRows = append(choiceRows, plateOptionChoiceModel{
				ID: c.ID, SlotID: s.ID, Name: c.Name, Position: c.Position, IsDefault: c.IsDefault,
				CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
			})
			for _, it := range c.Items {
				keepItems = append(keepItems, it.ID)
				itemRows = append(itemRows, plateOptionChoiceItemModel{
					ID: it.ID, ChoiceID: c.ID, IngredientProductID: it.IngredientProductID, Quantity: it.Quantity,
					CreatedAt: it.CreatedAt, UpdatedAt: it.UpdatedAt,
				})
			}
		}
	}

	if len(liveSlotIDs) > 0 {
		var liveChoiceIDs []string
		if err := tx.Model(&plateOptionChoiceModel{}).
			Where("slot_id IN ? AND deleted_at IS NULL", liveSlotIDs).
			Pluck("id", &liveChoiceIDs).Error; err != nil {
			return fmt.Errorf("query live choices: %w", err)
		}

		// Clear defaults first: moving the default between two choices would otherwise trip
		// the one-default index on the first row of the upsert.
		if len(liveChoiceIDs) > 0 {
			if err := tx.Model(&plateOptionChoiceModel{}).Where("id IN ?", liveChoiceIDs).
				Update("is_default", false).Error; err != nil {
				return fmt.Errorf("clear default choices: %w", err)
			}
			if err := softDeleteLeftOut(tx, &plateOptionChoiceItemModel{}, "choice_id", liveChoiceIDs, keepItems, now); err != nil {
				return err
			}
		}
		if err := softDeleteLeftOut(tx, &plateOptionChoiceModel{}, "slot_id", liveSlotIDs, keepChoices, now); err != nil {
			return err
		}
		if err := softDeleteLeftOut(tx, &plateOptionSlotModel{}, "id", liveSlotIDs, keepSlots, now); err != nil {
			return err
		}
	}

	if err := upsertRows(tx, &slotRows, "plate_option_slots",
		"composite_product_id", "name", "position", "updated_at", "deleted_at"); err != nil {
		return err
	}
	if err := upsertRows(tx, &choiceRows, "plate_option_choices",
		"slot_id", "name", "position", "is_default", "updated_at", "deleted_at"); err != nil {
		return err
	}
	return upsertRows(tx, &itemRows, "plate_option_choice_items",
		"choice_id", "ingredient_product_id", "quantity", "updated_at", "deleted_at")
}

func softDeleteLeftOut[M plateOptionSlotModel | plateOptionChoiceModel | plateOptionChoiceItemModel](
	tx *gorm.DB, model *M, parentColumn string, parentIDs, keepIDs []string, now time.Time,
) error {
	query := tx.Model(model).Where(parentColumn+" IN ? AND deleted_at IS NULL", parentIDs)
	if len(keepIDs) > 0 {
		query = query.Where("id NOT IN ?", keepIDs)
	}
	if err := query.Updates(map[string]any{"deleted_at": now, "updated_at": now}).Error; err != nil {
		return fmt.Errorf("soft-delete removed %s rows: %w", parentColumn, err)
	}
	return nil
}

func upsertRows[M plateOptionSlotModel | plateOptionChoiceModel | plateOptionChoiceItemModel](
	tx *gorm.DB, rows *[]M, table string, columns ...string,
) error {
	if len(*rows) == 0 {
		return nil
	}
	if err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		Where:     postgres.SameOrganization(table),
		DoUpdates: clause.AssignmentColumns(columns),
	}).Create(rows).Error; err != nil {
		return fmt.Errorf("upsert %s: %w", table, err)
	}
	return nil
}

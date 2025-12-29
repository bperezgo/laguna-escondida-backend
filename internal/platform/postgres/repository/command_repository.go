package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/aggregate/command"
	"laguna-escondida/backend/internal/domain/aggregate/command_item"
	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"gorm.io/gorm"
)

type CommandRepository struct {
	db *gorm.DB
}

func NewCommandRepository(db *gorm.DB) ports.CommandRepository {
	return &CommandRepository{db: db}
}

type commandModel struct {
	ID         string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OpenBillID string    `gorm:"type:uuid;not null"`
	Area       string    `gorm:"type:varchar(255);not null"`
	Status     string    `gorm:"type:varchar(50);not null;default:pending"`
	CreatedAt  time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
	UpdatedAt  time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (commandModel) TableName() string {
	return "commands"
}

type commandItemModel struct {
	ID                string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CommandID         string    `gorm:"type:uuid;not null"`
	ProductID         string    `gorm:"type:uuid;not null"`
	OpenBillProductID string    `gorm:"type:uuid;not null;references:open_bills_products(id)"`
	Priority          int       `gorm:"type:integer;not null;default:0"`
	Status            string    `gorm:"type:varchar(50);not null;default:created"`
	Quantity          int       `gorm:"type:integer;not null;default:1"`
	Notes             *string   `gorm:"type:text"`
	CreatedAt         time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (commandItemModel) TableName() string {
	return "command_items"
}

func (r *CommandRepository) Create(ctx context.Context, cmd *command.Aggregate) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	return db.Transaction(func(tx *gorm.DB) error {
		model := &commandModel{
			ID:         cmd.ID(),
			OpenBillID: cmd.OpenBillID(),
			Area:       cmd.Area(),
			Status:     string(cmd.Status()),
			CreatedAt:  cmd.CreatedAt(),
			UpdatedAt:  cmd.UpdatedAt(),
		}

		if err := tx.Create(model).Error; err != nil {
			return err
		}

		for _, item := range cmd.Items() {
			itemModel := &commandItemModel{
				ID:                item.ID(),
				CommandID:         cmd.ID(),
				ProductID:         item.ProductID(),
				OpenBillProductID: item.OpenBillProductID(),
				Priority:          item.Priority(),
				Status:            string(item.Status()),
				Quantity:          item.Quantity(),
				Notes:             item.Notes(),
				CreatedAt:         cmd.CreatedAt(),
			}
			if err := tx.Create(itemModel).Error; err != nil {
				return err
			}
		}

		return nil
	})
}

func (r *CommandRepository) FindByID(ctx context.Context, id string) (*command.Aggregate, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		ID                 string
		OpenBillID         string
		Area               string
		Status             string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		TemporalIdentifier string
		UserID             string
		UserUsername       string
		UserName           string
	}

	var res result
	err := db.Table("commands").
		Select(`
			commands.id,
			commands.open_bill_id,
			commands.area,
			commands.status,
			commands.created_at,
			commands.updated_at,
			open_bills.temporal_identifier,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("INNER JOIN open_bills ON commands.open_bill_id = open_bills.id").
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("commands.id = ?", id).
		Scan(&res).Error

	if err != nil {
		return nil, err
	}

	if res.ID == "" {
		return nil, domainError.ErrCommandNotFound
	}

	var createdBy *dto.OpenBillCreator
	if res.UserID != "" {
		createdBy = &dto.OpenBillCreator{
			ID:       res.UserID,
			Username: res.UserUsername,
			Name:     res.UserName,
		}
	}

	items, err := r.findItemAggregatesByCommandID(db, id)
	if err != nil {
		return nil, err
	}

	return command.NewCommandFromRepository(
		res.ID,
		res.OpenBillID,
		res.TemporalIdentifier,
		createdBy,
		res.Area,
		items,
		res.CreatedAt,
		res.UpdatedAt,
	)
}

func (r *CommandRepository) FindByItemID(ctx context.Context, itemID string) (*command.Aggregate, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var commandID string
	err := db.Table("command_items").
		Select("command_id").
		Where("id = ?", itemID).
		Scan(&commandID).Error

	if err != nil {
		return nil, err
	}

	if commandID == "" {
		return nil, domainError.ErrCommandItemNotFound
	}

	return r.FindByID(ctx, commandID)
}

func (r *CommandRepository) FindByOpenBillID(ctx context.Context, openBillID string) (*command.Aggregate, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		ID                 string
		OpenBillID         string
		Area               string
		Status             string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		TemporalIdentifier string
		UserID             string
		UserUsername       string
		UserName           string
	}

	var res result
	err := db.Table("commands").
		Select(`
			commands.id,
			commands.open_bill_id,
			commands.area,
			commands.status,
			commands.created_at,
			commands.updated_at,
			open_bills.temporal_identifier,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("INNER JOIN open_bills ON commands.open_bill_id = open_bills.id").
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("commands.open_bill_id = ?", openBillID).
		Scan(&res).Error

	if err != nil {
		return nil, err
	}

	if res.ID == "" {
		return nil, domainError.ErrCommandNotFound
	}

	var createdBy *dto.OpenBillCreator
	if res.UserID != "" {
		createdBy = &dto.OpenBillCreator{
			ID:       res.UserID,
			Username: res.UserUsername,
			Name:     res.UserName,
		}
	}

	items, err := r.findItemAggregatesByCommandID(db, res.ID)
	if err != nil {
		return nil, err
	}

	return command.NewCommandFromRepository(
		res.ID,
		res.OpenBillID,
		res.TemporalIdentifier,
		createdBy,
		res.Area,
		items,
		res.CreatedAt,
		res.UpdatedAt,
	)
}

func (r *CommandRepository) FindAllByOpenBillID(ctx context.Context, openBillID string) ([]*command.Aggregate, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		ID                 string
		OpenBillID         string
		Area               string
		Status             string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		TemporalIdentifier string
		UserID             string
		UserUsername       string
		UserName           string
	}

	var results []result
	err := db.Table("commands").
		Select(`
			commands.id,
			commands.open_bill_id,
			commands.area,
			commands.status,
			commands.created_at,
			commands.updated_at,
			open_bills.temporal_identifier,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("INNER JOIN open_bills ON commands.open_bill_id = open_bills.id").
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("commands.open_bill_id = ?", openBillID).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	commands := make([]*command.Aggregate, 0, len(results))
	for _, res := range results {
		var createdBy *dto.OpenBillCreator
		if res.UserID != "" {
			createdBy = &dto.OpenBillCreator{
				ID:       res.UserID,
				Username: res.UserUsername,
				Name:     res.UserName,
			}
		}

		items, err := r.findItemAggregatesByCommandID(db, res.ID)
		if err != nil {
			return nil, err
		}

		cmd, err := command.NewCommandFromRepository(
			res.ID,
			res.OpenBillID,
			res.TemporalIdentifier,
			createdBy,
			res.Area,
			items,
			res.CreatedAt,
			res.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		commands = append(commands, cmd)
	}

	return commands, nil
}

func (r *CommandRepository) FindByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	db := postgres.GetTxOrDB(ctx, r.db)
	return r.findCommandsByAreaAndStatus(db, area, "")
}

func (r *CommandRepository) FindPendingByArea(ctx context.Context, area string) ([]*dto.Command, error) {
	db := postgres.GetTxOrDB(ctx, r.db)
	return r.findCommandsByAreaAndStatus(db, area, string(dto.CommandStatusCreated))
}

func (r *CommandRepository) findCommandsByAreaAndStatus(db *gorm.DB, area string, status string) ([]*dto.Command, error) {
	type result struct {
		ID                 string
		OpenBillID         string
		Area               string
		Status             string
		CreatedAt          time.Time
		UpdatedAt          time.Time
		TemporalIdentifier string
		UserID             string
		UserUsername       string
		UserName           string
	}

	query := db.Table("commands").
		Select(`
			commands.id,
			commands.open_bill_id,
			commands.area,
			commands.status,
			commands.created_at,
			commands.updated_at,
			open_bills.temporal_identifier,
			users.id as user_id,
			users.username as user_username,
			users.name as user_name
		`).
		Joins("INNER JOIN open_bills ON commands.open_bill_id = open_bills.id").
		Joins("LEFT JOIN users ON open_bills.created_by = users.id AND users.deleted_at IS NULL").
		Where("commands.area = ?", area)

	if status != "" {
		query = query.Where("commands.status = ?", status)
	}

	var results []result
	if err := query.Order("commands.created_at ASC").Scan(&results).Error; err != nil {
		return nil, err
	}

	commands := make([]*dto.Command, len(results))
	for i, res := range results {
		var createdBy *dto.OpenBillCreator
		if res.UserID != "" {
			createdBy = &dto.OpenBillCreator{
				ID:       res.UserID,
				Username: res.UserUsername,
				Name:     res.UserName,
			}
		}

		items, err := r.findItemsByCommandID(db, res.ID)
		if err != nil {
			return nil, err
		}

		commands[i] = &dto.Command{
			ID:                 res.ID,
			OpenBillID:         res.OpenBillID,
			TemporalIdentifier: res.TemporalIdentifier,
			CreatedBy:          createdBy,
			Area:               res.Area,
			Status:             dto.CommandStatus(res.Status),
			Items:              items,
			CreatedAt:          res.CreatedAt,
			UpdatedAt:          res.UpdatedAt,
		}
	}

	return commands, nil
}

func (r *CommandRepository) findItemsByCommandID(db *gorm.DB, commandID string) ([]dto.CommandItem, error) {
	type itemResult struct {
		ID                string
		ProductID         string
		ProductName       string
		Quantity          int
		Notes             *string
		OpenBillProductID string
		Priority          int
		Status            dto.CommandStatus
	}

	var itemResults []itemResult
	err := db.Table("command_items").
		Select(`
			command_items.id,
			command_items.product_id,
			products.name as product_name,
			command_items.quantity,
			command_items.notes,
			command_items.open_bill_product_id,
			command_items.priority,
			command_items.status
		`).
		Joins("INNER JOIN products ON command_items.product_id = products.id").
		Where("command_items.command_id = ?", commandID).
		Scan(&itemResults).Error

	if err != nil {
		return nil, err
	}

	items := make([]dto.CommandItem, len(itemResults))
	for i, ir := range itemResults {
		items[i] = dto.CommandItem{
			ID:                ir.ID,
			ProductID:         ir.ProductID,
			ProductName:       ir.ProductName,
			Quantity:          ir.Quantity,
			Notes:             ir.Notes,
			OpenBillProductID: ir.OpenBillProductID,
			Priority:          ir.Priority,
			Status:            ir.Status,
		}
	}

	return items, nil
}

func (r *CommandRepository) findItemAggregatesByCommandID(db *gorm.DB, commandID string) ([]*command_item.Aggregate, error) {
	type itemResult struct {
		ID                string
		ProductID         string
		ProductName       string
		Quantity          int
		Notes             *string
		OpenBillProductID string
		Priority          int
		Status            dto.CommandStatus
	}

	var itemResults []itemResult
	err := db.Table("command_items").
		Select(`
			command_items.id,
			command_items.product_id,
			products.name as product_name,
			command_items.quantity,
			command_items.notes,
			command_items.open_bill_product_id,
			command_items.priority,
			command_items.status
		`).
		Joins("INNER JOIN products ON command_items.product_id = products.id").
		Where("command_items.command_id = ?", commandID).
		Scan(&itemResults).Error

	if err != nil {
		return nil, err
	}

	items := make([]*command_item.Aggregate, len(itemResults))
	for i, ir := range itemResults {
		item, err := command_item.NewCommandItemFromRepository(
			ir.ID,
			ir.OpenBillProductID,
			ir.ProductID,
			ir.ProductName,
			ir.Quantity,
			ir.Notes,
			ir.Status,
			ir.Priority,
		)
		if err != nil {
			return nil, err
		}
		items[i] = item
	}

	return items, nil
}

func (r *CommandRepository) Update(ctx context.Context, cmd *command.Aggregate) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	return db.Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&commandModel{}).
			Where("id = ?", cmd.ID()).
			Updates(map[string]any{
				"status":     string(cmd.Status()),
				"updated_at": cmd.UpdatedAt(),
			})

		if result.Error != nil {
			return result.Error
		}

		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		for _, item := range cmd.Items() {
			itemResult := tx.Model(&commandItemModel{}).
				Where("id = ?", item.ID()).
				Updates(map[string]any{
					"status":   string(item.Status()),
					"quantity": item.Quantity(),
					"notes":    item.Notes(),
				})

			if itemResult.Error != nil {
				return itemResult.Error
			}
		}

		return nil
	})
}

func (r *CommandRepository) GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]dto.ProductPreparationResponsibilityWithProduct, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	type result struct {
		ProductID   string
		ProductName string
		Area        string
		Priority    int
	}

	var results []result
	err := db.Table("product_preparation_responsibilities").
		Select(`
			product_preparation_responsibilities.product_id,
			products.name as product_name,
			product_preparation_responsibilities.area,
			product_preparation_responsibilities.priority
		`).
		Joins("INNER JOIN products ON product_preparation_responsibilities.product_id = products.id").
		Where("product_preparation_responsibilities.product_id IN ? AND product_preparation_responsibilities.deleted_at IS NULL", productIDs).
		Scan(&results).Error

	if err != nil {
		return nil, err
	}

	responsibilities := make([]dto.ProductPreparationResponsibilityWithProduct, len(results))
	for i, r := range results {
		responsibilities[i] = dto.ProductPreparationResponsibilityWithProduct{
			ProductID:   r.ProductID,
			ProductName: r.ProductName,
			Area:        r.Area,
			Priority:    r.Priority,
		}
	}

	return responsibilities, nil
}

var _ ports.CommandRepository = (*CommandRepository)(nil)

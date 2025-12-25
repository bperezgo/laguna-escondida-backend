package repository

import (
	"context"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
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
	ID        string    `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	CommandID string    `gorm:"type:uuid;not null"`
	ProductID string    `gorm:"type:uuid;not null"`
	Quantity  int       `gorm:"type:integer;not null;default:1"`
	Notes     *string   `gorm:"type:text"`
	CreatedAt time.Time `gorm:"type:timestamp;not null;default:CURRENT_TIMESTAMP"`
}

func (commandItemModel) TableName() string {
	return "command_items"
}

func (r *CommandRepository) Create(ctx context.Context, command *dto.Command) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	return db.Transaction(func(tx *gorm.DB) error {
		model := &commandModel{
			ID:         command.ID,
			OpenBillID: command.OpenBillID,
			Area:       command.Area,
			Status:     string(command.Status),
			CreatedAt:  command.CreatedAt,
			UpdatedAt:  command.UpdatedAt,
		}

		if err := tx.Create(model).Error; err != nil {
			return err
		}

		for i := range command.Items {
			itemModel := &commandItemModel{
				ID:        command.Items[i].ID,
				CommandID: command.ID,
				ProductID: command.Items[i].ProductID,
				Quantity:  command.Items[i].Quantity,
				Notes:     command.Items[i].Notes,
				CreatedAt: command.CreatedAt,
			}
			if err := tx.Create(itemModel).Error; err != nil {
				return err
			}
			command.Items[i].ID = itemModel.ID
		}

		return nil
	})
}

func (r *CommandRepository) FindByID(ctx context.Context, id string) (*dto.Command, error) {
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
		return nil, gorm.ErrRecordNotFound
	}

	var createdBy *dto.OpenBillCreator
	if res.UserID != "" {
		createdBy = &dto.OpenBillCreator{
			ID:       res.UserID,
			Username: res.UserUsername,
			Name:     res.UserName,
		}
	}

	items, err := r.findItemsByCommandID(db, id)
	if err != nil {
		return nil, err
	}

	return &dto.Command{
		ID:                 res.ID,
		OpenBillID:         res.OpenBillID,
		TemporalIdentifier: res.TemporalIdentifier,
		CreatedBy:          createdBy,
		Area:               res.Area,
		Status:             dto.CommandStatus(res.Status),
		Items:              items,
		CreatedAt:          res.CreatedAt,
		UpdatedAt:          res.UpdatedAt,
	}, nil
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
		ID          string
		ProductID   string
		ProductName string
		Quantity    int
		Notes       *string
	}

	var itemResults []itemResult
	err := db.Table("command_items").
		Select(`
			command_items.id,
			command_items.product_id,
			products.name as product_name,
			command_items.quantity,
			command_items.notes
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
			ID:          ir.ID,
			ProductID:   ir.ProductID,
			ProductName: ir.ProductName,
			Quantity:    ir.Quantity,
			Notes:       ir.Notes,
		}
	}

	return items, nil
}

func (r *CommandRepository) UpdateStatus(ctx context.Context, id string, status dto.CommandStatus) error {
	db := postgres.GetTxOrDB(ctx, r.db)

	result := db.Model(&commandModel{}).
		Where("id = ?", id).
		Update("status", string(status))

	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (r *CommandRepository) GetProductPreparationResponsibilities(ctx context.Context, productIDs []string) ([]ports.ProductPreparationResponsibility, error) {
	db := postgres.GetTxOrDB(ctx, r.db)

	var models []productPreparationResponsibilityModel
	err := db.Where("product_id IN ? AND deleted_at IS NULL", productIDs).
		Find(&models).Error

	if err != nil {
		return nil, err
	}

	responsibilities := make([]ports.ProductPreparationResponsibility, len(models))
	for i, m := range models {
		responsibilities[i] = ports.ProductPreparationResponsibility{
			ProductID: m.ProductID,
			Area:      m.Area,
		}
	}

	return responsibilities, nil
}

var _ ports.CommandRepository = (*CommandRepository)(nil)

package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

const (
	pgUniqueViolation          = "23505"
	organizationSlugConstraint = "organizations_slug_key"
)

type organizationModel struct {
	ID                string    `gorm:"column:id;primaryKey"`
	Name              string    `gorm:"column:name"`
	Slug              string    `gorm:"column:slug"`
	Status            string    `gorm:"column:status"`
	Timezone          string    `gorm:"column:timezone"`
	Country           string    `gorm:"column:country"`
	Locale            string    `gorm:"column:locale"`
	Currency          string    `gorm:"column:currency"`
	LegalName         *string   `gorm:"column:legal_name"`
	NIT               *string   `gorm:"column:nit"`
	Address           *string   `gorm:"column:address"`
	Phone             *string   `gorm:"column:phone"`
	TicketFooter      *string   `gorm:"column:ticket_footer"`
	TicketLegalNotice *string   `gorm:"column:ticket_legal_notice"`
	DisplayName       *string   `gorm:"column:display_name"`
	LogoURL           *string   `gorm:"column:logo_url"`
	PrimaryColor      *string   `gorm:"column:primary_color"`
	SecondaryColor    *string   `gorm:"column:secondary_color"`
	CreatedAt         time.Time `gorm:"column:created_at"`
	UpdatedAt         time.Time `gorm:"column:updated_at"`
}

func (organizationModel) TableName() string {
	return "organizations"
}

type OrganizationRepository struct {
	db *gorm.DB
}

func NewOrganizationRepository(db *gorm.DB) ports.OrganizationRepository {
	return &OrganizationRepository{db: db}
}

func (r *OrganizationRepository) Create(ctx context.Context, org *dto.Organization) error {
	model := organizationToModel(org)
	if err := postgres.GetTxOrDB(ctx, r.db).Create(model).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == organizationSlugConstraint {
			return domainError.ErrOrganizationSlugTaken
		}
		return fmt.Errorf("create organization: %w", err)
	}
	return nil
}

func (r *OrganizationRepository) Get(ctx context.Context, id string) (*dto.Organization, error) {
	// A malformed id can't match the UUID column; Postgres would reject it as a 500-worthy error.
	if uuid.Validate(id) != nil {
		return nil, domainError.ErrOrganizationNotFound
	}
	db := postgres.GetTxOrDB(ctx, r.db)
	var model organizationModel
	err := db.Where("id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrOrganizationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get organization: %w", err)
	}
	org := organizationToDTO(&model)
	settings, err := findEInvoicingSettings(db, id)
	switch {
	case errors.Is(err, domainError.ErrEInvoicingNotConfigured):
	case err != nil:
		return nil, err
	default:
		org.EInvoicing = einvoicingSettingsToDTO(settings)
	}
	return org, nil
}

func (r *OrganizationRepository) List(ctx context.Context) ([]*dto.Organization, error) {
	db := postgres.GetTxOrDB(ctx, r.db)
	var models []organizationModel
	if err := db.Order("name, id").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list organizations: %w", err)
	}
	if len(models) == 0 {
		return []*dto.Organization{}, nil
	}
	ids := make([]string, len(models))
	for i := range models {
		ids[i] = models[i].ID
	}
	var settings []einvoicingSettingsModel
	if err := db.Where("organization_id IN ?", ids).Find(&settings).Error; err != nil {
		return nil, fmt.Errorf("list organization e-invoicing settings: %w", err)
	}
	byOrg := make(map[string]*einvoicingSettingsModel, len(settings))
	for i := range settings {
		byOrg[settings[i].OrganizationID] = &settings[i]
	}

	orgs := make([]*dto.Organization, len(models))
	for i := range models {
		orgs[i] = organizationToDTO(&models[i])
		if s, ok := byOrg[models[i].ID]; ok {
			orgs[i].EInvoicing = einvoicingSettingsToDTO(s)
		}
	}
	return orgs, nil
}

func (r *OrganizationRepository) Update(ctx context.Context, id string, req *dto.UpdateOrganizationRequest) (*dto.Organization, error) {
	if uuid.Validate(id) != nil {
		return nil, domainError.ErrOrganizationNotFound
	}
	updates := map[string]any{"updated_at": time.Now()}
	for column, value := range map[string]*string{
		"name": req.Name, "status": req.Status, "timezone": req.Timezone,
		"country": req.Country, "locale": req.Locale, "currency": req.Currency,
	} {
		if value != nil {
			updates[column] = *value
		}
	}
	for column, value := range map[string]*string{
		"legal_name": req.LegalName, "nit": req.NIT, "address": req.Address, "phone": req.Phone,
		"ticket_footer": req.TicketFooter, "ticket_legal_notice": req.TicketLegalNotice,
		"display_name": req.DisplayName, "logo_url": req.LogoURL,
		"primary_color": req.PrimaryColor, "secondary_color": req.SecondaryColor,
	} {
		switch {
		case value == nil:
		case *value == "":
			updates[column] = nil
		default:
			updates[column] = *value
		}
	}

	result := postgres.GetTxOrDB(ctx, r.db).Model(&organizationModel{}).Where("id = ?", id).Updates(updates)
	if result.Error != nil {
		return nil, fmt.Errorf("update organization: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, domainError.ErrOrganizationNotFound
	}
	return r.Get(ctx, id)
}

func organizationToModel(org *dto.Organization) *organizationModel {
	return &organizationModel{
		ID: org.ID, Name: org.Name, Slug: org.Slug, Status: org.Status,
		Timezone: org.Timezone, Country: org.Country, Locale: org.Locale, Currency: org.Currency,
		LegalName: org.LegalName, NIT: org.NIT, Address: org.Address, Phone: org.Phone,
		TicketFooter: org.TicketFooter, TicketLegalNotice: org.TicketLegalNotice,
		DisplayName: org.DisplayName, LogoURL: org.LogoURL,
		PrimaryColor: org.PrimaryColor, SecondaryColor: org.SecondaryColor,
		CreatedAt: org.CreatedAt, UpdatedAt: org.UpdatedAt,
	}
}

func organizationToDTO(m *organizationModel) *dto.Organization {
	return &dto.Organization{
		ID: m.ID, Name: m.Name, Slug: m.Slug, Status: m.Status,
		Timezone: m.Timezone, Country: m.Country, Locale: m.Locale, Currency: m.Currency,
		LegalName: m.LegalName, NIT: m.NIT, Address: m.Address, Phone: m.Phone,
		TicketFooter: m.TicketFooter, TicketLegalNotice: m.TicketLegalNotice,
		DisplayName: m.DisplayName, LogoURL: m.LogoURL,
		PrimaryColor: m.PrimaryColor, SecondaryColor: m.SecondaryColor,
		CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
}

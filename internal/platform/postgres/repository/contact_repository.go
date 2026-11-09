package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository(db *gorm.DB) ports.ContactRepository {
	return &ContactRepository{db: db}
}

type contactModel struct {
	postgres.TenantOwned
	ID                     string     `gorm:"column:id;primaryKey"`
	Phone                  string     `gorm:"column:phone"`
	Name                   *string    `gorm:"column:name"`
	FirstTouchSource       *string    `gorm:"column:first_touch_source"`
	FirstTouchAdID         *string    `gorm:"column:first_touch_ad_id"`
	FirstTouchClickedAt    *time.Time `gorm:"column:first_touch_clicked_at"`
	FirstTouchCampaignID   *string    `gorm:"column:first_touch_campaign_id"`
	FirstTouchCampaignName *string    `gorm:"column:first_touch_campaign_name"`
	FirstSeenAt            time.Time  `gorm:"column:first_seen_at"`
	LastSeenAt             time.Time  `gorm:"column:last_seen_at"`
	CreatedAt              time.Time  `gorm:"column:created_at"`
	UpdatedAt              time.Time  `gorm:"column:updated_at"`
}

func (contactModel) TableName() string {
	return "contacts"
}

func newContactModel(c *dto.Contact) *contactModel {
	m := &contactModel{
		ID: c.ID, Phone: c.Phone, Name: c.Name,
		FirstSeenAt: c.FirstSeenAt, LastSeenAt: c.LastSeenAt, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt,
	}
	if t := c.FirstTouch; t != nil {
		m.FirstTouchSource, m.FirstTouchAdID, m.FirstTouchClickedAt = t.Source, t.AdID, t.ClickedAt
		m.FirstTouchCampaignID, m.FirstTouchCampaignName = t.CampaignID, t.CampaignName
	}
	return m
}

func (m *contactModel) toDTO() *dto.Contact {
	c := &dto.Contact{
		ID: m.ID, Phone: m.Phone, Name: m.Name,
		FirstSeenAt: m.FirstSeenAt, LastSeenAt: m.LastSeenAt, CreatedAt: m.CreatedAt, UpdatedAt: m.UpdatedAt,
	}
	touch := &dto.Attribution{
		Source: m.FirstTouchSource, AdID: m.FirstTouchAdID, ClickedAt: m.FirstTouchClickedAt,
		CampaignID: m.FirstTouchCampaignID, CampaignName: m.FirstTouchCampaignName,
	}
	if touch.Source != nil || touch.AdID != nil || touch.ClickedAt != nil || touch.CampaignID != nil || touch.CampaignName != nil {
		c.FirstTouch = touch
	}
	return c
}

// Create inserts nothing when the phone already has a contact, so a concurrent create is reported, not raised.
func (r *ContactRepository) Create(ctx context.Context, contact *dto.Contact) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "organization_id"}, {Name: "phone"}},
		DoNothing: true,
	}).Create(newContactModel(contact))
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return domainError.ErrContactAlreadyExists
	}
	return nil
}

// Update never writes the phone; first touch is the aggregate's to keep once set.
func (r *ContactRepository) Update(ctx context.Context, contact *dto.Contact) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	m := newContactModel(contact)
	return db.Model(&contactModel{}).Where("id = ?", contact.ID).Updates(map[string]any{
		"name":                      m.Name,
		"first_touch_source":        m.FirstTouchSource,
		"first_touch_ad_id":         m.FirstTouchAdID,
		"first_touch_clicked_at":    m.FirstTouchClickedAt,
		"first_touch_campaign_id":   m.FirstTouchCampaignID,
		"first_touch_campaign_name": m.FirstTouchCampaignName,
		"last_seen_at":              m.LastSeenAt,
		"updated_at":                m.UpdatedAt,
	}).Error
}

func (r *ContactRepository) FindByPhone(ctx context.Context, phone string) (*dto.Contact, error) {
	return r.findOne(ctx, "phone = ?", phone)
}

func (r *ContactRepository) FindByID(ctx context.Context, id string) (*dto.Contact, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, domainError.ErrContactNotFound
	}
	return r.findOne(ctx, "id = ?", id)
}

func (r *ContactRepository) findOne(ctx context.Context, where string, value string) (*dto.Contact, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var m contactModel
	err = db.Where(where, value).Take(&m).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrContactNotFound
	}
	if err != nil {
		return nil, err
	}
	return m.toDTO(), nil
}

func (r *ContactRepository) List(ctx context.Context, filter dto.ContactListFilter) ([]*dto.Contact, int, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, 0, err
	}
	matching := func() *gorm.DB {
		query := db.Model(&contactModel{})
		if filter.Search != "" {
			pattern := "%" + escapeLike(filter.Search) + "%"
			query = query.Where("(contacts.phone LIKE ? OR contacts.name ILIKE ?)", pattern, pattern)
		}
		return query
	}
	var total int64
	if err := matching().Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var models []contactModel
	if err := matching().Order("last_seen_at DESC, id DESC").Limit(filter.Limit).Offset(filter.Offset).Find(&models).Error; err != nil {
		return nil, 0, err
	}
	contacts := make([]*dto.Contact, len(models))
	for i := range models {
		contacts[i] = models[i].toDTO()
	}
	return contacts, int(total), nil
}

func escapeLike(value string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(value)
}

package repository

import (
	"context"
	"database/sql/driver"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"laguna-escondida/backend/internal/domain/dto"
	domainError "laguna-escondida/backend/internal/domain/error"
	"laguna-escondida/backend/internal/domain/ports"
	"laguna-escondida/backend/internal/platform/postgres"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const invitationPendingEmailIndex = "uq_invitations_pending_email"

// intArray is a Postgres integer[] in its text form, e.g. {2,5}.
type intArray []int

func (a intArray) Value() (driver.Value, error) {
	parts := make([]string, len(a))
	for i, v := range a {
		parts[i] = strconv.Itoa(v)
	}
	return "{" + strings.Join(parts, ",") + "}", nil
}

func (a *intArray) Scan(src any) error {
	var text string
	switch v := src.(type) {
	case string:
		text = v
	case []byte:
		text = string(v)
	case nil:
		*a = nil
		return nil
	default:
		return fmt.Errorf("scan integer array: unsupported type %T", src)
	}
	text = strings.Trim(text, "{}")
	if text == "" {
		*a = intArray{}
		return nil
	}
	parts := strings.Split(text, ",")
	out := make(intArray, len(parts))
	for i, p := range parts {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil {
			return fmt.Errorf("scan integer array: %w", err)
		}
		out[i] = n
	}
	*a = out
	return nil
}

type invitationModel struct {
	ID             string     `gorm:"column:id;primaryKey"`
	OrganizationID string     `gorm:"column:organization_id"`
	Email          string     `gorm:"column:email"`
	Name           string     `gorm:"column:name"`
	RoleIDs        intArray   `gorm:"column:role_ids;type:integer[]"`
	TokenHash      string     `gorm:"column:token_hash"`
	InvitedBy      string     `gorm:"column:invited_by"`
	ExpiresAt      time.Time  `gorm:"column:expires_at"`
	AcceptedAt     *time.Time `gorm:"column:accepted_at"`
	RevokedAt      *time.Time `gorm:"column:revoked_at"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	InvitedByName  string     `gorm:"column:invited_by_name;->;-:migration"`
}

func (invitationModel) TableName() string { return "invitations" }

func (m *invitationModel) toDTO() *dto.Invitation {
	return &dto.Invitation{
		ID: m.ID, OrganizationID: m.OrganizationID, Email: m.Email, Name: m.Name, RoleIDs: []int(m.RoleIDs),
		InvitedBy: m.InvitedBy, InvitedByName: m.InvitedByName, ExpiresAt: m.ExpiresAt, CreatedAt: m.CreatedAt,
		AcceptedAt: m.AcceptedAt, RevokedAt: m.RevokedAt,
	}
}

type InvitationRepository struct {
	db *gorm.DB
}

func NewInvitationRepository(db *gorm.DB) ports.InvitationRepository {
	return &InvitationRepository{db: db}
}

const invitationColumns = "invitations.*, users.name AS invited_by_name"

func (r *InvitationRepository) Create(ctx context.Context, inv *dto.Invitation, tokenHash string) error {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return err
	}
	model := &invitationModel{
		ID: inv.ID, OrganizationID: orgID, Email: inv.Email, Name: inv.Name, RoleIDs: intArray(inv.RoleIDs),
		TokenHash: tokenHash, InvitedBy: inv.InvitedBy, ExpiresAt: inv.ExpiresAt, CreatedAt: inv.CreatedAt,
	}
	if err := postgres.GetTxOrDB(ctx, r.db).Omit("invited_by_name").Create(model).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == invitationPendingEmailIndex {
			return domainError.ErrInvitationPending
		}
		return fmt.Errorf("create invitation: %w", err)
	}
	inv.OrganizationID = orgID
	return nil
}

func (r *InvitationRepository) pending(ctx context.Context) (*gorm.DB, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	return db.Model(&invitationModel{}).Select(invitationColumns).
		Joins("LEFT JOIN users ON users.id = invitations.invited_by").
		Where("invitations.accepted_at IS NULL AND invitations.revoked_at IS NULL"), nil
}

func (r *InvitationRepository) ListPending(ctx context.Context) ([]*dto.Invitation, error) {
	q, err := r.pending(ctx)
	if err != nil {
		return nil, err
	}
	var models []invitationModel
	if err := q.Order("invitations.created_at DESC").Find(&models).Error; err != nil {
		return nil, fmt.Errorf("list invitations: %w", err)
	}
	out := make([]*dto.Invitation, len(models))
	for i := range models {
		out[i] = models[i].toDTO()
	}
	return out, nil
}

func (r *InvitationRepository) FindPending(ctx context.Context, id string) (*dto.Invitation, error) {
	if uuid.Validate(id) != nil {
		return nil, domainError.ErrInvitationNotFound
	}
	q, err := r.pending(ctx)
	if err != nil {
		return nil, err
	}
	var model invitationModel
	err = q.Where("invitations.id = ?", id).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find invitation: %w", err)
	}
	return model.toDTO(), nil
}

func (r *InvitationRepository) ReplaceToken(ctx context.Context, id, tokenHash string, expiresAt time.Time) error {
	return r.updatePending(ctx, id, map[string]any{"token_hash": tokenHash, "expires_at": expiresAt})
}

func (r *InvitationRepository) Revoke(ctx context.Context, id string, at time.Time) error {
	return r.updatePending(ctx, id, map[string]any{"revoked_at": at})
}

func (r *InvitationRepository) updatePending(ctx context.Context, id string, updates map[string]any) error {
	if uuid.Validate(id) != nil {
		return domainError.ErrInvitationNotFound
	}
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.Model(&invitationModel{}).Where("id = ? AND accepted_at IS NULL AND revoked_at IS NULL", id).Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("update invitation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrInvitationNotFound
	}
	return nil
}

func (r *InvitationRepository) FindByTokenHash(ctx context.Context, tokenHash string, forUpdate bool) (*dto.Invitation, error) {
	q := postgres.GetTxOrDB(ctx, r.db).Model(&invitationModel{})
	if forUpdate {
		q = q.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	var model invitationModel
	err := q.Where("token_hash = ?", tokenHash).First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrInvitationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find invitation by token: %w", err)
	}
	return model.toDTO(), nil
}

func (r *InvitationRepository) MarkAccepted(ctx context.Context, id string, at time.Time) error {
	result := postgres.GetTxOrDB(ctx, r.db).Model(&invitationModel{}).
		Where("id = ? AND accepted_at IS NULL", id).Update("accepted_at", at)
	if result.Error != nil {
		return fmt.Errorf("accept invitation: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrInvitationInvalidOrExpired
	}
	return nil
}

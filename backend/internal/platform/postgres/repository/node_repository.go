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
	"gorm.io/gorm/clause"
)

const activeEdgePerOrgIndex = "uq_nodes_one_active_edge_per_org"

type nodeModel struct {
	postgres.TenantOwned
	ID         string     `gorm:"column:id;primaryKey"`
	Name       string     `gorm:"column:name"`
	Kind       string     `gorm:"column:kind"`
	KeyHash    *string    `gorm:"column:key_hash"`
	KeyPrefix  *string    `gorm:"column:key_prefix"`
	EnrolledAt *time.Time `gorm:"column:enrolled_at"`
	RevokedAt  *time.Time `gorm:"column:revoked_at"`
	LastSeenAt *time.Time `gorm:"column:last_seen_at"`
}

func (nodeModel) TableName() string {
	return "nodes"
}

func (m *nodeModel) toDTO() *dto.Node {
	return &dto.Node{
		ID:             m.ID,
		Name:           m.Name,
		Kind:           m.Kind,
		OrganizationID: m.OrganizationID,
		HasKey:         m.KeyHash != nil,
		EnrolledAt:     m.EnrolledAt,
		RevokedAt:      m.RevokedAt,
		LastSeenAt:     m.LastSeenAt,
	}
}

type nodeAuthRow struct {
	ID                 string     `gorm:"column:id"`
	Name               string     `gorm:"column:name"`
	Kind               string     `gorm:"column:kind"`
	OrganizationID     string     `gorm:"column:organization_id"`
	KeyHash            *string    `gorm:"column:key_hash"`
	EnrolledAt         *time.Time `gorm:"column:enrolled_at"`
	RevokedAt          *time.Time `gorm:"column:revoked_at"`
	LastSeenAt         *time.Time `gorm:"column:last_seen_at"`
	OrganizationStatus string     `gorm:"column:organization_status"`
}

type NodeRepository struct {
	db *gorm.DB
}

func NewNodeRepository(db *gorm.DB) ports.NodeRepository {
	return &NodeRepository{db: db}
}

// FindForAuth is how a calling node's organization is learned, so it runs before there is one
// and can't go through TenantDB.
func (r *NodeRepository) FindForAuth(ctx context.Context, id string) (*dto.NodeAuthRecord, error) {
	if uuid.Validate(id) != nil {
		return nil, domainError.ErrNodeNotFound
	}
	var row nodeAuthRow
	result := postgres.GetTxOrDB(ctx, r.db).WithContext(ctx).Raw(`
		SELECT n.id, n.name, n.kind, n.organization_id, n.key_hash,
		       n.enrolled_at, n.revoked_at, n.last_seen_at, o.status AS organization_status
		FROM nodes n
		JOIN organizations o ON o.id = n.organization_id
		WHERE n.id = ?`, id).Scan(&row)
	if result.Error != nil {
		return nil, fmt.Errorf("find node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return nil, domainError.ErrNodeNotFound
	}
	record := &dto.NodeAuthRecord{
		Node: dto.Node{
			ID: row.ID, Name: row.Name, Kind: row.Kind, OrganizationID: row.OrganizationID, HasKey: row.KeyHash != nil,
			EnrolledAt: row.EnrolledAt, RevokedAt: row.RevokedAt, LastSeenAt: row.LastSeenAt,
		},
		OrganizationStatus: row.OrganizationStatus,
	}
	if row.KeyHash != nil {
		record.KeyHash = *row.KeyHash
	}
	return record, nil
}

func (r *NodeRepository) FindActiveEdgeForUpdate(ctx context.Context) (*dto.Node, error) {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return nil, err
	}
	var model nodeModel
	err = db.WithContext(ctx).Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("kind = 'edge' AND revoked_at IS NULL").First(&model).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domainError.ErrNodeNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("find active edge node: %w", err)
	}
	return model.toDTO(), nil
}

func (r *NodeRepository) Insert(ctx context.Context, node *dto.Node, keyHash, keyPrefix string) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	model := &nodeModel{
		ID:         node.ID,
		Name:       node.Name,
		Kind:       node.Kind,
		KeyHash:    &keyHash,
		KeyPrefix:  &keyPrefix,
		EnrolledAt: node.EnrolledAt,
	}
	if err = db.WithContext(ctx).Create(model).Error; err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == pgUniqueViolation && pgErr.ConstraintName == activeEdgePerOrgIndex {
			return domainError.ErrBoxAlreadyEnrolled
		}
		return fmt.Errorf("insert node: %w", err)
	}
	node.OrganizationID = model.OrganizationID
	return nil
}

func (r *NodeRepository) SetKey(ctx context.Context, id, name, keyHash, keyPrefix string, enrolledAt time.Time) error {
	return r.update(ctx, id, map[string]any{
		"name": name, "key_hash": keyHash, "key_prefix": keyPrefix, "enrolled_at": enrolledAt,
	})
}

func (r *NodeRepository) Revoke(ctx context.Context, id string, at time.Time) error {
	return r.update(ctx, id, map[string]any{"revoked_at": at})
}

func (r *NodeRepository) update(ctx context.Context, id string, columns map[string]any) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	result := db.WithContext(ctx).Model(&nodeModel{}).Where("id = ?", id).Updates(columns)
	if result.Error != nil {
		return fmt.Errorf("update node: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return domainError.ErrNodeNotFound
	}
	return nil
}

// Runs before the request has an organization in ctx, so it filters by the authenticated id alone.
func (r *NodeRepository) TouchLastSeen(ctx context.Context, id string, at time.Time, minInterval time.Duration) error {
	err := postgres.GetTxOrDB(ctx, r.db).WithContext(ctx).Exec(`
		UPDATE nodes SET last_seen_at = ?
		WHERE id = ? AND (last_seen_at IS NULL OR last_seen_at < ?)`,
		at, id, at.Add(-minInterval)).Error
	if err != nil {
		return fmt.Errorf("touch node: %w", err)
	}
	return nil
}

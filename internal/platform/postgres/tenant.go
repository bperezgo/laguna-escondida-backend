package postgres

import (
	"context"

	"laguna-escondida/backend/internal/domain/tenant"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// TenantDB is GetTxOrDB filtered to the context's organization, the only way repositories
// reach tenant tables. Without an organization it errors rather than read every one. The
// filter qualifies the statement's own table, so joins stay unambiguous; Raw and Exec ignore
// it and must bind the org themselves (see OrgID).
func TenantDB(ctx context.Context, db *gorm.DB) (*gorm.DB, error) {
	orgID, err := tenant.RequireOrg(ctx)
	if err != nil {
		return nil, err
	}
	scoped := GetTxOrDB(ctx, db).Where(clause.Eq{
		Column: clause.Column{Table: clause.CurrentTable, Name: "organization_id"},
		Value:  orgID,
	})
	// A fresh session per call, so statements chained off the result each start from the
	// filter instead of piling up each other's conditions.
	return scoped.Session(&gorm.Session{}), nil
}

// OrgID is the context's organization, for raw SQL that TenantDB's filter can't reach.
func OrgID(ctx context.Context) (string, error) {
	return tenant.RequireOrg(ctx)
}

// TenantOwned is embedded in every tenant model. It stamps organization_id from the
// statement's context on every save, so no create can take it from a DTO or forget it.
type TenantOwned struct {
	OrganizationID string `gorm:"column:organization_id"`
}

func (t *TenantOwned) BeforeSave(tx *gorm.DB) error {
	orgID, err := tenant.RequireOrg(tx.Statement.Context)
	if err != nil {
		return err
	}
	t.OrganizationID = orgID
	return nil
}

// SameOrganization guards an upsert's DO UPDATE: a key that collides with another
// organization's row leaves that row untouched instead of overwriting it. table is a
// compile-time constant, never input.
func SameOrganization(table string) clause.Where {
	return clause.Where{Exprs: []clause.Expression{
		clause.Expr{SQL: table + ".organization_id = EXCLUDED.organization_id"},
	}}
}

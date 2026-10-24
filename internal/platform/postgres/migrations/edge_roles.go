package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

// PosAppRoleVersion creates the pos_app group role, which needs CREATEROLE. A box migrates as
// the database owner, so its provisioning creates the role first (edge repo, create-db-roles.ps1).
const PosAppRoleVersion = 65

// ErrPosAppRoleMissing stops the box before migration 65 would fail and leave the schema dirty.
var ErrPosAppRoleMissing = errors.New("role pos_app does not exist and the migration user cannot create it: " +
	"create it as the postgres superuser (laguna-escondida-edge: scripts\\create-db-roles.ps1)")

func checkPosAppRole(ctx context.Context, db *sql.DB) error {
	var ready bool
	err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'pos_app')
		OR (SELECT rolsuper OR rolcreaterole FROM pg_roles WHERE rolname = current_user)`).Scan(&ready)
	if err != nil {
		return fmt.Errorf("check the pos_app role: %w", err)
	}
	if !ready {
		return ErrPosAppRoleMissing
	}
	return nil
}

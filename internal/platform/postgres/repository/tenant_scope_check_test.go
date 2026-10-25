package repository

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// globalTableFiles reach tables that belong to no organization (or name it explicitly), so
// they may use the unscoped database handle.
var globalTableFiles = map[string]bool{
	"database.go":                       true,
	"einvoicing_settings_repository.go": true,
	"invoice_sequence.go":               true,
	"node_repository.go":                true,
	"organization_repository.go":        true,
	"role_repository.go":                true,
}

// unscopedByDesign are single functions in tenant files that must cross organizations: cloud
// sign-in learns the organization from the email.
var unscopedByDesign = map[string]bool{
	"user_repository.go FindByEmail": true,
}

// unscopedTenantAccess reports the functions in src that reach the database without the
// organization: GetTxOrDB or InTx without binding postgres.OrgID for raw SQL, or calling a
// method on the receiver's db directly. Everything else must go through TenantDB or TenantTx.
func unscopedTenantAccess(filename, src string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filename, src, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		var unscoped, bindsOrg bool
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "postgres" {
				switch sel.Sel.Name {
				case "GetTxOrDB", "InTx":
					unscoped = true
				case "OrgID":
					bindsOrg = true
				}
			}
			// r.db.Where(...): the receiver's handle used directly.
			if inner, ok := sel.X.(*ast.SelectorExpr); ok && inner.Sel.Name == "db" {
				if _, isIdent := inner.X.(*ast.Ident); isIdent {
					unscoped = true
				}
			}
			return true
		})
		if unscoped && !bindsOrg {
			findings = append(findings, fmt.Sprintf("%s: %s", fset.Position(fn.Pos()), fn.Name.Name))
		}
	}
	return findings, nil
}

func TestTenantScopeCheck_RepositoriesAreScoped(t *testing.T) {
	pkgDir := os.DirFS(".")
	paths, err := fs.Glob(pkgDir, "*.go")
	require.NoError(t, err)
	var findings []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || globalTableFiles[path] {
			continue
		}
		src, err := fs.ReadFile(pkgDir, path)
		require.NoError(t, err)
		found, err := unscopedTenantAccess(path, string(src))
		require.NoError(t, err)
		for _, finding := range found {
			if !unscopedByDesign[path+" "+finding[strings.LastIndex(finding, " ")+1:]] {
				findings = append(findings, finding)
			}
		}
	}
	sort.Strings(findings)
	assert.Empty(t, findings, "use postgres.TenantDB / TenantTx, or bind postgres.OrgID in raw SQL")
}

func TestTenantScopeCheck_FlagsUnscopedAccess(t *testing.T) {
	const src = `package repository

func (r *Repo) Scoped(ctx context.Context) error {
	db, err := postgres.TenantDB(ctx, r.db)
	if err != nil {
		return err
	}
	return db.Find(&x).Error
}

func (r *Repo) RawWithOrg(ctx context.Context) error {
	orgID, err := postgres.OrgID(ctx)
	if err != nil {
		return err
	}
	return postgres.GetTxOrDB(ctx, r.db).Exec("DELETE FROM x WHERE organization_id = ?", orgID).Error
}

func (r *Repo) Unscoped(ctx context.Context) error {
	return postgres.GetTxOrDB(ctx, r.db).Find(&x).Error
}

func (r *Repo) UnscopedTx(ctx context.Context) error {
	return postgres.InTx(ctx, r.db, func(tx *gorm.DB) error { return tx.Find(&x).Error })
}

func (r *Repo) Direct(ctx context.Context) error {
	return r.db.WithContext(ctx).Find(&x).Error
}
`
	findings, err := unscopedTenantAccess("fixture.go", src)

	require.NoError(t, err)
	require.Len(t, findings, 3)
	assert.Contains(t, findings[0], "Unscoped")
	assert.Contains(t, findings[1], "UnscopedTx")
	assert.Contains(t, findings[2], "Direct")
}

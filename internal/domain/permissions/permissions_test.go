package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAllPermissions(t *testing.T) {
	perms := AllPermissions()

	assert.NotEmpty(t, perms)
	assert.Contains(t, perms, OrdersRead)
	assert.Contains(t, perms, OrdersCreate)
	assert.Contains(t, perms, ProductsRead)
	assert.Contains(t, perms, ExpensesRead)
}

func TestPermissionStrings(t *testing.T) {
	perms := []Permission{OrdersRead, ProductsCreate, ExpensesDelete}
	result := PermissionStrings(perms)

	assert.Len(t, result, 3)
	assert.Equal(t, "orders:read", result[0])
	assert.Equal(t, "products:create", result[1])
	assert.Equal(t, "expenses:delete", result[2])
}

func TestPermissionStrings_Empty(t *testing.T) {
	result := PermissionStrings([]Permission{})

	assert.Empty(t, result)
}

func TestPermissionConstants(t *testing.T) {
	tests := []struct {
		name     string
		perm     Permission
		expected string
	}{
		{"OrdersRead", OrdersRead, "orders:read"},
		{"OrdersCreate", OrdersCreate, "orders:create"},
		{"ProductsRead", ProductsRead, "products:read"},
		{"ExpensesCreate", ExpensesCreate, "expenses:create"},
		{"InvoicesExport", InvoicesExport, "invoices:export"},
		{"StockUpdate", StockUpdate, "stock:update"},
		{"SuppliersDelete", SuppliersDelete, "suppliers:delete"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, string(tt.perm))
		})
	}
}

func TestOrdersCompleteProductConstant(t *testing.T) {
	assert.Equal(t, "orders:complete-product", string(OrdersCompleteProduct))
	assert.Contains(t, AllPermissions(), OrdersCompleteProduct)
}

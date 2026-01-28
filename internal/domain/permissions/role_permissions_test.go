package permissions

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetPermissionsForRoles_SingleRole(t *testing.T) {
	tests := []struct {
		name             string
		roleID           int
		expectedContains []Permission
		expectedNotContain []Permission
	}{
		{
			name:             "Waitress permissions",
			roleID:           RoleWaitress,
			expectedContains: []Permission{OrdersRead, OrdersCreate, ProductsRead, CommandsRead},
			expectedNotContain: []Permission{ExpensesRead, SuppliersCreate, StockCreate},
		},
		{
			name:             "Cooker permissions",
			roleID:           RoleCooker,
			expectedContains: []Permission{OrdersRead, ProductsRead, CommandsRead, CommandsUpdate},
			expectedNotContain: []Permission{OrdersCreate, ExpensesRead, SuppliersCreate},
		},
		{
			name:             "Accountant permissions",
			roleID:           RoleAccountant,
			expectedContains: []Permission{ExpensesRead, ExpensesCreate, PurchaseEntriesRead, InvoicesRead},
			expectedNotContain: []Permission{OrdersCreate, ProductsCreate, SuppliersCreate},
		},
		{
			name:             "Manager permissions",
			roleID:           RoleManager,
			expectedContains: []Permission{OrdersRead, OrdersCreate, ProductsCreate, ExpensesRead, SuppliersRead},
			expectedNotContain: []Permission{UsersCreate},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			perms := GetPermissionsForRoles([]int{tt.roleID})

			for _, expected := range tt.expectedContains {
				assert.Contains(t, perms, expected, "Role %d should have permission %s", tt.roleID, expected)
			}

			for _, notExpected := range tt.expectedNotContain {
				assert.NotContains(t, perms, notExpected, "Role %d should NOT have permission %s", tt.roleID, notExpected)
			}
		})
	}
}

func TestGetPermissionsForRoles_Admin(t *testing.T) {
	perms := GetPermissionsForRoles([]int{RoleAdmin})
	allPerms := AllPermissions()

	assert.Len(t, perms, len(allPerms), "Admin should have all permissions")
}

func TestGetPermissionsForRoles_MultipleRoles(t *testing.T) {
	perms := GetPermissionsForRoles([]int{RoleWaitress, RoleAccountant})

	assert.Contains(t, perms, OrdersRead, "Should have waitress permission")
	assert.Contains(t, perms, OrdersCreate, "Should have waitress permission")
	assert.Contains(t, perms, ExpensesRead, "Should have accountant permission")
	assert.Contains(t, perms, PurchaseEntriesCreate, "Should have accountant permission")
}

func TestGetPermissionsForRoles_UnknownRole(t *testing.T) {
	perms := GetPermissionsForRoles([]int{999})

	assert.Empty(t, perms, "Unknown role should return empty permissions")
}

func TestGetPermissionsForRoles_EmptyRoles(t *testing.T) {
	perms := GetPermissionsForRoles([]int{})

	assert.Empty(t, perms, "Empty roles should return empty permissions")
}

func TestGetPermissionsForRoles_UniquePermissions(t *testing.T) {
	perms := GetPermissionsForRoles([]int{RoleWaitress, RoleManager})

	permSet := make(map[Permission]bool)
	for _, p := range perms {
		assert.False(t, permSet[p], "Permission %s should not be duplicated", p)
		permSet[p] = true
	}
}

func TestHasPermission_Success(t *testing.T) {
	tests := []struct {
		name       string
		roleIDs    []int
		permission Permission
		expected   bool
	}{
		{
			name:       "Waitress has OrdersRead",
			roleIDs:    []int{RoleWaitress},
			permission: OrdersRead,
			expected:   true,
		},
		{
			name:       "Waitress does not have ExpensesRead",
			roleIDs:    []int{RoleWaitress},
			permission: ExpensesRead,
			expected:   false,
		},
		{
			name:       "Admin has ExpensesDelete",
			roleIDs:    []int{RoleAdmin},
			permission: ExpensesDelete,
			expected:   true,
		},
		{
			name:       "Multiple roles - one has permission",
			roleIDs:    []int{RoleCooker, RoleAccountant},
			permission: ExpensesRead,
			expected:   true,
		},
		{
			name:       "Empty roles",
			roleIDs:    []int{},
			permission: OrdersRead,
			expected:   false,
		},
		{
			name:       "Unknown role",
			roleIDs:    []int{999},
			permission: OrdersRead,
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasPermission(tt.roleIDs, tt.permission)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestHasAnyPermission(t *testing.T) {
	tests := []struct {
		name        string
		roleIDs     []int
		permissions []Permission
		expected    bool
	}{
		{
			name:        "Has first permission",
			roleIDs:     []int{RoleWaitress},
			permissions: []Permission{OrdersRead, ExpensesRead},
			expected:    true,
		},
		{
			name:        "Has second permission",
			roleIDs:     []int{RoleAccountant},
			permissions: []Permission{OrdersCreate, ExpensesRead},
			expected:    true,
		},
		{
			name:        "Has none of the permissions",
			roleIDs:     []int{RoleCooker},
			permissions: []Permission{ExpensesCreate, SuppliersCreate},
			expected:    false,
		},
		{
			name:        "Empty permissions list",
			roleIDs:     []int{RoleAdmin},
			permissions: []Permission{},
			expected:    false,
		},
		{
			name:        "Empty roles",
			roleIDs:     []int{},
			permissions: []Permission{OrdersRead},
			expected:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := HasAnyPermission(tt.roleIDs, tt.permissions)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRoleConstants(t *testing.T) {
	assert.Equal(t, 1, RoleWaitress)
	assert.Equal(t, 2, RoleAdmin)
	assert.Equal(t, 3, RoleManager)
	assert.Equal(t, 4, RoleCooker)
	assert.Equal(t, 5, RoleAccountant)
}

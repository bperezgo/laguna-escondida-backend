package permissions

// Role constants matching database IDs from migration 000014
const (
	RoleWaitress   = 1
	RoleAdmin      = 2
	RoleManager    = 3
	RoleCooker     = 4
	RoleAccountant = 5
)

// RolePermissions maps role IDs to their permissions
var RolePermissions = map[int][]Permission{
	RoleWaitress: {
		// Orders - can view and create orders
		OrdersRead, OrdersCreate, OrdersUpdate,
		// Products - can view products
		ProductsRead,
		// Commands - can view and update commands
		CommandsRead, CommandsUpdate,
		// Bill Owners - can view
		BillOwnersRead,
		// SSE - can receive real-time updates
		SSECommandsRead, SSECommandItemsRead,
	},
	RoleAdmin: AllPermissions(),
	RoleManager: {
		// Orders - full access
		OrdersRead, OrdersCreate, OrdersUpdate, OrdersDelete, OrdersPay,
		// Products - full access
		ProductsRead, ProductsCreate, ProductsUpdate, ProductsDelete,
		// Stock - full access
		StockRead, StockCreate, StockUpdate, StockDelete,
		// Invoices - full access
		InvoicesRead, InvoicesCreate, InvoicesExport,
		// Commands - full access
		CommandsRead, CommandsUpdate,
		// Bill Owners - can view
		BillOwnersRead,
		// Suppliers - full access
		SuppliersRead, SuppliersCreate, SuppliersUpdate, SuppliersDelete,
		// Supplier Catalog - full access
		SupplierCatalogRead, SupplierCatalogCreate, SupplierCatalogUpdate, SupplierCatalogDelete,
		// Purchase Entries - full access
		PurchaseEntriesRead, PurchaseEntriesCreate, PurchaseEntriesUpload,
		// Expense Categories - full access
		ExpenseCategoriesRead, ExpenseCategoriesCreate, ExpenseCategoriesUpdate,
		// Expenses - full access
		ExpensesRead, ExpensesCreate, ExpensesUpdate, ExpensesDelete, ExpensesUpload,
		// Users - can view
		UsersRead,
		// SSE - can receive real-time updates
		SSECommandsRead, SSECommandItemsRead,
	},
	RoleCooker: {
		// Orders - can view orders
		OrdersRead,
		// Products - can view products
		ProductsRead,
		// Commands - can view and update commands (mark as done)
		CommandsRead, CommandsUpdate,
		// SSE - can receive real-time updates
		SSECommandsRead, SSECommandItemsRead,
	},
	RoleAccountant: {
		// Invoices - can view and export
		InvoicesRead, InvoicesExport,
		// Suppliers - can view
		SuppliersRead,
		// Purchase Entries - full access
		PurchaseEntriesRead, PurchaseEntriesCreate, PurchaseEntriesUpload,
		// Expense Categories - can view and create
		ExpenseCategoriesRead, ExpenseCategoriesCreate, ExpenseCategoriesUpdate,
		// Expenses - full access
		ExpensesRead, ExpensesCreate, ExpensesUpdate, ExpensesDelete, ExpensesUpload,
	},
}

// GetPermissionsForRoles returns unique permissions for given role IDs
func GetPermissionsForRoles(roleIDs []int) []Permission {
	permissionSet := make(map[Permission]bool)
	for _, roleID := range roleIDs {
		if perms, ok := RolePermissions[roleID]; ok {
			for _, perm := range perms {
				permissionSet[perm] = true
			}
		}
	}

	result := make([]Permission, 0, len(permissionSet))
	for perm := range permissionSet {
		result = append(result, perm)
	}
	return result
}

// HasPermission checks if the given roles have a specific permission
func HasPermission(roleIDs []int, required Permission) bool {
	for _, roleID := range roleIDs {
		if perms, ok := RolePermissions[roleID]; ok {
			for _, perm := range perms {
				if perm == required {
					return true
				}
			}
		}
	}
	return false
}

// HasAnyPermission checks if the given roles have at least one of the required permissions
func HasAnyPermission(roleIDs []int, required []Permission) bool {
	for _, perm := range required {
		if HasPermission(roleIDs, perm) {
			return true
		}
	}
	return false
}

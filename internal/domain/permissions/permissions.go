package permissions

// Permission represents a specific action that can be performed on a resource
type Permission string

// Order permissions
const (
	OrdersRead   Permission = "orders:read"
	OrdersCreate Permission = "orders:create"
	OrdersUpdate Permission = "orders:update"
	OrdersDelete Permission = "orders:delete"
)

// Product permissions
const (
	ProductsRead   Permission = "products:read"
	ProductsCreate Permission = "products:create"
	ProductsUpdate Permission = "products:update"
	ProductsDelete Permission = "products:delete"
)

// Stock permissions
const (
	StockRead   Permission = "stock:read"
	StockCreate Permission = "stock:create"
	StockUpdate Permission = "stock:update"
	StockDelete Permission = "stock:delete"
)

// Invoice permissions
const (
	InvoicesRead   Permission = "invoices:read"
	InvoicesCreate Permission = "invoices:create"
	InvoicesExport Permission = "invoices:export"
)

// Command permissions
const (
	CommandsRead   Permission = "commands:read"
	CommandsUpdate Permission = "commands:update"
)

// Bill Owner permissions
const (
	BillOwnersRead Permission = "bill-owners:read"
)

// Supplier permissions
const (
	SuppliersRead   Permission = "suppliers:read"
	SuppliersCreate Permission = "suppliers:create"
	SuppliersUpdate Permission = "suppliers:update"
	SuppliersDelete Permission = "suppliers:delete"
)

// Supplier Catalog permissions
const (
	SupplierCatalogRead   Permission = "supplier-catalog:read"
	SupplierCatalogCreate Permission = "supplier-catalog:create"
	SupplierCatalogUpdate Permission = "supplier-catalog:update"
	SupplierCatalogDelete Permission = "supplier-catalog:delete"
)

// Purchase Entry permissions
const (
	PurchaseEntriesRead   Permission = "purchase-entries:read"
	PurchaseEntriesCreate Permission = "purchase-entries:create"
	PurchaseEntriesUpload Permission = "purchase-entries:upload"
)

// Expense Category permissions
const (
	ExpenseCategoriesRead   Permission = "expense-categories:read"
	ExpenseCategoriesCreate Permission = "expense-categories:create"
	ExpenseCategoriesUpdate Permission = "expense-categories:update"
)

// Expense permissions
const (
	ExpensesRead   Permission = "expenses:read"
	ExpensesCreate Permission = "expenses:create"
	ExpensesUpdate Permission = "expenses:update"
	ExpensesDelete Permission = "expenses:delete"
	ExpensesUpload Permission = "expenses:upload"
)

// User permissions
const (
	UsersRead   Permission = "users:read"
	UsersCreate Permission = "users:create"
)

// SSE permissions
const (
	SSECommandsRead     Permission = "sse:commands:read"
	SSECommandItemsRead Permission = "sse:command-items:read"
)

// AllPermissions returns all defined permissions (useful for admin role)
func AllPermissions() []Permission {
	return []Permission{
		// Orders
		OrdersRead, OrdersCreate, OrdersUpdate, OrdersDelete,
		// Products
		ProductsRead, ProductsCreate, ProductsUpdate, ProductsDelete,
		// Stock
		StockRead, StockCreate, StockUpdate, StockDelete,
		// Invoices
		InvoicesRead, InvoicesCreate, InvoicesExport,
		// Commands
		CommandsRead, CommandsUpdate,
		// Bill Owners
		BillOwnersRead,
		// Suppliers
		SuppliersRead, SuppliersCreate, SuppliersUpdate, SuppliersDelete,
		// Supplier Catalog
		SupplierCatalogRead, SupplierCatalogCreate, SupplierCatalogUpdate, SupplierCatalogDelete,
		// Purchase Entries
		PurchaseEntriesRead, PurchaseEntriesCreate, PurchaseEntriesUpload,
		// Expense Categories
		ExpenseCategoriesRead, ExpenseCategoriesCreate, ExpenseCategoriesUpdate,
		// Expenses
		ExpensesRead, ExpensesCreate, ExpensesUpdate, ExpensesDelete, ExpensesUpload,
		// Users
		UsersRead, UsersCreate,
		// SSE
		SSECommandsRead, SSECommandItemsRead,
	}
}

// PermissionStrings converts a slice of Permission to a slice of strings
func PermissionStrings(perms []Permission) []string {
	result := make([]string, len(perms))
	for i, p := range perms {
		result[i] = string(p)
	}
	return result
}

/**
 * Permission constants matching the backend PBAC system
 */

export const PERMISSIONS = {
  // Orders
  ORDERS_READ: 'orders:read',
  ORDERS_CREATE: 'orders:create',
  ORDERS_UPDATE: 'orders:update',
  ORDERS_DELETE: 'orders:delete',

  // Products
  PRODUCTS_READ: 'products:read',
  PRODUCTS_CREATE: 'products:create',
  PRODUCTS_UPDATE: 'products:update',
  PRODUCTS_DELETE: 'products:delete',

  // Stock
  STOCK_READ: 'stock:read',
  STOCK_CREATE: 'stock:create',
  STOCK_UPDATE: 'stock:update',
  STOCK_DELETE: 'stock:delete',

  // Invoices
  INVOICES_READ: 'invoices:read',
  INVOICES_CREATE: 'invoices:create',
  INVOICES_EXPORT: 'invoices:export',

  // Commands
  COMMANDS_READ: 'commands:read',
  COMMANDS_UPDATE: 'commands:update',

  // Bill Owners
  BILL_OWNERS_READ: 'bill-owners:read',

  // Suppliers
  SUPPLIERS_READ: 'suppliers:read',
  SUPPLIERS_CREATE: 'suppliers:create',
  SUPPLIERS_UPDATE: 'suppliers:update',
  SUPPLIERS_DELETE: 'suppliers:delete',

  // Supplier Catalog
  SUPPLIER_CATALOG_READ: 'supplier-catalog:read',
  SUPPLIER_CATALOG_CREATE: 'supplier-catalog:create',
  SUPPLIER_CATALOG_UPDATE: 'supplier-catalog:update',
  SUPPLIER_CATALOG_DELETE: 'supplier-catalog:delete',

  // Purchase Entries
  PURCHASE_ENTRIES_READ: 'purchase-entries:read',
  PURCHASE_ENTRIES_CREATE: 'purchase-entries:create',
  PURCHASE_ENTRIES_UPLOAD: 'purchase-entries:upload',

  // Expense Categories
  EXPENSE_CATEGORIES_READ: 'expense-categories:read',
  EXPENSE_CATEGORIES_CREATE: 'expense-categories:create',
  EXPENSE_CATEGORIES_UPDATE: 'expense-categories:update',

  // Expenses
  EXPENSES_READ: 'expenses:read',
  EXPENSES_CREATE: 'expenses:create',
  EXPENSES_UPDATE: 'expenses:update',
  EXPENSES_DELETE: 'expenses:delete',
  EXPENSES_UPLOAD: 'expenses:upload',

  // Users
  USERS_READ: 'users:read',
  USERS_CREATE: 'users:create',

  // SSE (Real-time)
  SSE_COMMANDS_READ: 'sse:commands:read',
  SSE_COMMAND_ITEMS_READ: 'sse:command-items:read',
} as const;

export type Permission = (typeof PERMISSIONS)[keyof typeof PERMISSIONS];

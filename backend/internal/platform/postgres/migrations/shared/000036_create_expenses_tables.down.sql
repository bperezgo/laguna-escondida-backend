-- Drop indexes
DROP INDEX IF EXISTS idx_expenses_category_id;
DROP INDEX IF EXISTS idx_expenses_supplier_id;
DROP INDEX IF EXISTS idx_expenses_expense_date;
DROP INDEX IF EXISTS idx_expense_categories_code;
DROP INDEX IF EXISTS idx_expense_categories_is_active;

-- Drop tables
DROP TABLE IF EXISTS expenses;
DROP TABLE IF EXISTS expense_categories;

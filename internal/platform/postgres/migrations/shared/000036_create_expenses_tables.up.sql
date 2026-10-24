-- Create expense_categories table
CREATE TABLE expense_categories (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    code VARCHAR(50) NOT NULL UNIQUE,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    is_active BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create expenses table
CREATE TABLE expenses (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    category_id UUID NOT NULL REFERENCES expense_categories(id),
    supplier_id UUID REFERENCES suppliers(id),
    amount NUMERIC(19, 4) NOT NULL,
    description VARCHAR(500) NOT NULL,
    expense_date TIMESTAMP NOT NULL,
    reference VARCHAR(255),
    notes TEXT,
    pdf_storage_path TEXT,
    xml_storage_path TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create indexes for faster lookups
CREATE INDEX idx_expenses_category_id ON expenses(category_id);
CREATE INDEX idx_expenses_supplier_id ON expenses(supplier_id);
CREATE INDEX idx_expenses_expense_date ON expenses(expense_date);
CREATE INDEX idx_expense_categories_code ON expense_categories(code);
CREATE INDEX idx_expense_categories_is_active ON expense_categories(is_active);

-- Seed initial expense categories
INSERT INTO expense_categories (code, name, description) VALUES
    ('indirect_cost', 'Indirect Cost', 'Indirect costs like cleaning supplies, paper, soap for restrooms'),
    ('expense', 'General Expense', 'General operational expenses'),
    ('investment', 'Investment', 'Capital investments like equipment, materials, improvements'),
    ('rent', 'Rent', 'Rental payments for buildings and spaces'),
    ('service', 'Service', 'Utility services like electricity, water, internet');

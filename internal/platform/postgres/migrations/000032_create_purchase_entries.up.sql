-- Create purchase_entries table
CREATE TABLE purchase_entries (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    total_amount NUMERIC(19, 4) NOT NULL,
    invoice_reference VARCHAR(255),
    entry_date TIMESTAMP NOT NULL,
    notes TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create purchase_entry_items table
CREATE TABLE purchase_entry_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    purchase_entry_id UUID NOT NULL REFERENCES purchase_entries(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    quantity NUMERIC(19, 4) NOT NULL,
    unit_cost NUMERIC(19, 4) NOT NULL,
    total_cost NUMERIC(19, 4) NOT NULL
);

-- Create indexes for faster lookups
CREATE INDEX idx_purchase_entries_supplier_id ON purchase_entries(supplier_id);
CREATE INDEX idx_purchase_entries_entry_date ON purchase_entries(entry_date);
CREATE INDEX idx_purchase_entry_items_entry_id ON purchase_entry_items(purchase_entry_id);
CREATE INDEX idx_purchase_entry_items_product_id ON purchase_entry_items(product_id);

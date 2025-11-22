-- Remove columns vat, ico, tip, document_url from open_bills table
ALTER TABLE open_bills DROP COLUMN IF EXISTS vat;
ALTER TABLE open_bills DROP COLUMN IF EXISTS ico;
ALTER TABLE open_bills DROP COLUMN IF EXISTS tip;
ALTER TABLE open_bills DROP COLUMN IF EXISTS document_url;

-- Rename total_price to total_amount in open_bills table
ALTER TABLE open_bills RENAME COLUMN total_price TO total_amount;

-- Add created_by column to open_bills table (references users table)
ALTER TABLE open_bills ADD COLUMN created_by UUID REFERENCES users(id) ON DELETE SET NULL;

-- Add descriptor column to open_bills table
ALTER TABLE open_bills ADD COLUMN descriptor TEXT;

-- Add notes column to open_bills_products table
ALTER TABLE open_bills_products ADD COLUMN notes TEXT;

-- Create product_preparation_responsibilities table
CREATE TABLE IF NOT EXISTS product_preparation_responsibilities (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    area VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    UNIQUE(product_id, area)
);


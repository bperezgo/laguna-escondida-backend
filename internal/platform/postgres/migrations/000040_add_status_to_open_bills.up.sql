-- Add status column to open_bills table
ALTER TABLE open_bills ADD COLUMN status VARCHAR(50) NOT NULL DEFAULT 'created';

-- Backfill existing open_bills with derived status based on their products
UPDATE open_bills SET status = (
    CASE
        -- No products: created
        WHEN NOT EXISTS (
            SELECT 1 FROM open_bills_products 
            WHERE open_bills_products.open_bill_id = open_bills.id 
            AND open_bills_products.deleted_at IS NULL
        ) THEN 'created'
        -- All products finalized (completed or cancelled) AND at least one completed: completed
        WHEN NOT EXISTS (
            SELECT 1 FROM open_bills_products 
            WHERE open_bills_products.open_bill_id = open_bills.id 
            AND open_bills_products.deleted_at IS NULL
            AND open_bills_products.status NOT IN ('completed', 'cancelled')
        ) AND EXISTS (
            SELECT 1 FROM open_bills_products 
            WHERE open_bills_products.open_bill_id = open_bills.id 
            AND open_bills_products.deleted_at IS NULL
            AND open_bills_products.status = 'completed'
        ) THEN 'completed'
        -- All products cancelled: cancelled
        WHEN NOT EXISTS (
            SELECT 1 FROM open_bills_products 
            WHERE open_bills_products.open_bill_id = open_bills.id 
            AND open_bills_products.deleted_at IS NULL
            AND open_bills_products.status != 'cancelled'
        ) THEN 'cancelled'
        -- Otherwise: created
        ELSE 'created'
    END
);

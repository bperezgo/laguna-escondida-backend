-- Migration: remove_unique_constraint_open_bills_products (DOWN)
-- Version: 000019
-- Description: Re-add unique constraint on (open_bill_id, product_id)

-- Note: This will fail if there are duplicate (open_bill_id, product_id) combinations
ALTER TABLE open_bills_products 
ADD CONSTRAINT open_bills_products_open_bill_id_product_id_key 
UNIQUE(open_bill_id, product_id);


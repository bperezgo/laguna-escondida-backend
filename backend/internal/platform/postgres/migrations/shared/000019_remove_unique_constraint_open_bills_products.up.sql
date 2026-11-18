-- Migration: remove_unique_constraint_open_bills_products
-- Version: 000019
-- Description: Remove unique constraint on (open_bill_id, product_id) to allow same product with different notes

ALTER TABLE open_bills_products 
DROP CONSTRAINT IF EXISTS open_bills_products_open_bill_id_product_id_key;


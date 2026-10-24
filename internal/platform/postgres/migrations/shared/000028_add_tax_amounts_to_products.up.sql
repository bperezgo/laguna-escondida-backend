-- Add vat_amount and ico_amount columns to products table
ALTER TABLE products
ADD COLUMN vat_amount NUMERIC(19,4) NOT NULL DEFAULT 0,
ADD COLUMN ico_amount NUMERIC(19,4) NOT NULL DEFAULT 0;

-- Migrate existing data: calculate tax amounts from total_price_with_taxes
-- Formula: taxAmount = totalPriceWithTaxes * taxPercentage / (1 + vatPercentage + icoPercentage)
-- unitPrice = totalPriceWithTaxes - vatAmount - icoAmount
UPDATE products
SET 
    vat_amount = ROUND(total_price_with_taxes * vat / (1 + vat + ico), 2),
    ico_amount = ROUND(total_price_with_taxes * ico / (1 + vat + ico), 2),
    unit_price = total_price_with_taxes - ROUND(total_price_with_taxes * vat / (1 + vat + ico), 2) - ROUND(total_price_with_taxes * ico / (1 + vat + ico), 2)
WHERE deleted_at IS NULL;

-- Remove default constraint after data migration
ALTER TABLE products
ALTER COLUMN vat_amount DROP DEFAULT,
ALTER COLUMN ico_amount DROP DEFAULT;

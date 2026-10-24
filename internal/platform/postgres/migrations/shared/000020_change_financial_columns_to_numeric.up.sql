-- Change financial columns from double precision to NUMERIC(19, 4) for exact decimal precision

-- Bills table
ALTER TABLE bills
  ALTER COLUMN total_amount TYPE NUMERIC(19, 4),
  ALTER COLUMN discount_amount TYPE NUMERIC(19, 4),
  ALTER COLUMN vat TYPE NUMERIC(19, 4),
  ALTER COLUMN ico TYPE NUMERIC(19, 4),
  ALTER COLUMN tip TYPE NUMERIC(19, 4);

-- Products table
ALTER TABLE products
  ALTER COLUMN unit_price TYPE NUMERIC(19, 4),
  ALTER COLUMN vat TYPE NUMERIC(19, 4),
  ALTER COLUMN ico TYPE NUMERIC(19, 4),
  ALTER COLUMN total_price_with_taxes TYPE NUMERIC(19, 4);

-- Open bills table
ALTER TABLE open_bills
  ALTER COLUMN total_amount TYPE NUMERIC(19, 4);


-- Remove identification fields from suppliers table
DROP INDEX IF EXISTS idx_suppliers_identification_number;
ALTER TABLE suppliers DROP COLUMN IF EXISTS identification_number;
ALTER TABLE suppliers DROP COLUMN IF EXISTS identification_type;

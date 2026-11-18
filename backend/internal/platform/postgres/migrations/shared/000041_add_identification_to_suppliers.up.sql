-- Add identification fields to suppliers table
ALTER TABLE suppliers ADD COLUMN identification_type VARCHAR(50);
ALTER TABLE suppliers ADD COLUMN identification_number VARCHAR(50);

-- Create index for identification number lookups
CREATE INDEX idx_suppliers_identification_number ON suppliers(identification_number) WHERE deleted_at IS NULL;

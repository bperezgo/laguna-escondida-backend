-- Restore unit_cost column to supplier_catalog table
ALTER TABLE supplier_catalog ADD COLUMN unit_cost NUMERIC(19, 4) NOT NULL DEFAULT 0;

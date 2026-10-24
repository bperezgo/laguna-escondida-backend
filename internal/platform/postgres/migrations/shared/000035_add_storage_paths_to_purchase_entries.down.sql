-- Remove storage paths from purchase_entries table
ALTER TABLE purchase_entries DROP COLUMN IF EXISTS pdf_storage_path;
ALTER TABLE purchase_entries DROP COLUMN IF EXISTS xml_storage_path;

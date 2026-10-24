-- Remove storage paths from bills table
ALTER TABLE bills DROP COLUMN IF EXISTS pdf_storage_path;
ALTER TABLE bills DROP COLUMN IF EXISTS xml_storage_path;

-- Add storage paths to bills table
ALTER TABLE bills ADD COLUMN pdf_storage_path TEXT;
ALTER TABLE bills ADD COLUMN xml_storage_path TEXT;

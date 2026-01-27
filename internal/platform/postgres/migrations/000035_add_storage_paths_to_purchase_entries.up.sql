-- Add storage paths to purchase_entries table
ALTER TABLE purchase_entries ADD COLUMN pdf_storage_path TEXT;
ALTER TABLE purchase_entries ADD COLUMN xml_storage_path TEXT;

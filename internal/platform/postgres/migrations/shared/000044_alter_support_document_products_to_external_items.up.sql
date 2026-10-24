ALTER TABLE support_document_products DROP CONSTRAINT IF EXISTS support_document_products_product_id_fkey;
ALTER TABLE support_document_products DROP COLUMN IF EXISTS product_id;
ALTER TABLE support_document_products ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE support_document_products ADD COLUMN price NUMERIC(19,4) NOT NULL DEFAULT 0;

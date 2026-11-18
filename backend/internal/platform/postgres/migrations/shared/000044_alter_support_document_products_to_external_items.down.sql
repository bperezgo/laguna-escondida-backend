ALTER TABLE support_document_products DROP COLUMN IF EXISTS price;
ALTER TABLE support_document_products DROP COLUMN IF EXISTS description;
ALTER TABLE support_document_products ADD COLUMN product_id UUID NOT NULL DEFAULT gen_random_uuid();
ALTER TABLE support_document_products ADD CONSTRAINT support_document_products_product_id_fkey FOREIGN KEY (product_id) REFERENCES products(id) ON DELETE CASCADE;

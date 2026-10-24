-- Create supplier_catalog table
CREATE TABLE supplier_catalog (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    supplier_id UUID NOT NULL REFERENCES suppliers(id),
    product_id UUID NOT NULL REFERENCES products(id),
    unit_cost NUMERIC(19, 4) NOT NULL,
    supplier_sku VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP,
    CONSTRAINT uk_supplier_product UNIQUE (supplier_id, product_id)
);

-- Create indexes for faster lookups
CREATE INDEX idx_supplier_catalog_supplier_id ON supplier_catalog(supplier_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_supplier_catalog_product_id ON supplier_catalog(product_id) WHERE deleted_at IS NULL;
CREATE INDEX idx_supplier_catalog_deleted_at ON supplier_catalog(deleted_at);

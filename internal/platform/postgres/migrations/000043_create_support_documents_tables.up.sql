CREATE TABLE IF NOT EXISTS support_documents (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    provider_document_number VARCHAR(255) NOT NULL,
    provider_document_type VARCHAR(10) NOT NULL,
    provider_name VARCHAR(255) NOT NULL,
    provider_email VARCHAR(255) NOT NULL,
    total_amount NUMERIC(19,4) NOT NULL,
    discount_amount NUMERIC(19,4) NOT NULL DEFAULT 0,
    vat NUMERIC(19,4) NOT NULL,
    ico NUMERIC(19,4) NOT NULL,
    tip NUMERIC(19,4) NOT NULL,
    document_url TEXT,
    pdf_storage_path TEXT,
    xml_storage_path TEXT,
    cufe VARCHAR(255),
    tascode VARCHAR(255),
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

CREATE TABLE IF NOT EXISTS support_document_products (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    support_document_id UUID NOT NULL REFERENCES support_documents(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL DEFAULT 1,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL
);

INSERT INTO invoice_sequences (prefix, last_consecutive) VALUES ('DS', -1)
ON CONFLICT (prefix) DO NOTHING;

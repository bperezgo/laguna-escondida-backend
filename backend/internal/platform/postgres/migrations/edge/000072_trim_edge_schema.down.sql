-- Recreates the trimmed tables empty, exactly as at v71 (seeded rows are not restored), so a
-- local rig can step back down. A real box rolls forward instead: dropped data is gone.

CREATE TABLE delivery_zones (
    id uuid NOT NULL,
    name character varying(100) NOT NULL,
    boundary jsonb NOT NULL,
    active boolean DEFAULT true NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp without time zone,
    organization_id uuid NOT NULL
);
CREATE TABLE expense_categories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    code character varying(50) NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    is_active boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    organization_id uuid NOT NULL
);
CREATE TABLE expenses (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    category_id uuid NOT NULL,
    supplier_id uuid,
    amount numeric(19,4) NOT NULL,
    description character varying(500) NOT NULL,
    expense_date timestamp with time zone NOT NULL,
    reference character varying(255),
    notes text,
    pdf_storage_path text,
    xml_storage_path text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    organization_id uuid NOT NULL
);
CREATE TABLE invoice_sequences (
    prefix character varying(10) NOT NULL,
    last_consecutive integer DEFAULT 0 NOT NULL,
    organization_id uuid NOT NULL
);
CREATE TABLE nodes (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    kind character varying(10) NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    organization_id uuid NOT NULL,
    CONSTRAINT nodes_kind_check CHECK (kind IN ('cloud', 'edge'))
);
CREATE TABLE organization_einvoicing (
    organization_id uuid NOT NULL,
    invoice_prefix character varying(10) NOT NULL,
    support_document_prefix character varying(10) NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT organization_einvoicing_distinct_prefixes CHECK (((invoice_prefix)::text <> (support_document_prefix)::text))
);
CREATE TABLE purchase_entries (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    supplier_id uuid NOT NULL,
    total_amount numeric(19,4) NOT NULL,
    invoice_reference character varying(255),
    entry_date timestamp with time zone NOT NULL,
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    pdf_storage_path text,
    xml_storage_path text,
    organization_id uuid NOT NULL
);
CREATE TABLE purchase_entry_items (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    purchase_entry_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity numeric(19,4) NOT NULL,
    unit_cost numeric(19,4) NOT NULL,
    total_cost numeric(19,4) NOT NULL,
    organization_id uuid NOT NULL
);
CREATE TABLE supplier_catalog (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    supplier_id uuid NOT NULL,
    product_id uuid NOT NULL,
    supplier_sku character varying(255),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    organization_id uuid NOT NULL
);
CREATE TABLE suppliers (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    name character varying(255) NOT NULL,
    contact_name character varying(255),
    phone character varying(50),
    email character varying(255),
    notes text,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    identification_type character varying(50),
    identification_number character varying(50),
    organization_id uuid NOT NULL
);
CREATE TABLE support_document_products (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    support_document_id uuid NOT NULL,
    quantity integer DEFAULT 1 NOT NULL,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    description text DEFAULT ''::text NOT NULL,
    price numeric(19,4) DEFAULT 0 NOT NULL,
    organization_id uuid NOT NULL
);
CREATE TABLE support_documents (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    provider_document_number character varying(255) NOT NULL,
    provider_document_type character varying(10) NOT NULL,
    provider_name character varying(255) NOT NULL,
    provider_email character varying(255) NOT NULL,
    total_amount numeric(19,4) NOT NULL,
    discount_amount numeric(19,4) DEFAULT 0 NOT NULL,
    vat numeric(19,4) NOT NULL,
    ico numeric(19,4) NOT NULL,
    tip numeric(19,4) NOT NULL,
    document_url text,
    pdf_storage_path text,
    xml_storage_path text,
    cuds character varying(255),
    tascode character varying(255),
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    deleted_at timestamp with time zone,
    organization_id uuid NOT NULL
);
CREATE TABLE sync_inbox (
    op_id uuid NOT NULL,
    applied_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    organization_id uuid NOT NULL
);
ALTER TABLE ONLY delivery_zones
    ADD CONSTRAINT delivery_zones_pkey PRIMARY KEY (id);
ALTER TABLE ONLY expense_categories
    ADD CONSTRAINT expense_categories_organization_id_code_key UNIQUE (organization_id, code);
ALTER TABLE ONLY expense_categories
    ADD CONSTRAINT expense_categories_pkey PRIMARY KEY (id);
ALTER TABLE ONLY expenses
    ADD CONSTRAINT expenses_pkey PRIMARY KEY (id);
ALTER TABLE ONLY invoice_sequences
    ADD CONSTRAINT invoice_sequences_pkey PRIMARY KEY (organization_id, prefix);
ALTER TABLE ONLY nodes
    ADD CONSTRAINT nodes_pkey PRIMARY KEY (id);
ALTER TABLE ONLY organization_einvoicing
    ADD CONSTRAINT organization_einvoicing_pkey PRIMARY KEY (organization_id);
ALTER TABLE ONLY purchase_entries
    ADD CONSTRAINT purchase_entries_organization_id_id_key UNIQUE (organization_id, id);
ALTER TABLE ONLY purchase_entries
    ADD CONSTRAINT purchase_entries_pkey PRIMARY KEY (id);
ALTER TABLE ONLY purchase_entry_items
    ADD CONSTRAINT purchase_entry_items_pkey PRIMARY KEY (id);
ALTER TABLE ONLY supplier_catalog
    ADD CONSTRAINT supplier_catalog_pkey PRIMARY KEY (id);
ALTER TABLE ONLY suppliers
    ADD CONSTRAINT suppliers_organization_id_id_key UNIQUE (organization_id, id);
ALTER TABLE ONLY suppliers
    ADD CONSTRAINT suppliers_pkey PRIMARY KEY (id);
ALTER TABLE ONLY support_document_products
    ADD CONSTRAINT support_document_products_pkey PRIMARY KEY (id);
ALTER TABLE ONLY support_documents
    ADD CONSTRAINT support_documents_pkey PRIMARY KEY (id);
ALTER TABLE ONLY sync_inbox
    ADD CONSTRAINT sync_inbox_pkey PRIMARY KEY (op_id);
ALTER TABLE ONLY supplier_catalog
    ADD CONSTRAINT uk_supplier_product UNIQUE (supplier_id, product_id);
CREATE INDEX idx_delivery_zones_org ON delivery_zones USING btree (organization_id);
CREATE INDEX idx_expense_categories_code ON expense_categories USING btree (code);
CREATE INDEX idx_expense_categories_is_active ON expense_categories USING btree (is_active);
CREATE INDEX idx_expenses_category_id ON expenses USING btree (category_id);
CREATE INDEX idx_expenses_expense_date ON expenses USING btree (expense_date);
CREATE INDEX idx_expenses_org_expense_date ON expenses USING btree (organization_id, expense_date);
CREATE INDEX idx_expenses_supplier_id ON expenses USING btree (supplier_id);
CREATE INDEX idx_nodes_org ON nodes USING btree (organization_id);
CREATE INDEX idx_purchase_entries_entry_date ON purchase_entries USING btree (entry_date);
CREATE INDEX idx_purchase_entries_supplier_id ON purchase_entries USING btree (supplier_id);
CREATE INDEX idx_purchase_entry_items_entry_id ON purchase_entry_items USING btree (purchase_entry_id);
CREATE INDEX idx_purchase_entry_items_org_entry ON purchase_entry_items USING btree (organization_id, purchase_entry_id);
CREATE INDEX idx_purchase_entry_items_product_id ON purchase_entry_items USING btree (product_id);
CREATE INDEX idx_supplier_catalog_deleted_at ON supplier_catalog USING btree (deleted_at);
CREATE INDEX idx_supplier_catalog_org_supplier ON supplier_catalog USING btree (organization_id, supplier_id);
CREATE INDEX idx_supplier_catalog_product_id ON supplier_catalog USING btree (product_id) WHERE (deleted_at IS NULL);
CREATE INDEX idx_supplier_catalog_supplier_id ON supplier_catalog USING btree (supplier_id) WHERE (deleted_at IS NULL);
CREATE INDEX idx_suppliers_deleted_at ON suppliers USING btree (deleted_at);
CREATE INDEX idx_suppliers_identification_number ON suppliers USING btree (identification_number) WHERE (deleted_at IS NULL);
CREATE INDEX idx_suppliers_name ON suppliers USING btree (name) WHERE (deleted_at IS NULL);
CREATE INDEX idx_support_document_products_org_doc ON support_document_products USING btree (organization_id, support_document_id);
CREATE INDEX idx_support_documents_org ON support_documents USING btree (organization_id);
CREATE INDEX idx_sync_inbox_org ON sync_inbox USING btree (organization_id);
ALTER TABLE ONLY delivery_zones
    ADD CONSTRAINT delivery_zones_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY expense_categories
    ADD CONSTRAINT expense_categories_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY expenses
    ADD CONSTRAINT expenses_category_id_fkey FOREIGN KEY (category_id) REFERENCES expense_categories(id);
ALTER TABLE ONLY expenses
    ADD CONSTRAINT expenses_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY expenses
    ADD CONSTRAINT expenses_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES suppliers(id);
ALTER TABLE ONLY invoice_sequences
    ADD CONSTRAINT invoice_sequences_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY nodes
    ADD CONSTRAINT nodes_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY organization_einvoicing
    ADD CONSTRAINT organization_einvoicing_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY purchase_entries
    ADD CONSTRAINT purchase_entries_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY purchase_entries
    ADD CONSTRAINT purchase_entries_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES suppliers(id);
ALTER TABLE ONLY purchase_entry_items
    ADD CONSTRAINT purchase_entry_items_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY purchase_entry_items
    ADD CONSTRAINT purchase_entry_items_product_id_fkey FOREIGN KEY (organization_id, product_id) REFERENCES products(organization_id, id);
ALTER TABLE ONLY purchase_entry_items
    ADD CONSTRAINT purchase_entry_items_purchase_entry_id_fkey FOREIGN KEY (organization_id, purchase_entry_id) REFERENCES purchase_entries(organization_id, id) ON DELETE CASCADE;
ALTER TABLE ONLY supplier_catalog
    ADD CONSTRAINT supplier_catalog_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY supplier_catalog
    ADD CONSTRAINT supplier_catalog_product_id_fkey FOREIGN KEY (organization_id, product_id) REFERENCES products(organization_id, id);
ALTER TABLE ONLY supplier_catalog
    ADD CONSTRAINT supplier_catalog_supplier_id_fkey FOREIGN KEY (organization_id, supplier_id) REFERENCES suppliers(organization_id, id);
ALTER TABLE ONLY suppliers
    ADD CONSTRAINT suppliers_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY support_document_products
    ADD CONSTRAINT support_document_products_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY support_document_products
    ADD CONSTRAINT support_document_products_support_document_id_fkey FOREIGN KEY (support_document_id) REFERENCES support_documents(id) ON DELETE CASCADE;
ALTER TABLE ONLY support_documents
    ADD CONSTRAINT support_documents_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
ALTER TABLE ONLY sync_inbox
    ADD CONSTRAINT sync_inbox_organization_id_fkey FOREIGN KEY (organization_id) REFERENCES organizations(id);
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE delivery_zones TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE expense_categories TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE expenses TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE invoice_sequences TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE nodes TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE organization_einvoicing TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE purchase_entries TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE purchase_entry_items TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE supplier_catalog TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE suppliers TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE support_document_products TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE support_documents TO pos_app;
GRANT SELECT,INSERT,DELETE,UPDATE ON TABLE sync_inbox TO pos_app;

ALTER TABLE ONLY open_bills
    ADD CONSTRAINT fk_open_bills_delivery_zone FOREIGN KEY (delivery_zone_id) REFERENCES delivery_zones(id);

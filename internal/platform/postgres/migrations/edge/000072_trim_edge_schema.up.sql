-- The box keeps only the restaurant floor: accounting, e-invoicing, delivery zones and the
-- cloud's sync receive side live in the cloud. The edge binary runs this only after its
-- pre-check finds no data that exists only on the box (edgeTrimBlockers).
-- open_bills.delivery_zone_id stays: it is synced, the box just has no zones to point at.

ALTER TABLE open_bills DROP CONSTRAINT fk_open_bills_delivery_zone;

DROP TABLE
    support_document_products,
    support_documents,
    purchase_entry_items,
    purchase_entries,
    supplier_catalog,
    expenses,
    expense_categories,
    suppliers,
    organization_einvoicing,
    invoice_sequences,
    delivery_zones,
    nodes,
    sync_inbox;

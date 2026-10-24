ALTER TABLE open_bills_products ADD COLUMN created_by UUID;

UPDATE open_bills_products
SET created_by = open_bills.created_by
FROM open_bills
WHERE open_bills_products.open_bill_id = open_bills.id;

ALTER TABLE open_bills_products ALTER COLUMN created_by SET NOT NULL;

ALTER TABLE open_bills_products
    ADD CONSTRAINT fk_open_bills_products_created_by
    FOREIGN KEY (created_by) REFERENCES users(id);

CREATE TABLE open_bill_product_side_dishes (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    open_bill_product_id UUID NOT NULL REFERENCES open_bills_products(id) ON DELETE CASCADE,
    ingredient_product_id UUID NOT NULL REFERENCES products(id),
    quantity INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(open_bill_product_id, ingredient_product_id)
);

CREATE INDEX idx_obp_side_dishes_line ON open_bill_product_side_dishes(open_bill_product_id);

CREATE TABLE product_ingredients (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    composite_product_id UUID NOT NULL REFERENCES products(id),
    ingredient_product_id UUID NOT NULL REFERENCES products(id),
    quantity NUMERIC(19, 4) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(composite_product_id, ingredient_product_id)
);

CREATE INDEX idx_product_ingredients_composite ON product_ingredients(composite_product_id);
CREATE INDEX idx_product_ingredients_ingredient ON product_ingredients(ingredient_product_id);

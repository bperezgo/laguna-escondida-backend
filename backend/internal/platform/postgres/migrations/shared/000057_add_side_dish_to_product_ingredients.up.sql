ALTER TABLE product_ingredients RENAME COLUMN quantity TO default_quantity;
ALTER TABLE product_ingredients ADD COLUMN is_side_dish BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE product_ingredients ADD COLUMN min_quantity INTEGER NOT NULL DEFAULT 0;
ALTER TABLE product_ingredients ADD COLUMN max_quantity INTEGER NOT NULL DEFAULT 0;

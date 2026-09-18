ALTER TABLE product_ingredients DROP COLUMN max_quantity;
ALTER TABLE product_ingredients DROP COLUMN min_quantity;
ALTER TABLE product_ingredients DROP COLUMN is_side_dish;
ALTER TABLE product_ingredients RENAME COLUMN default_quantity TO quantity;

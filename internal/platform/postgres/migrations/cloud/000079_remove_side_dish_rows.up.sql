-- Option slots replace per-ingredient side dishes. Side-dish rows leave the recipe so a plate
-- set up again with slots does not also consume them as fixed ingredients. The columns stay
-- for orders and pushes from boxes that predate slots.
DELETE FROM product_ingredients WHERE is_side_dish;

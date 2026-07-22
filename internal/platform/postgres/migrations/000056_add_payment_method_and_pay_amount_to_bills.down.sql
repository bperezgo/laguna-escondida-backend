ALTER TABLE bills
    DROP COLUMN IF EXISTS payment_method,
    DROP COLUMN IF EXISTS pay_amount;

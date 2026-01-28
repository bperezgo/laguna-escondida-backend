-- Migration: drop_commands_tables (down)
-- Version: 000037
-- Description: Recreate the commands and command_items tables

CREATE TABLE commands (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    open_bill_id UUID NOT NULL REFERENCES open_bills(id),
    area VARCHAR(255) NOT NULL,
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE command_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    command_id UUID NOT NULL REFERENCES commands(id) ON DELETE CASCADE,
    product_id UUID NOT NULL REFERENCES products(id),
    quantity INTEGER NOT NULL DEFAULT 1,
    notes TEXT,
    status VARCHAR(50) NOT NULL DEFAULT 'created',
    open_bill_product_id UUID NOT NULL REFERENCES open_bills_products(id),
    priority INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_commands_open_bill_id ON commands(open_bill_id);
CREATE INDEX idx_commands_area ON commands(area);
CREATE INDEX idx_commands_status ON commands(status);
CREATE INDEX idx_command_items_command_id ON command_items(command_id);
CREATE INDEX idx_command_items_status ON command_items(status);
CREATE INDEX idx_command_items_open_bill_product_id ON command_items(open_bill_product_id);
CREATE INDEX idx_command_items_priority ON command_items(priority);

-- Migration: create_plate_option_slots
-- Version: 000078

-- A composite plate's single-choice side groups. Rows are soft-deleted so a removed choice
-- still propagates to the box; the pull reads every slot row of a plate as one document.
CREATE TABLE plate_option_slots (
    id                   UUID PRIMARY KEY,
    organization_id      UUID NOT NULL REFERENCES organizations(id),
    composite_product_id UUID NOT NULL,
    name                 VARCHAR(255) NOT NULL,
    position             INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at           TIMESTAMP,
    CONSTRAINT plate_option_slots_organization_id_id_key UNIQUE (organization_id, id),
    CONSTRAINT plate_option_slots_composite_product_id_fkey FOREIGN KEY (organization_id, composite_product_id)
        REFERENCES products (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_plate_option_slots_org_composite ON plate_option_slots (organization_id, composite_product_id);
CREATE INDEX idx_plate_option_slots_updated_at    ON plate_option_slots (updated_at);

CREATE TABLE plate_option_choices (
    id              UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations(id),
    slot_id         UUID NOT NULL,
    name            VARCHAR(255) NOT NULL,
    position        INTEGER NOT NULL DEFAULT 0,
    is_default      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at      TIMESTAMP,
    CONSTRAINT plate_option_choices_organization_id_id_key UNIQUE (organization_id, id),
    CONSTRAINT plate_option_choices_slot_id_fkey FOREIGN KEY (organization_id, slot_id)
        REFERENCES plate_option_slots (organization_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_plate_option_choices_org_slot ON plate_option_choices (organization_id, slot_id);
CREATE UNIQUE INDEX uq_plate_option_choices_one_default ON plate_option_choices (slot_id)
    WHERE is_default AND deleted_at IS NULL;

CREATE TABLE plate_option_choice_items (
    id                    UUID PRIMARY KEY,
    organization_id       UUID NOT NULL REFERENCES organizations(id),
    choice_id             UUID NOT NULL,
    ingredient_product_id UUID NOT NULL,
    quantity              INTEGER NOT NULL CHECK (quantity >= 1),
    created_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at            TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at            TIMESTAMP,
    CONSTRAINT plate_option_choice_items_choice_id_fkey FOREIGN KEY (organization_id, choice_id)
        REFERENCES plate_option_choices (organization_id, id) ON DELETE CASCADE,
    CONSTRAINT plate_option_choice_items_ingredient_product_id_fkey FOREIGN KEY (organization_id, ingredient_product_id)
        REFERENCES products (organization_id, id)
);

CREATE INDEX idx_plate_option_choice_items_org_choice ON plate_option_choice_items (organization_id, choice_id);

-- A line's resolved choices with the names it was taken with; no FK to the configuration so
-- renaming or removing a choice never rewrites an order.
CREATE TABLE open_bill_product_choices (
    id                   UUID PRIMARY KEY,
    organization_id      UUID NOT NULL REFERENCES organizations(id),
    open_bill_product_id UUID NOT NULL REFERENCES open_bills_products(id) ON DELETE CASCADE,
    slot_id              UUID NOT NULL,
    slot_name            VARCHAR(255) NOT NULL,
    choice_id            UUID NOT NULL,
    choice_name          VARCHAR(255) NOT NULL,
    position             INTEGER NOT NULL DEFAULT 0,
    created_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at           TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at           TIMESTAMP
);

CREATE INDEX idx_obp_choices_org_line ON open_bill_product_choices (organization_id, open_bill_product_id);

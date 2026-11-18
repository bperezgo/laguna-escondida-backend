-- Migration: add_reservation_pending_reason
-- Version: 000080


-- Why a WhatsApp request waits for staff: the first auto-confirm rule it failed. Set once, at creation.
ALTER TABLE reservations
    ADD COLUMN pending_reason VARCHAR(16)
        CHECK (pending_reason IN ('disabled', 'party_size', 'notice', 'outside_schedule', 'daily_cap'));

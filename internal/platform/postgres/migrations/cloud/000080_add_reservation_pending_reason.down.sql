-- Migration: add_reservation_pending_reason
-- Version: 000080


ALTER TABLE reservations DROP COLUMN pending_reason;

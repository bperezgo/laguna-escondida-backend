-- Migration: create_reservations_crm
-- Version: 000076

DROP TABLE IF EXISTS marketing_event_outbox;
DROP TABLE IF EXISTS feedback;
DROP TABLE IF EXISTS feedback_requests;
DROP TABLE IF EXISTS reservations;
DROP TABLE IF EXISTS contacts;

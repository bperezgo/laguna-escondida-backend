-- Migration: add_users_api_key_managed
-- Version: 000093

-- Marks a user created for one organization API key: hidden from user management and edge sync.
ALTER TABLE users ADD COLUMN api_key_managed BOOLEAN NOT NULL DEFAULT false;

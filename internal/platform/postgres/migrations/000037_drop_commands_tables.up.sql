-- Migration: drop_commands_tables
-- Version: 000037
-- Description: Drop the commands and command_items tables as they have been consolidated into open_bills and open_bills_products

-- Drop command_items first due to foreign key dependency on commands
DROP TABLE IF EXISTS command_items;

-- Drop commands table
DROP TABLE IF EXISTS commands;

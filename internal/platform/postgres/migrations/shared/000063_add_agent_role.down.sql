-- Migration: add_agent_role (down)
-- Version: 000063

DELETE FROM roles WHERE name = 'agent';

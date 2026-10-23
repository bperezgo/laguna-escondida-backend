-- Migration: register_laguna_edge_node (down)
-- Version: 000071

DELETE FROM nodes WHERE id = 'ce6ebef8-37cf-576b-b7a8-15aaaaae44a1';

-- Migration: register_laguna_edge_node
-- Version: 000071

-- The cloud learns a calling node's organization from this table. Laguna's box uses the id it
-- derives from its organization (config.deriveNodeID for mode "edge"); a box that overrides
-- NODE_ID needs its own row. On an edge database the row is inert.
INSERT INTO nodes (id, name, kind, organization_id)
VALUES ('ce6ebef8-37cf-576b-b7a8-15aaaaae44a1', 'Laguna Escondida edge', 'edge', 'bb0cad3e-be13-4bad-9a37-bba9525bfece')
ON CONFLICT (id) DO NOTHING;

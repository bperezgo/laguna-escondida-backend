-- Create organizations table
CREATE TABLE organizations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name VARCHAR(255) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Insert default organization: Laguna Escondida, Restaurante y Pesca S.A.S
INSERT INTO organizations (id, name) VALUES (
    'bb0cad3e-be13-4bad-9a37-bba9525bfece',
    'Laguna Escondida, Restaurante y Pesca S.A.S'
);

-- Endereço de entrega do cliente: para onde a central envia o rastreador
-- contratado. Um por cliente; sem ele, o cliente não contrata pelo painel.
CREATE TABLE customer_addresses (
    customer_id UUID PRIMARY KEY REFERENCES users (id) ON DELETE CASCADE,
    -- CEP só com os 8 dígitos.
    zip_code    TEXT        NOT NULL CHECK (zip_code ~ '^[0-9]{8}$'),
    street      TEXT        NOT NULL,
    number      TEXT        NOT NULL,
    complement  TEXT        NOT NULL DEFAULT '',
    district    TEXT        NOT NULL,
    city        TEXT        NOT NULL,
    state       TEXT        NOT NULL CHECK (state ~ '^[A-Z]{2}$'),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cópia do endereço no momento da contratação: se o cliente mudar o
-- endereço depois, o rastreador já contratado continua indo para onde foi
-- pedido. Nulo nas assinaturas criadas sem entrega (e nas anteriores a isto).
ALTER TABLE subscriptions ADD COLUMN delivery_address JSONB;

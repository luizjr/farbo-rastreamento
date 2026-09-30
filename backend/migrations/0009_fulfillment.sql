-- Acompanhamento do pedido até a casa do cliente, em duas linhas do tempo:
-- o chip M2M (que vai dentro do rastreador) e o próprio rastreador, que sai
-- pelo Melhor Envios.
CREATE TABLE fulfillments (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id     UUID NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    vehicle_id      UUID NOT NULL REFERENCES vehicles (id) ON DELETE CASCADE,
    subscription_id UUID REFERENCES subscriptions (id) ON DELETE SET NULL,

    chip_status     TEXT NOT NULL DEFAULT 'REQUESTED'
                    CHECK (chip_status IN ('REQUESTED', 'SHIPPED', 'AT_BASE', 'SEPARATED')),
    tracker_status  TEXT NOT NULL DEFAULT 'AWAITING_SUPPLIER'
                    CHECK (tracker_status IN ('AWAITING_SUPPLIER', 'AT_BASE', 'AWAITING_CHIP', 'CONFIGURING',
                                              'CONFIGURED', 'SHIPPED', 'IN_TRANSIT', 'DELIVERED')),

    -- Etiqueta no Melhor Envios. shipping_status é o status de lá (pending,
    -- released, generated, posted, delivered...).
    shipping_order_id    TEXT,
    shipping_protocol    TEXT NOT NULL DEFAULT '',
    shipping_service     TEXT NOT NULL DEFAULT '',
    shipping_price_cents INTEGER,
    shipping_status      TEXT NOT NULL DEFAULT '',
    tracking_code        TEXT NOT NULL DEFAULT '',
    label_url            TEXT NOT NULL DEFAULT '',

    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fulfillments_customer ON fulfillments (customer_id);
CREATE INDEX idx_fulfillments_vehicle ON fulfillments (vehicle_id);
CREATE UNIQUE INDEX idx_fulfillments_shipping_order ON fulfillments (shipping_order_id)
    WHERE shipping_order_id IS NOT NULL;
-- O que ainda está a caminho: é o que a sincronização consulta.
CREATE INDEX idx_fulfillments_in_transit ON fulfillments (tracker_status)
    WHERE tracker_status IN ('SHIPPED', 'IN_TRANSIT');

-- Histórico: cada mudança de status, com quem mudou (nulo = automático, pelo
-- Melhor Envios). É daqui que saem as datas da linha do tempo do cliente.
CREATE TABLE fulfillment_events (
    id             BIGSERIAL PRIMARY KEY,
    fulfillment_id UUID NOT NULL REFERENCES fulfillments (id) ON DELETE CASCADE,
    track          TEXT NOT NULL CHECK (track IN ('CHIP', 'TRACKER')),
    status         TEXT NOT NULL,
    note           TEXT NOT NULL DEFAULT '',
    actor_id       UUID REFERENCES users (id) ON DELETE SET NULL,
    automatic      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_fulfillment_events ON fulfillment_events (fulfillment_id, id);

-- Tokens OAuth de integrações (hoje, o Melhor Envios). Os valores são
-- cifrados pela aplicação antes de gravar.
CREATE TABLE integration_tokens (
    provider      TEXT PRIMARY KEY,
    access_token  TEXT NOT NULL,
    refresh_token TEXT NOT NULL,
    expires_at    TIMESTAMPTZ NOT NULL,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- "state" do OAuth em andamento: liga o retorno do Melhor Envios a quem
-- clicou em Conectar e barra retornos forjados.
CREATE TABLE integration_states (
    state      TEXT PRIMARY KEY,
    provider   TEXT NOT NULL,
    user_id    UUID REFERENCES users (id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Veículos de cliente que ainda esperam o rastreador passam a ser
-- acompanhados desde o início das duas linhas do tempo.
WITH created AS (
    INSERT INTO fulfillments (customer_id, vehicle_id, subscription_id)
    SELECT v.owner_id, v.id, s.id
    FROM vehicles v
    LEFT JOIN subscriptions s ON s.vehicle_id = v.id AND s.status = 'ACTIVE'
    WHERE v.owner_id IS NOT NULL AND v.device_id IS NULL
    RETURNING id
)
INSERT INTO fulfillment_events (fulfillment_id, track, status, note)
SELECT created.id, t.track, t.status, 'Acompanhamento iniciado'
FROM created
CROSS JOIN (VALUES ('CHIP', 'REQUESTED'), ('TRACKER', 'AWAITING_SUPPLIER')) AS t (track, status);

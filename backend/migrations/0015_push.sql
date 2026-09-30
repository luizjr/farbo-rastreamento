-- Notificações no celular (Web Push) do app do cliente.
--
-- push_config guarda o par de chaves VAPID gerado na primeira subida (quando
-- VAPID_PRIVATE_KEY não é informada). A chave privada fica cifrada com uma
-- chave derivada do JWT_SECRET: quem lê só o banco não consegue mandar
-- notificação em nome da central.
CREATE TABLE push_config (
    id                  BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (id),
    public_key          TEXT NOT NULL,
    private_key_sealed  TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- Cada navegador/celular em que o cliente ligou as notificações.
CREATE TABLE push_subscriptions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    -- Endereço no serviço de push do navegador (FCM, Mozilla, Apple...).
    endpoint        TEXT NOT NULL UNIQUE,
    p256dh          TEXT NOT NULL,
    auth            TEXT NOT NULL,
    user_agent      TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_success_at TIMESTAMPTZ
);

CREATE INDEX idx_push_subscriptions_user ON push_subscriptions (user_id);

-- Quantos aparelhos receberam cada alerta como notificação.
ALTER TABLE alert_notifications ADD COLUMN push_sent INT NOT NULL DEFAULT 0;

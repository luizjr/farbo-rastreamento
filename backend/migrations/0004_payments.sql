-- Pagamento das faturas por Pix (AbacatePay, checkout transparente).

-- Cada Pix gerado para uma fatura. Uma fatura pode ter vários: o Pix expira
-- e o cliente gera outro. O status é o do provedor (PENDING, PAID, EXPIRED,
-- REFUNDED, UNDER_DISPUTE...), sempre confirmado na API dele antes de valer.
CREATE TABLE payment_charges (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    invoice_id         UUID        NOT NULL REFERENCES invoices (id) ON DELETE CASCADE,
    provider           TEXT        NOT NULL,
    provider_charge_id TEXT        NOT NULL UNIQUE,
    amount_cents       INTEGER     NOT NULL CHECK (amount_cents > 0),
    status             TEXT        NOT NULL,
    -- Pix copia-e-cola e a imagem do QR Code (data:image/png;base64,...).
    br_code            TEXT        NOT NULL DEFAULT '',
    qr_code_image      TEXT        NOT NULL DEFAULT '',
    -- Cobrança feita com chave de testes do provedor.
    dev_mode           BOOLEAN     NOT NULL DEFAULT FALSE,
    expires_at         TIMESTAMPTZ,
    paid_at            TIMESTAMPTZ,
    -- Última consulta à API do provedor (limita a frequência das consultas).
    checked_at         TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_payment_charges_invoice ON payment_charges (invoice_id, created_at DESC);
CREATE INDEX idx_payment_charges_pending ON payment_charges (created_at) WHERE status = 'PENDING';

-- Como a fatura foi quitada: MANUAL (baixa da central) ou PIX (confirmado
-- pelo provedor). Vazio enquanto não paga.
ALTER TABLE invoices ADD COLUMN paid_via TEXT NOT NULL DEFAULT '';
UPDATE invoices SET paid_via = 'MANUAL' WHERE status = 'PAID';

-- Webhooks já processados: o provedor reenvia até 7 vezes, e o id do evento
-- é o que garante processar cada um uma vez só.
CREATE TABLE payment_webhook_events (
    id          TEXT PRIMARY KEY,
    provider    TEXT        NOT NULL,
    event       TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

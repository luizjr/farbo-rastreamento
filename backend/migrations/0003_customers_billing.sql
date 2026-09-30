-- Painel do cliente: perfil "customer", dono do veículo, assinaturas e faturas.

-- Cliente é um usuário com perfil próprio. O CHECK original foi declarado na
-- coluna, sem nome explícito; o Postgres o chama de users_role_check.
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
    CHECK (role IN ('admin', 'operator', 'viewer', 'customer'));

-- Dados de contato e cobrança; ficam vazios para a equipe da central.
ALTER TABLE users ADD COLUMN phone    TEXT NOT NULL DEFAULT '';
ALTER TABLE users ADD COLUMN document TEXT NOT NULL DEFAULT '';

-- Veículo sem dono (NULL) é da central: só a equipe enxerga.
ALTER TABLE vehicles ADD COLUMN owner_id UUID REFERENCES users (id) ON DELETE SET NULL;
CREATE INDEX idx_vehicles_owner ON vehicles (owner_id);

-- Cada assinatura ativa dá direito a um veículo cadastrado.
CREATE TABLE subscriptions (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id   UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    plan_name     TEXT        NOT NULL,
    price_cents   INTEGER     NOT NULL CHECK (price_cents >= 0),
    -- Até 28 para existir em todos os meses.
    due_day       SMALLINT    NOT NULL CHECK (due_day BETWEEN 1 AND 28),
    -- Próximo vencimento ainda sem fatura; o gerador avança mês a mês.
    next_due_date DATE        NOT NULL,
    status        TEXT        NOT NULL DEFAULT 'ACTIVE'
                  CHECK (status IN ('ACTIVE', 'CANCELED')),
    canceled_at   TIMESTAMPTZ,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_subscriptions_customer ON subscriptions (customer_id);
CREATE INDEX idx_subscriptions_due ON subscriptions (next_due_date) WHERE status = 'ACTIVE';

-- "Vencida" não é um status gravado: é uma fatura OPEN com vencimento no
-- passado, calculada na leitura.
CREATE TABLE invoices (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    customer_id     UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    -- NULL para fatura avulsa (instalação, por exemplo).
    subscription_id UUID        REFERENCES subscriptions (id) ON DELETE SET NULL,
    description     TEXT        NOT NULL,
    amount_cents    INTEGER     NOT NULL CHECK (amount_cents >= 0),
    due_date        DATE        NOT NULL,
    status          TEXT        NOT NULL DEFAULT 'OPEN'
                    CHECK (status IN ('OPEN', 'PAID', 'CANCELED')),
    paid_at         TIMESTAMPTZ,
    payment_url     TEXT        NOT NULL DEFAULT '',
    pix_code        TEXT        NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    -- Uma fatura por vencimento de cada assinatura: o gerador pode rodar de
    -- novo, ou em várias instâncias, sem duplicar.
    UNIQUE (subscription_id, due_date)
);
CREATE INDEX idx_invoices_customer ON invoices (customer_id, due_date DESC);
CREATE INDEX idx_invoices_open ON invoices (customer_id, due_date) WHERE status = 'OPEN';

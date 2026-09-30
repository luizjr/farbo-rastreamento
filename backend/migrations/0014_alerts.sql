-- Alertas por e-mail.
--
-- alert_settings guarda as escolhas de cada usuário (quais alertas recebe e o
-- horário de vigilância). Sem linha, valem os padrões do backend
-- (internal/alerts): quem nunca abriu a tela já recebe os alertas de
-- segurança.
CREATE TABLE alert_settings (
    user_id     UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    -- Tipos de alerta ligados; os que não aparecem aqui estão desligados.
    kinds       TEXT[] NOT NULL,
    -- Horário de vigilância em minutos desde a meia-noite (fuso da central).
    -- Início maior que o fim atravessa a meia-noite (22:00–06:00).
    guard_start SMALLINT NOT NULL CHECK (guard_start BETWEEN 0 AND 1439),
    guard_end   SMALLINT NOT NULL CHECK (guard_end BETWEEN 0 AND 1439),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CHECK (guard_start <> guard_end)
);

-- alert_notifications é o histórico: cada alerta que saiu, falhou ou foi
-- segurado (mesmo alerta repetido dentro do intervalo mínimo, ou teto por
-- hora). É também daqui que sai o "intervalo mínimo" entre dois e-mails
-- iguais — por isso ele sobrevive a reinícios.
CREATE TABLE alert_notifications (
    id              BIGSERIAL PRIMARY KEY,
    recipient       TEXT NOT NULL,
    user_id         UUID REFERENCES users(id) ON DELETE CASCADE,
    vehicle_id      UUID REFERENCES vehicles(id) ON DELETE SET NULL,
    device_id       UUID,
    kind            TEXT NOT NULL,
    event_id        BIGINT,
    occurred_at     TIMESTAMPTZ NOT NULL,
    -- PENDING (na fila de envio), SENT, FAILED ou SUPPRESSED.
    status          TEXT NOT NULL,
    -- Em SUPPRESSED: por que foi segurado (cooldown, hourly_cap).
    reason          TEXT NOT NULL DEFAULT '',
    -- Em SENT: quantas ocorrências seguradas o e-mail resumiu.
    suppressed_count INT NOT NULL DEFAULT 0,
    error           TEXT NOT NULL DEFAULT '',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    sent_at         TIMESTAMPTZ
);

-- Intervalo mínimo e contagem de repetidos: o último envio de um alerta,
-- para um destinatário, de um aparelho.
CREATE INDEX idx_alert_notifications_dedupe
    ON alert_notifications (recipient, device_id, kind, created_at DESC);
-- Teto por hora de cada destinatário.
CREATE INDEX idx_alert_notifications_recipient ON alert_notifications (recipient, created_at DESC);
-- Histórico na tela do cliente.
CREATE INDEX idx_alert_notifications_user ON alert_notifications (user_id, created_at DESC);

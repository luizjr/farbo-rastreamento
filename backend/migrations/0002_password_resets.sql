-- Pedidos de redefinição de senha por e-mail.
--
-- Como nos refresh tokens, só o hash SHA-256 do token fica no banco: o valor
-- em claro existe apenas no link enviado ao usuário. Cada token vale uma vez
-- (used_at) e por tempo limitado (expires_at); um pedido novo apaga os
-- anteriores ainda não usados, então só o link mais recente funciona.
CREATE TABLE password_reset_tokens (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID        NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_password_reset_tokens_user ON password_reset_tokens (user_id, created_at DESC);

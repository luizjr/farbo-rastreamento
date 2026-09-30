-- Cada sessão (refresh token) guarda qual JWT_SECRET estava em uso quando foi
-- aberta: um identificador derivado do segredo, nunca o segredo. Trocar o
-- JWT_SECRET encerra todas as sessões — os access tokens antigos já deixavam
-- de valer, mas os refresh tokens são opacos e sobreviviam à troca.
--
-- As sessões já abertas ficam com key_id vazio e são encerradas na primeira
-- subida desta versão (não há como saber com que segredo foram emitidas).
ALTER TABLE refresh_tokens ADD COLUMN key_id TEXT NOT NULL DEFAULT '';

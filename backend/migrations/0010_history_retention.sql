-- Quantos dias de histórico (posições e eventos) guardar: 7, 14 ou 30. No
-- cliente vale para todos os veículos dele; no veículo é uma exceção. Nulo
-- herda: veículo → cliente → padrão da central (HISTORY_RETENTION_DAYS).
ALTER TABLE users ADD COLUMN history_retention_days SMALLINT
    CHECK (history_retention_days IN (7, 14, 30));
ALTER TABLE vehicles ADD COLUMN history_retention_days SMALLINT
    CHECK (history_retention_days IN (7, 14, 30));

-- Posições ocupam a maior parte do disco (~43 milhões por dia com 5000
-- rastreadores a cada 10 s). O índice por received_at não atende nenhuma
-- consulta — o histórico é sempre por aparelho e hora do GPS — e custava
-- ~7% do espaço de cada posição.
DROP INDEX IF EXISTS idx_positions_received;

-- A partir daqui o pacote bruto só é guardado com POSITIONS_STORE_RAW=true
-- (investigação de um modelo novo); as linhas antigas saem com a limpeza do
-- histórico. Pacotes com problema continuam em raw_packets.

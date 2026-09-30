-- Estorno de pagamentos Pix pela central (AbacatePay /transparents/refund).

-- O pedido de estorno: em produção ele é assíncrono — a cobrança só vira
-- REFUNDED quando a AbacatePay conclui —, então o pedido fica registrado para
-- não ser feito duas vezes e para a consulta periódica acompanhar.
ALTER TABLE payment_charges ADD COLUMN refund_requested_at TIMESTAMPTZ;
ALTER TABLE payment_charges ADD COLUMN refund_id     TEXT NOT NULL DEFAULT '';
ALTER TABLE payment_charges ADD COLUMN refund_reason TEXT NOT NULL DEFAULT '';

-- Qual Pix quitou a fatura. Estornar esse Pix reabre a fatura; estornar
-- outro (um pagamento em duplicidade, por exemplo) não mexe nela.
ALTER TABLE invoices ADD COLUMN paid_charge_id UUID REFERENCES payment_charges (id) ON DELETE SET NULL;

UPDATE invoices i
SET paid_charge_id = (
    SELECT c.id FROM payment_charges c
    WHERE c.invoice_id = i.id AND c.status = 'PAID'
    ORDER BY c.paid_at NULLS LAST, c.created_at
    LIMIT 1
)
WHERE i.status = 'PAID' AND i.paid_via = 'PIX';

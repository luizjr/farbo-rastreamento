import { useEffect, useRef, useState } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';

import type { PixApi } from '@/api/resources';
import { Button } from '@/components/ui/Button';
import fieldStyles from '@/components/ui/Field.module.css';
import { Modal } from '@/components/ui/Modal';
import { Spinner } from '@/components/ui/Spinner';
import { useToast } from '@/components/ui/Toast';
import { formatDateTime, formatMoney } from '@/services/format';
import type { Invoice, PixCharge } from '@/types';

import styles from './Billing.module.css';

/** De quanto em quanto tempo a janela pergunta se o Pix foi pago. */
const POLL_MS = 4000;

interface PixPaymentModalProps {
  /** Fatura a pagar; nula fecha a janela. */
  invoice: Invoice | null;
  api: PixApi;
  /** Quem está vendo: muda só o texto do fim. */
  audience: 'customer' | 'staff';
  onClose: () => void;
  /** Chamado uma vez quando o pagamento é confirmado. */
  onPaid: () => void;
}

/**
 * Pagamento de uma fatura por Pix: QR Code, copia-e-cola e a confirmação,
 * que aparece sozinha — a janela consulta o status enquanto está aberta.
 */
export function PixPaymentModal({ invoice, api, audience, onClose, onPaid }: PixPaymentModalProps) {
  const { notify } = useToast();
  const [charge, setCharge] = useState<PixCharge | null>(null);
  const [error, setError] = useState('');
  const notified = useRef(false);

  const create = useMutation({
    mutationFn: api.invoicePix,
    onSuccess: (created) => setCharge(created),
    onError: (err: Error) => setError(err.message),
  });
  const start = (invoiceId: string) => {
    setCharge(null);
    setError('');
    create.mutate(invoiceId);
  };

  // Abrir a janela já gera (ou reaproveita) o Pix da fatura.
  const invoiceId = invoice?.id;
  useEffect(() => {
    notified.current = false;
    if (invoiceId) start(invoiceId);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [invoiceId]);

  const paid = Boolean(charge && (charge.status === 'PAID' || charge.invoiceStatus === 'PAID'));
  const ended = Boolean(charge && ['EXPIRED', 'CANCELLED', 'REFUNDED'].includes(charge.status));

  const status = useQuery({
    queryKey: ['pix-charge', charge?.id],
    queryFn: () => api.charge(charge!.id),
    enabled: Boolean(invoice && charge) && !paid && !ended,
    refetchInterval: POLL_MS,
  });
  useEffect(() => {
    if (status.data) setCharge(status.data);
  }, [status.data]);

  useEffect(() => {
    if (paid && !notified.current) {
      notified.current = true;
      onPaid();
    }
  }, [paid, onPaid]);

  const simulate = useMutation({
    mutationFn: () => api.simulateCharge(charge!.id),
    onSuccess: (updated) => setCharge(updated),
    onError: (err: Error) => notify({ tone: 'error', title: 'Simulação falhou', description: err.message }),
  });

  const copy = async () => {
    if (!charge) return;
    try {
      await navigator.clipboard.writeText(charge.brCode);
      notify({ tone: 'success', title: 'Código Pix copiado', description: 'Cole no app do seu banco para pagar.' });
    } catch {
      notify({ tone: 'error', title: 'Não foi possível copiar', description: 'Selecione o código e copie manualmente.' });
    }
  };

  return (
    <Modal
      open={invoice !== null}
      title={paid ? 'Pagamento confirmado' : 'Pagar com Pix'}
      onClose={onClose}
      footer={
        <Button variant={paid ? 'primary' : 'secondary'} onClick={onClose}>
          {paid ? 'Concluir' : 'Fechar'}
        </Button>
      }
    >
      {create.isPending && <Spinner label="Gerando o Pix…" />}

      {error && (
        <div className={styles.pix}>
          <p>{error}</p>
          <Button variant="primary" onClick={() => invoiceId && start(invoiceId)}>
            Tentar de novo
          </Button>
        </div>
      )}

      {charge && paid && (
        <div className={styles.pix} role="status">
          <div className={styles.pixDone} aria-hidden="true">
            ✓
          </div>
          <div className={styles.pixAmount}>{formatMoney(charge.amountCents)}</div>
          <p>
            {audience === 'customer'
              ? 'Pagamento recebido. Obrigado! A fatura foi quitada e, se o acesso estava suspenso, ele já foi liberado.'
              : 'Pagamento confirmado pelo provedor; a fatura foi quitada automaticamente.'}
          </p>
        </div>
      )}

      {charge && ended && !paid && (
        <div className={styles.pix}>
          <p>Este Pix expirou antes do pagamento. Gere um novo para pagar a fatura.</p>
          <Button variant="primary" onClick={() => invoiceId && start(invoiceId)}>
            Gerar novo Pix
          </Button>
        </div>
      )}

      {charge && !paid && !ended && (
        <div className={styles.pix}>
          <div>
            <div className={styles.pixAmount}>{formatMoney(charge.amountCents)}</div>
            <div className={styles.muted}>{invoice?.description}</div>
          </div>

          <img className={styles.pixQr} src={charge.qrCodeImage} alt="QR Code do Pix" />

          <div className={styles.pixCopy}>
            <input
              className={fieldStyles.input}
              readOnly
              value={charge.brCode}
              aria-label="Pix copia e cola"
              onFocus={(event) => event.target.select()}
            />
            <Button variant="primary" onClick={() => void copy()}>
              Copiar
            </Button>
          </div>

          <ol className={styles.pixSteps}>
            <li>Abra o app do seu banco e escolha pagar com Pix.</li>
            <li>Leia o QR Code ou cole o código copiado.</li>
            <li>Confirme o pagamento — a confirmação aparece aqui sozinha.</li>
          </ol>

          <div className={styles.pixWaiting}>
            <span className={styles.pixSpinner} aria-hidden="true" />
            Aguardando pagamento
            {charge.expiresAt && ` · válido até ${formatDateTime(charge.expiresAt).slice(0, 17)}`}
          </div>

          {charge.devMode && (
            <div className={styles.pixSandbox}>
              <span>
                <strong>Ambiente de testes.</strong> Este Pix é do sandbox da AbacatePay: nenhum
                dinheiro de verdade é movimentado.
              </span>
              <Button size="small" variant="secondary" loading={simulate.isPending} onClick={() => simulate.mutate()}>
                Simular pagamento
              </Button>
            </div>
          )}
        </div>
      )}
    </Modal>
  );
}

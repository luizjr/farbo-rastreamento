import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { useMutation } from '@tanstack/react-query';

import { Button } from '@/components/ui/Button';
import { Modal } from '@/components/ui/Modal';
import type { DeliveryAddress } from '@/types';

import pageStyles from '@/pages/Page.module.css';
import { AddressFields, EMPTY_ADDRESS, isAddressComplete } from './AddressFields';

/** Cadastro ou troca do endereço de entrega (cliente ou central). */
export function AddressModal({
  open,
  initial,
  intro,
  save,
  onClose,
  onSaved,
}: {
  open: boolean;
  initial: DeliveryAddress | null;
  intro?: ReactNode;
  save: (address: DeliveryAddress) => Promise<DeliveryAddress>;
  onClose: () => void;
  onSaved: (address: DeliveryAddress) => void;
}) {
  const [draft, setDraft] = useState<DeliveryAddress>(EMPTY_ADDRESS);
  const [error, setError] = useState('');

  useEffect(() => {
    if (open) {
      setDraft(initial ?? EMPTY_ADDRESS);
      setError('');
    }
    // Só ao abrir: o endereço salvo não deve apagar o que está sendo digitado.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const mutation = useMutation({
    mutationFn: save,
    onSuccess: onSaved,
    onError: (err: Error) => setError(err.message),
  });

  return (
    <Modal
      open={open}
      title={initial ? 'Alterar endereço de entrega' : 'Cadastrar endereço de entrega'}
      onClose={onClose}
      footer={
        <>
          <Button variant="ghost" onClick={onClose}>
            Cancelar
          </Button>
          <Button
            variant="primary"
            loading={mutation.isPending}
            disabled={!isAddressComplete(draft)}
            onClick={() => {
              setError('');
              mutation.mutate(draft);
            }}
          >
            Salvar endereço
          </Button>
        </>
      }
    >
      <div className={pageStyles.form}>
        {intro && <p className={pageStyles.description}>{intro}</p>}
        {error && <div className={pageStyles.note}>{error}</div>}
        <AddressFields value={draft} onChange={setDraft} autoFocus />
      </div>
    </Modal>
  );
}

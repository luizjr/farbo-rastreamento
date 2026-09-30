import { useEffect, useRef, useState } from 'react';

import { SelectField, TextField } from '@/components/ui/Field';
import { formatZipCode } from '@/services/format';
import { lookupZipCode } from '@/services/zipcode';
import type { DeliveryAddress } from '@/types';

import pageStyles from '@/pages/Page.module.css';

export const EMPTY_ADDRESS: DeliveryAddress = {
  zipCode: '',
  street: '',
  number: '',
  complement: '',
  district: '',
  city: '',
  state: '',
};

const STATES = [
  'AC', 'AL', 'AP', 'AM', 'BA', 'CE', 'DF', 'ES', 'GO', 'MA', 'MT', 'MS', 'MG', 'PA',
  'PB', 'PR', 'PE', 'PI', 'RJ', 'RN', 'RS', 'RO', 'RR', 'SC', 'SP', 'SE', 'TO',
];

/** O mínimo para enviar ao servidor (que confere o resto). */
export function isAddressComplete(a: DeliveryAddress): boolean {
  return (
    a.zipCode.replace(/\D/g, '').length === 8 &&
    [a.street, a.number, a.district, a.city, a.state].every((field) => field.trim() !== '')
  );
}

/**
 * Campos do endereço de entrega, os mesmos para o cliente e para a central.
 * Ao completar o CEP, busca rua, bairro, cidade e UF e leva o foco ao número.
 */
export function AddressFields({
  value,
  onChange,
  autoFocus = false,
}: {
  value: DeliveryAddress;
  onChange: (value: DeliveryAddress) => void;
  autoFocus?: boolean;
}) {
  const container = useRef<HTMLDivElement>(null);
  const [lookup, setLookup] = useState<'idle' | 'loading' | 'not-found' | 'error'>('idle');
  // O CEP já consultado: editar outro campo não dispara uma nova busca.
  const lastLookup = useRef(value.zipCode.replace(/\D/g, ''));
  // Os campos mais recentes, para a resposta da busca não apagar o que foi
  // digitado enquanto ela voltava.
  const latest = useRef(value);
  latest.current = value;

  const digits = value.zipCode.replace(/\D/g, '');

  useEffect(() => {
    if (digits.length !== 8 || digits === lastLookup.current) return;
    lastLookup.current = digits;
    const controller = new AbortController();
    setLookup('loading');
    lookupZipCode(digits, controller.signal).then((result) => {
      if (controller.signal.aborted) return;
      if (result.status !== 'found') {
        setLookup(result.status);
        return;
      }
      setLookup('idle');
      const found = result.address;
      const current = latest.current;
      onChange({
        ...current,
        street: found.street || current.street,
        district: found.district || current.district,
        city: found.city || current.city,
        state: found.state || current.state,
      });
      // CEP de cidade pequena vem sem rua: o foco vai para o que falta.
      const next = found.street ? 'number' : 'street';
      container.current?.querySelector<HTMLInputElement>(`input[name=${next}]`)?.focus();
    });
    return () => controller.abort();
    // onChange muda a cada render do pai; a busca depende só do CEP.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [digits]);

  const zipHint =
    lookup === 'loading'
      ? 'Buscando o endereço…'
      : lookup === 'not-found'
        ? 'CEP não encontrado. Confira o número ou preencha o endereço abaixo.'
        : lookup === 'error'
          ? 'Não deu para buscar o CEP agora; preencha o endereço abaixo.'
          : 'Preenchemos rua, bairro e cidade a partir do CEP.';

  const set = (field: keyof DeliveryAddress) => (event: { target: { value: string } }) =>
    onChange({ ...value, [field]: event.target.value });

  return (
    <div className={pageStyles.form} ref={container}>
      <div className={pageStyles.formRow}>
        <TextField
          label="CEP"
          name="zipCode"
          inputMode="numeric"
          autoComplete="postal-code"
          placeholder="00000-000"
          required
          autoFocus={autoFocus}
          hint={zipHint}
          value={formatZipCode(value.zipCode)}
          onChange={(event) => onChange({ ...value, zipCode: event.target.value.replace(/\D/g, '').slice(0, 8) })}
        />
      </div>
      <TextField
        label="Rua"
        name="street"
        autoComplete="address-line1"
        required
        value={value.street}
        onChange={set('street')}
      />
      <div className={pageStyles.formRow}>
        <TextField
          label="Número"
          name="number"
          placeholder="Ex.: 120 ou S/N"
          required
          value={value.number}
          onChange={set('number')}
        />
        <TextField
          label="Complemento"
          name="complement"
          autoComplete="address-line2"
          placeholder="Apto, bloco, referência"
          value={value.complement}
          onChange={set('complement')}
        />
      </div>
      <TextField
        label="Bairro"
        name="district"
        required
        value={value.district}
        onChange={set('district')}
      />
      <div className={pageStyles.formRow}>
        <TextField
          label="Cidade"
          name="city"
          autoComplete="address-level2"
          required
          value={value.city}
          onChange={set('city')}
        />
        <SelectField label="UF" name="state" required value={value.state} onChange={set('state')}>
          <option value="">Selecione</option>
          {STATES.map((uf) => (
            <option key={uf} value={uf}>
              {uf}
            </option>
          ))}
        </SelectField>
      </div>
    </div>
  );
}

import { Button } from '@/components/ui/Button';
import billing from '@/components/billing/Billing.module.css';
import { formatAddressLines } from '@/services/format';
import type { DeliveryAddress } from '@/types';

/** "Entrega do rastreador" com o endereço e, opcionalmente, o botão de trocar. */
export function DeliveryBox({
  address,
  label = 'Entrega do rastreador',
  onChange,
}: {
  address: DeliveryAddress;
  label?: string;
  onChange?: () => void;
}) {
  const [line1, line2] = formatAddressLines(address);
  return (
    <div className={billing.delivery}>
      <div className={billing.deliveryText}>
        <span className={billing.deliveryLabel}>{label}</span>
        <strong>{line1}</strong>
        <span className={billing.muted}>{line2}</span>
      </div>
      {onChange && (
        <Button size="small" variant="ghost" onClick={onChange}>
          Alterar
        </Button>
      )}
    </div>
  );
}

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { customersApi } from '@/api/resources';
import { Spinner } from '@/components/ui/Spinner';
import { useToast } from '@/components/ui/Toast';
import type { AlertSettingsInput } from '@/types';

import { AlertSettingsPanel } from './AlertSettingsPanel';

/** Alertas por e-mail de um cliente, vistos (e ajustados) pela central. */
export function CustomerAlertsSection({ customerId }: { customerId: string }) {
  const queryClient = useQueryClient();
  const { notify } = useToast();
  const key = ['customer', customerId, 'alerts'];
  const settings = useQuery({ queryKey: key, queryFn: () => customersApi.alerts(customerId) });
  const save = useMutation({
    mutationFn: (input: AlertSettingsInput) => customersApi.saveAlerts(customerId, input),
    onSuccess: (data) => {
      queryClient.setQueryData(key, data);
      notify({ tone: 'success', title: 'Alertas do cliente salvos' });
    },
    onError: (error: Error) => notify({ tone: 'error', title: 'Não foi possível salvar', description: error.message }),
  });

  if (settings.isLoading) return <Spinner label="Carregando alertas" />;
  if (!settings.data) return null;
  return (
    <AlertSettingsPanel
      settings={settings.data}
      audience="central"
      saving={save.isPending}
      onSave={(input) => save.mutate(input)}
    />
  );
}

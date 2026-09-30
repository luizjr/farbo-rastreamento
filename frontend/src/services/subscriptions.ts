import type { Subscription } from '@/types';

/**
 * A assinatura de cada veículo: a ativa ou, sem ela, a encerrada mais
 * recente (para mostrar "Assinatura encerrada").
 */
export function subscriptionsByVehicle(subscriptions: Subscription[]): Map<string, Subscription> {
  const byVehicle = new Map<string, Subscription>();
  for (const sub of subscriptions) {
    if (!sub.vehicleId) continue;
    const current = byVehicle.get(sub.vehicleId);
    const better =
      !current ||
      (current.status !== 'ACTIVE' && (sub.status === 'ACTIVE' || sub.createdAt > current.createdAt));
    if (better) byVehicle.set(sub.vehicleId, sub);
  }
  return byVehicle;
}

/** Assinaturas ativas sem veículo: só existem em dados de antes do fluxo único. */
export function subscriptionsWithoutVehicle(subscriptions: Subscription[]): Subscription[] {
  return subscriptions.filter((sub) => sub.status === 'ACTIVE' && !sub.vehicleId);
}

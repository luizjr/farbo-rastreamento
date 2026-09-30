import { useMemo, useState } from 'react';
import { useNavigate } from 'react-router-dom';

import { isSuspendedError, SuspendedNotice } from '@/components/billing/SuspendedNotice';
import { TrackerMap } from '@/components/map/TrackerMap';
import { Badge } from '@/components/ui/Badge';
import type { BadgeTone } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Spinner } from '@/components/ui/Spinner';
import { useVehicles } from '@/hooks/useVehicles';
import { formatDeviceStatus, formatRelative, formatSpeed } from '@/services/format';
import type { VehicleView } from '@/types';

import styles from './Screen.module.css';

export function statusTone(vehicle: VehicleView): BadgeTone {
  switch (vehicle.device?.status) {
    case 'ONLINE':
      return 'success';
    case 'STALE':
      return 'warning';
    default:
      return 'neutral';
  }
}

/** Primeiro os que estão comunicando, depois por nome. */
function sortVehicles(list: VehicleView[]): VehicleView[] {
  const rank = (v: VehicleView) => (v.device?.status === 'ONLINE' ? 0 : v.device ? 1 : 2);
  return [...list].sort((a, b) => rank(a) - rank(b) || a.name.localeCompare(b.name));
}

/** Tela inicial: mapa com os veículos e a lista embaixo. */
export function MapScreen() {
  const navigate = useNavigate();
  const vehicles = useVehicles();
  const [selected, setSelected] = useState<string | null>(null);
  const list = useMemo(() => sortVehicles(vehicles.data ?? []), [vehicles.data]);

  if (isSuspendedError(vehicles.error)) return <SuspendedNotice message={vehicles.error?.message} />;
  if (vehicles.isLoading) return <Spinner label="Carregando veículos" />;

  return (
    <div className={styles.mapScreen}>
      <div className={styles.map}>
        <TrackerMap vehicles={list} selectedId={selected} onSelect={setSelected} showControls={false} />
      </div>
      <section className={styles.sheet} aria-label="Seus veículos">
        <div className={styles.handle} />
        <div className={styles.sheetHeader}>
          <h2 className={styles.sectionTitle}>Seus veículos</h2>
          <span className={styles.muted}>
            {list.filter((v) => v.device?.status === 'ONLINE').length} de {list.length} online
          </span>
        </div>
        <div className={styles.sheetList}>
          {list.length === 0 && (
            <div className={styles.section}>
              <p className={styles.muted}>Você ainda não tem veículos com rastreador.</p>
              <Button onClick={() => navigate('/meus-veiculos')}>Novo veículo</Button>
            </div>
          )}
          {list.map((vehicle) => {
            const position = vehicle.lastPosition;
            const acc = vehicle.state?.acc ?? position?.acc ?? null;
            return (
              <button
                key={vehicle.id}
                type="button"
                className={`${styles.vehicleCard} ${vehicle.id === selected ? styles.vehicleCardSelected : ''}`}
                onClick={() => navigate(`/veiculos/${vehicle.id}`)}
              >
                <span className={styles.vehicleTop}>
                  <span className={styles.vehicleName}>{vehicle.name}</span>
                  <Badge tone={statusTone(vehicle)} dot>
                    {vehicle.device ? formatDeviceStatus(vehicle.device.status) : 'Sem rastreador'}
                  </Badge>
                </span>
                <span className={styles.vehicleMeta}>
                  {vehicle.plate && <span className={styles.plate}>{vehicle.plate}</span>}
                  {vehicle.device ? (
                    <>
                      <span>Ignição {acc === null ? '—' : acc ? 'ligada' : 'desligada'}</span>
                      {position && <span>{formatSpeed(position.speedKmh)}</span>}
                      <span>{position ? formatRelative(position.gpsTimestamp) : 'aguardando o primeiro sinal'}</span>
                    </>
                  ) : (
                    <span>Acompanhe a entrega em Veículos</span>
                  )}
                </span>
              </button>
            );
          })}
        </div>
      </section>
    </div>
  );
}

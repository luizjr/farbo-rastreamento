import { useEffect, useMemo, useState } from 'react';
import { Circle, MapContainer, Marker, Polyline, Popup, TileLayer, useMap } from 'react-leaflet';
import type { LatLngExpression, LatLngTuple } from 'leaflet';
import L from 'leaflet';

import { Address } from '@/components/ui/Address';
import { formatCoordinates, formatDateTime, formatHeading, formatSpeed } from '@/services/format';
import type { Geofence, Position, VehicleView } from '@/types';

import { endpointIcon, vehicleIcon } from './markers';
import styles from './TrackerMap.module.css';

/** Centro padrão (São Paulo) quando ainda não há nenhuma posição. */
const DEFAULT_CENTER: LatLngTuple = [-23.5505, -46.6333];
const DEFAULT_ZOOM = 13;
const MOVING_SPEED_KMH = 3;
/** Verde da marca, o mesmo da landing. */
const BRAND_GREEN = '#3be558';

interface TrackerMapProps {
  vehicles: VehicleView[];
  selectedId?: string | null;
  onSelect?: (vehicleId: string) => void;
  /** Trajeto desenhado por cima (histórico ou playback). */
  track?: Position[];
  /** Posição destacada durante o playback. */
  highlight?: Position | null;
  geofences?: Geofence[];
  showControls?: boolean;
}

export function TrackerMap({
  vehicles,
  selectedId,
  onSelect,
  track,
  highlight,
  geofences = [],
  showControls = true,
}: TrackerMapProps) {
  const [following, setFollowing] = useState(true);
  const [showFences, setShowFences] = useState(true);

  const located = useMemo(
    () => vehicles.filter((vehicle) => vehicle.lastPosition !== null),
    [vehicles],
  );

  const selected = located.find((vehicle) => vehicle.id === selectedId) ?? null;

  const center: LatLngTuple = selected?.lastPosition
    ? [selected.lastPosition.latitude, selected.lastPosition.longitude]
    : located[0]?.lastPosition
      ? [located[0].lastPosition.latitude, located[0].lastPosition.longitude]
      : DEFAULT_CENTER;

  const trackLine = useMemo<LatLngExpression[]>(
    () => (track ?? []).map((point) => [point.latitude, point.longitude] as LatLngTuple),
    [track],
  );

  return (
    <div className={styles.wrapper}>
      <MapContainer
        center={center}
        zoom={DEFAULT_ZOOM}
        className={styles.map}
        zoomControl={false}
        preferCanvas
      >
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          maxZoom={19}
        />

        {showFences &&
          geofences
            .filter((fence) => fence.active)
            .map((fence) => (
              <Circle
                key={fence.id}
                center={[fence.latitude, fence.longitude]}
                radius={fence.radiusMeters}
                pathOptions={{ color: BRAND_GREEN, fillOpacity: 0.08, weight: 1.5, dashArray: '4 4' }}
              >
                <Popup>
                  <div className={styles.popup}>
                    <span className={styles.popupTitle}>{fence.name}</span>
                    <div className={styles.popupRow}>
                      <span>Raio</span>
                      <span className={styles.popupValue}>{fence.radiusMeters} m</span>
                    </div>
                  </div>
                </Popup>
              </Circle>
            ))}

        {/* Contorno escuro por baixo do trajeto, para o verde continuar
            legível também sobre o mapa claro. */}
        {trackLine.length > 1 && (
          <>
            <Polyline positions={trackLine} pathOptions={{ color: '#060907', weight: 6, opacity: 0.45 }} />
            <Polyline positions={trackLine} pathOptions={{ color: BRAND_GREEN, weight: 3.5, opacity: 0.95 }} />
          </>
        )}

        {track && track.length > 1 && (
          <>
            <Marker
              position={[track[0].latitude, track[0].longitude]}
              icon={endpointIcon('start')}
            >
              <Popup>Início — {formatDateTime(track[0].gpsTimestamp)}</Popup>
            </Marker>
            <Marker
              position={[track[track.length - 1].latitude, track[track.length - 1].longitude]}
              icon={endpointIcon('end')}
            >
              <Popup>Fim — {formatDateTime(track[track.length - 1].gpsTimestamp)}</Popup>
            </Marker>
          </>
        )}

        {highlight && (
          <Marker
            position={[highlight.latitude, highlight.longitude]}
            icon={vehicleIcon({
              ignition: highlight.acc,
              heading: highlight.heading,
              moving: highlight.speedKmh >= MOVING_SPEED_KMH,
              selected: true,
              blocked: highlight.relayOn === true,
              online: true, // reprodução do histórico: sempre trata como "atual"
            })}
          >
            <Popup>
              <PositionPopup title="Reprodução" position={highlight} showAddress={false} />
            </Popup>
          </Marker>
        )}

        {!highlight &&
          located.map((vehicle) => {
            const position = vehicle.lastPosition as Position;
            return (
              <Marker
                key={vehicle.id}
                position={[position.latitude, position.longitude]}
                icon={vehicleIcon({
                  ignition: vehicle.state?.acc ?? position.acc ?? null,
                  heading: position.heading,
                  moving: position.speedKmh >= MOVING_SPEED_KMH,
                  selected: vehicle.id === selectedId,
                  blocked: vehicle.state?.relayOn === true,
                  online: vehicle.device?.status === 'ONLINE',
                })}
                eventHandlers={{ click: () => onSelect?.(vehicle.id) }}
              >
                <Popup>
                  <PositionPopup title={vehicle.name} position={position} />
                </Popup>
              </Marker>
            );
          })}

        <MapFocus
          center={center}
          enabled={following && !track}
          fitTo={track && track.length > 1 ? trackLine : undefined}
        />
      </MapContainer>

      {located.length === 0 && !track && (
        <div className={styles.empty}>Nenhum veículo com posição conhecida ainda</div>
      )}

      {showControls && (
        <div className={styles.overlay}>
          <div className={styles.controlGroup}>
            <button
              type="button"
              className={`${styles.controlButton} ${following ? styles.controlActive : ''}`}
              onClick={() => setFollowing((value) => !value)}
              title="Centralizar o mapa no veículo selecionado a cada nova posição"
            >
              {following ? 'Seguindo' : 'Seguir'}
            </button>
            <button
              type="button"
              className={`${styles.controlButton} ${showFences ? styles.controlActive : ''}`}
              onClick={() => setShowFences((value) => !value)}
            >
              Cercas
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function PositionPopup({
  title,
  position,
  showAddress = true,
}: {
  title: string;
  position: Position;
  /** Falso no marcador de reprodução: evita uma consulta de endereço a
   * cada quadro enquanto o histórico é reproduzido/arrastado. */
  showAddress?: boolean;
}) {
  return (
    <div className={styles.popup}>
      <span className={styles.popupTitle}>{title}</span>
      {position.relayOn && <span className={styles.popupBlocked}>🔒 Motor bloqueado</span>}
      <div className={styles.popupRow}>
        <span>Velocidade</span>
        <span className={styles.popupValue}>{formatSpeed(position.speedKmh)}</span>
      </div>
      <div className={styles.popupRow}>
        <span>Direção</span>
        <span className={styles.popupValue}>{formatHeading(position.heading)}</span>
      </div>
      <div className={styles.popupRow}>
        <span>Ignição</span>
        <span className={styles.popupValue}>
          {position.acc === null ? '—' : position.acc ? 'ligada' : 'desligada'}
        </span>
      </div>
      <div className={styles.popupRow}>
        <span>Local</span>
        <span className={styles.popupValue}>
          {showAddress ? (
            <Address lat={position.latitude} lon={position.longitude} />
          ) : (
            formatCoordinates(position.latitude, position.longitude)
          )}
        </span>
      </div>
      <div className={styles.popupRow}>
        <span>Data do GPS</span>
        <span className={styles.popupValue}>{formatDateTime(position.gpsTimestamp)}</span>
      </div>
    </div>
  );
}

/** Centraliza o mapa sem recriar o container a cada atualização. */
function MapFocus({
  center,
  enabled,
  fitTo,
}: {
  center: LatLngTuple;
  enabled: boolean;
  fitTo?: LatLngExpression[];
}) {
  const map = useMap();

  useEffect(() => {
    if (!fitTo || fitTo.length < 2) return;
    map.fitBounds(L.latLngBounds(fitTo), { padding: [40, 40] });
  }, [map, fitTo]);

  useEffect(() => {
    if (!enabled || fitTo) return;
    map.setView(center, map.getZoom(), { animate: true });
  }, [map, center[0], center[1], enabled, fitTo]);

  return null;
}

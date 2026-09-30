import type { ReactNode } from 'react';

import { CHIP_STEPS, TRACKER_STEPS, reachedAt, stepIndex, trackingUrl } from '@/services/fulfillment';
import type { Step } from '@/services/fulfillment';
import type { ChipStatus, FulfillmentEvent, FulfillmentTrack, TrackerStatus } from '@/types';

import styles from './Fulfillment.module.css';

const stepDate = new Intl.DateTimeFormat('pt-BR', {
  day: '2-digit',
  month: '2-digit',
  hour: '2-digit',
  minute: '2-digit',
});

function when(iso: string | undefined): string {
  if (!iso) return '';
  return stepDate.format(new Date(iso)).replace(',', ' às');
}

/**
 * As duas linhas do tempo do pedido — o chip M2M e o rastreador —, lado a
 * lado (uma embaixo da outra no celular), com a data de cada etapa.
 */
export function FulfillmentTimeline({
  chipStatus,
  trackerStatus,
  events,
  carrier,
  trackingCode,
  children,
}: {
  chipStatus: ChipStatus;
  trackerStatus: TrackerStatus;
  events: FulfillmentEvent[];
  carrier?: string;
  trackingCode?: string;
  /** Abaixo do rastreador (ações, instaladores). */
  children?: ReactNode;
}) {
  // "Aguardando o chip" só aparece quando aconteceu: é um desvio, não uma
  // etapa de todo pedido.
  const trackerSteps = TRACKER_STEPS.filter(
    (step) => step.status !== 'AWAITING_CHIP' || reachedAt(events, 'TRACKER', 'AWAITING_CHIP'),
  );
  return (
    <div className={styles.timelines}>
      <Track title="Chip M2M" track="CHIP" steps={CHIP_STEPS} current={chipStatus} events={events} />
      <div>
        <Track title="Rastreador" track="TRACKER" steps={trackerSteps} current={trackerStatus} events={events} />
        {trackingCode && (
          <div className={styles.tracking}>
            <span className={styles.trackingLabel}>Código de rastreio{carrier ? ` · ${carrier}` : ''}</span>
            <a href={trackingUrl(trackingCode)} target="_blank" rel="noreferrer" className={styles.trackingCode}>
              {trackingCode} ↗
            </a>
          </div>
        )}
        {children}
      </div>
    </div>
  );
}

function Track({
  title,
  track,
  steps,
  current,
  events,
}: {
  title: string;
  track: FulfillmentTrack;
  steps: Step<string>[];
  current: string;
  events: FulfillmentEvent[];
}) {
  const currentIndex = stepIndex(track, current);
  return (
    <section className={styles.track} aria-label={title}>
      <h3 className={styles.trackTitle}>{title}</h3>
      <ol className={styles.steps}>
        {steps.map((step) => {
          const index = stepIndex(track, step.status);
          const state = index < currentIndex ? 'done' : index === currentIndex ? 'current' : 'upcoming';
          const final = index === stepIndex(track, steps[steps.length - 1].status);
          return (
            <li
              key={step.status}
              className={`${styles.step} ${styles[state]} ${state === 'current' && final ? styles.finished : ''}`}
              aria-current={state === 'current' ? 'step' : undefined}
            >
              <span className={styles.marker} aria-hidden="true">
                {state === 'done' || (state === 'current' && final) ? '✓' : ''}
              </span>
              <div className={styles.stepBody}>
                <span className={styles.stepLabel}>{step.label}</span>
                {state !== 'upcoming' && <span className={styles.stepDate}>{when(reachedAt(events, track, step.status))}</span>}
              </div>
            </li>
          );
        })}
      </ol>
    </section>
  );
}

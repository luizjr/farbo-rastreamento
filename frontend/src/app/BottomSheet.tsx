import { useCallback, useEffect, useLayoutEffect, useRef, useState } from 'react';
import type { PointerEvent as ReactPointerEvent, ReactNode } from 'react';

import { haptic } from './haptics';
import { computeSnaps, pickSnap } from './sheet';
import styles from './BottomSheet.module.css';

interface BottomSheetProps {
  title: ReactNode;
  aside?: ReactNode;
  children: ReactNode;
  label: string;
  /** Altura visível ao parar (px): o mapa atrás se ajusta a ela. */
  onSettle?: (visible: number, half: number) => void;
}

/**
 * Lista que desliza sobre o mapa, como no Apple Maps e no Google Maps: arrasta
 * pela alça (ou pela lista, enquanto não está aberta) e para na posição mais
 * próxima. Um toque na alça alterna entre o meio e aberta/recolhida.
 */
export function BottomSheet({ title, aside, children, label, onSettle }: BottomSheetProps) {
  const sheetRef = useRef<HTMLDivElement>(null);
  const headerRef = useRef<HTMLDivElement>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const contentRef = useRef<HTMLDivElement>(null);
  const [snaps, setSnaps] = useState<number[]>([0]);
  const [available, setAvailable] = useState(0);
  const [index, setIndex] = useState(1);
  const drag = useRef<{ startY: number; startVisible: number; lastY: number; lastT: number; velocity: number; active: boolean } | null>(null);
  const suppressClick = useRef(false);

  const snapIndex = Math.min(index, snaps.length - 1);
  const visible = snaps[snapIndex] ?? 0;
  const open = snapIndex === snaps.length - 1 && snaps.length > 2;

  // Mede a área disponível, o título e o conteúdo; refaz quando mudam.
  useLayoutEffect(() => {
    const sheet = sheetRef.current;
    const header = headerRef.current;
    const content = contentRef.current;
    if (!sheet || !header || !content) return;
    const measure = () => {
      const next = computeSnaps({ available: sheet.offsetHeight, header: header.offsetHeight, content: content.offsetHeight });
      setAvailable(sheet.offsetHeight);
      setSnaps((current) => (current.join() === next.join() ? current : next));
    };
    measure();
    const observer = new ResizeObserver(measure);
    [sheet, header, content].forEach((el) => observer.observe(el));
    return () => observer.disconnect();
  }, []);

  const place = useCallback(
    (height: number, animate: boolean) => {
      const sheet = sheetRef.current;
      if (!sheet) return;
      sheet.style.transition = animate ? '' : 'none';
      sheet.style.transform = `translate3d(0, ${Math.max(0, available - height)}px, 0)`;
    },
    [available],
  );

  useLayoutEffect(() => {
    place(visible, true);
  }, [place, visible]);

  useEffect(() => {
    if (visible > 0) onSettle?.(visible, snaps[Math.min(1, snaps.length - 1)] ?? visible);
  }, [visible, snaps, onSettle]);

  const settle = (next: number) => {
    if (next !== snapIndex) haptic(6);
    setIndex(next);
    place(snaps[next], true);
  };

  const onPointerDown = (event: ReactPointerEvent) => {
    // Gesto novo: o "ignora o próximo clique" do arrasto anterior não vale
    // mais (arrastar com o dedo nem gera clique, e a marca engoliria o toque
    // seguinte num cartão).
    suppressClick.current = false;
    if (event.pointerType === 'mouse' && event.button !== 0) return;
    const inList = listRef.current?.contains(event.target as Node);
    // Aberta, a lista rola normalmente; só a alça e o título arrastam.
    if (open && inList) return;
    drag.current = { startY: event.clientY, startVisible: visible, lastY: event.clientY, lastT: event.timeStamp, velocity: 0, active: false };
  };

  const onPointerMove = (event: ReactPointerEvent) => {
    const d = drag.current;
    if (!d) return;
    const dy = event.clientY - d.startY;
    if (!d.active) {
      if (Math.abs(dy) < 6) return;
      d.active = true;
      sheetRef.current?.setPointerCapture(event.pointerId);
    }
    const dt = Math.max(1, event.timeStamp - d.lastT);
    d.velocity = (event.clientY - d.lastY) / dt;
    d.lastY = event.clientY;
    d.lastT = event.timeStamp;
    const min = snaps[0] * 0.85;
    place(Math.min(available, Math.max(min, d.startVisible - dy)), false);
  };

  const onPointerUp = (event: ReactPointerEvent) => {
    const d = drag.current;
    drag.current = null;
    if (!d?.active) return;
    suppressClick.current = true;
    sheetRef.current?.releasePointerCapture?.(event.pointerId);
    const current = d.startVisible - (event.clientY - d.startY);
    settle(pickSnap(snaps, current, d.velocity));
  };

  const toggle = () => {
    if (snaps.length < 2) return;
    // Recolhida ou aberta → meio; no meio → abre (ou recolhe, se não há mais).
    if (snapIndex !== 1) settle(1);
    else settle(snaps.length > 2 ? 2 : 0);
  };

  return (
    <section
      ref={sheetRef}
      className={`${styles.sheet} ${open ? styles.open : ''}`}
      aria-label={label}
      onPointerDown={onPointerDown}
      onPointerMove={onPointerMove}
      onPointerUp={onPointerUp}
      onPointerCancel={onPointerUp}
      onClickCapture={(event) => {
        // O dedo que arrastou não conta como toque no cartão.
        if (suppressClick.current) {
          suppressClick.current = false;
          event.preventDefault();
          event.stopPropagation();
        }
      }}
    >
      <div ref={headerRef} className={styles.header}>
        <button
          type="button"
          className={styles.grabber}
          onClick={toggle}
          aria-label={open || snapIndex === 1 ? 'Recolher lista' : 'Mostrar lista'}
          aria-expanded={snapIndex > 0}
        >
          <span className={styles.grabberBar} />
        </button>
        <div className={styles.titleRow}>
          <h2 className={styles.title}>{title}</h2>
          {aside}
        </div>
      </div>
      <div ref={listRef} className={styles.list}>
        <div ref={contentRef} className={styles.content}>
          {children}
        </div>
      </div>
    </section>
  );
}

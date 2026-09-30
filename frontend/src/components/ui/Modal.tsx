import { useEffect, useRef } from 'react';
import type { ReactNode } from 'react';
import { createPortal } from 'react-dom';

import styles from './Modal.module.css';

interface ModalProps {
  open: boolean;
  title: ReactNode;
  icon?: ReactNode;
  wide?: boolean;
  onClose: () => void;
  footer?: ReactNode;
  children: ReactNode;
}

export function Modal({ open, title, icon, wide = false, onClose, footer, children }: ModalProps) {
  const dialogRef = useRef<HTMLDivElement>(null);

  // Quem usa o Modal passa onClose como função nova a cada render. Se ela
  // entrasse nas dependências do efeito, cada tecla digitada num campo do
  // diálogo reexecutaria o efeito e devolveria o foco ao diálogo, engolindo o
  // resto do texto. Por isso ela fica numa ref e o efeito só roda ao abrir.
  const onCloseRef = useRef(onClose);
  onCloseRef.current = onClose;

  // Esc fecha; o foco vai para o diálogo para quem navega pelo teclado.
  useEffect(() => {
    if (!open) return;

    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onCloseRef.current();
    };
    document.addEventListener('keydown', onKeyDown);
    // Um campo com autoFocus dentro do diálogo já tem o foco: não tira dele.
    if (!dialogRef.current?.contains(document.activeElement)) {
      dialogRef.current?.focus();
    }

    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = 'hidden';

    return () => {
      document.removeEventListener('keydown', onKeyDown);
      document.body.style.overflow = previousOverflow;
    };
  }, [open]);

  if (!open) return null;

  return createPortal(
    <div
      className={styles.backdrop}
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={dialogRef}
        className={`${styles.dialog} ${wide ? styles.wide : ''}`}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
      >
        <header className={styles.header}>
          {icon && (
            <span className={styles.icon} aria-hidden="true">
              {icon}
            </span>
          )}
          <h2 className={styles.title}>{title}</h2>
        </header>
        <div className={styles.body}>{children}</div>
        {footer && <footer className={styles.footer}>{footer}</footer>}
      </div>
    </div>,
    document.body,
  );
}

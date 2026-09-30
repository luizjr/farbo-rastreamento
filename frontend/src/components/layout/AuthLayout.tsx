import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

import styles from './AuthLayout.module.css';

interface AuthLayoutProps {
  /** Rótulo pequeno em verde acima do título, como na landing. */
  tag: string;
  title: string;
  subtitle?: ReactNode;
  /** Ícone de estado (e-mail enviado, sucesso, erro) acima do título. */
  icon?: ReactNode;
  iconTone?: 'accent' | 'danger';
  /** Links abaixo do cartão; o padrão é voltar para o site. */
  footer?: ReactNode;
  children: ReactNode;
}

/**
 * Moldura das telas de acesso (login, esqueci a senha, redefinição).
 *
 * Sempre escura, como a landing: a logo tem letras brancas.
 */
export function AuthLayout({
  tag,
  title,
  subtitle,
  icon,
  iconTone = 'accent',
  footer,
  children,
}: AuthLayoutProps) {
  return (
    <div className={styles.page} data-theme="dark">
      <div className={styles.content}>
        <Link to="/" className={styles.logoLink}>
          <img src="/assets/logo-header.png" alt="Farbo Rastreadores" className={styles.logo} />
        </Link>

        <div className={styles.card}>
          <div className={styles.heading}>
            {icon && (
              <div
                className={`${styles.statusIcon} ${iconTone === 'danger' ? styles.statusIconDanger : ''}`}
                aria-hidden="true"
              >
                {icon}
              </div>
            )}
            <span className={styles.tag}>{tag}</span>
            <h1 className={styles.title}>{title}</h1>
            {subtitle && <p className={styles.subtitle}>{subtitle}</p>}
          </div>

          {children}
        </div>

        {footer ?? (
          <Link to="/" className={styles.back}>
            ← Voltar para o site
          </Link>
        )}
      </div>
    </div>
  );
}

export const authStyles = styles;

/* Ícones das telas de acesso (traço de 24px, mesmo desenho da landing). */

function Icon({ children }: { children: ReactNode }) {
  return (
    <svg
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="2"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {children}
    </svg>
  );
}

export const MailIcon = () => (
  <Icon>
    <rect width="20" height="16" x="2" y="4" rx="2" />
    <path d="m22 7-8.97 5.7a1.94 1.94 0 0 1-2.06 0L2 7" />
  </Icon>
);

export const CheckIcon = () => (
  <Icon>
    <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
    <path d="m9 12 2 2 4-4" />
  </Icon>
);

export const AlertIcon = () => (
  <Icon>
    <circle cx="12" cy="12" r="10" />
    <path d="M12 8v4" />
    <path d="M12 16h.01" />
  </Icon>
);

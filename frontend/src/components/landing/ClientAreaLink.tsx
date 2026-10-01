import { useSyncExternalStore } from 'react';
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';

/**
 * Celular (e tablet em pé): tela de toque até 1024px. É onde o app do
 * cliente (/app, feito para o celular) é melhor que o painel.
 */
export const MOBILE_QUERY = '(pointer: coarse) and (max-width: 1024px)';

/** Destino da "Área do cliente": o app no celular, o painel no computador. */
export function clientAreaHref(mobile: boolean): string {
  return mobile
    ? import.meta.env.VITE_CUSTOMER_APP_URL || '/app/'
    : import.meta.env.VITE_PANEL_URL || '/login';
}

function subscribe(onChange: () => void) {
  const media = window.matchMedia?.(MOBILE_QUERY);
  media?.addEventListener('change', onChange);
  return () => media?.removeEventListener('change', onChange);
}

const isMobile = () => window.matchMedia?.(MOBILE_QUERY).matches ?? false;

interface ClientAreaLinkProps {
  className?: string;
  onClick?: () => void;
  children: ReactNode;
  'aria-label'?: string;
}

/**
 * Link da "Área do cliente". No celular abre o app (outra página, por isso um
 * <a> comum, que carrega o app); no computador, o login do painel pela
 * navegação interna.
 */
export function ClientAreaLink({ className, onClick, children, ...rest }: ClientAreaLinkProps) {
  const mobile = useSyncExternalStore(subscribe, isMobile, () => false);
  const href = clientAreaHref(mobile);
  if (mobile) {
    return (
      <a href={href} className={className} onClick={onClick} {...rest}>
        {children}
      </a>
    );
  }
  return (
    <Link to={href} className={className} onClick={onClick} {...rest}>
      {children}
    </Link>
  );
}

import { useEffect, useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { NavLink, Outlet } from 'react-router-dom';

import { meApi } from '@/api/resources';
import { useRealtime } from '@/hooks/useRealtime';

import { BellIcon, CarIcon, MapIcon, ReceiptIcon, UserIcon } from './icons';
import { applyUpdate, usePwa } from './pwa';
import styles from './AppLayout.module.css';

const TABS = [
  { to: '/mapa', label: 'Mapa', icon: MapIcon, end: true },
  { to: '/meus-veiculos', label: 'Veículos', icon: CarIcon, end: false },
  { to: '/alertas', label: 'Alertas', icon: BellIcon, end: false },
  { to: '/faturas', label: 'Faturas', icon: ReceiptIcon, end: false },
  { to: '/conta', label: 'Conta', icon: UserIcon, end: false },
];

function useOnline(): boolean {
  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => {
    const on = () => setOnline(true);
    const off = () => setOnline(false);
    window.addEventListener('online', on);
    window.addEventListener('offline', off);
    return () => {
      window.removeEventListener('online', on);
      window.removeEventListener('offline', off);
    };
  }, []);
  return online;
}

/** Moldura do app: barra do topo, avisos, conteúdo e abas embaixo. */
export function AppLayout() {
  const online = useOnline();
  const { connected } = useRealtime();
  const { updateReady } = usePwa();
  const account = useQuery({ queryKey: ['me', 'account'], queryFn: meApi.account, staleTime: 60_000 });
  const overdue = account.data?.overdueInvoices ?? 0;
  const liveLabel = online && connected ? 'Ao vivo' : online ? 'Conectando…' : 'Offline';

  return (
    <div className={styles.app}>
      <header className={styles.topbar}>
        {/* A logo completa, centralizada; o ícone da marca só entra quando
            a tela é estreita demais para ela (ver o <source>). */}
        <picture className={styles.logo}>
          <source media="(max-width: 259px)" srcSet="/assets/logo-mark.png" />
          <img src="/assets/logo-header.png" alt="Farbo Rastreadores" />
        </picture>
        <span
          className={`${styles.live} ${online && connected ? styles.liveOn : ''}`}
          title={liveLabel}
          aria-label={liveLabel}
        >
          <span className={styles.liveLabel}>{liveLabel}</span>
        </span>
      </header>

      {!online && (
        <div className={styles.banner} role="status">
          Sem internet: você está vendo os últimos dados guardados neste celular.
        </div>
      )}
      {account.data?.suspended && (
        <NavLink to="/faturas" className={`${styles.banner} ${styles.bannerDanger}`}>
          Acesso suspenso por fatura vencida. Toque para pagar e voltar a ver o mapa.
        </NavLink>
      )}
      {updateReady && (
        <button type="button" className={`${styles.banner} ${styles.bannerAccent}`} onClick={applyUpdate}>
          Nova versão do app disponível — toque para atualizar
        </button>
      )}

      <main className={styles.content}>
        <Outlet />
      </main>

      <nav className={styles.tabbar} aria-label="Navegação principal">
        {TABS.map(({ to, label, icon: Icon, end }) => (
          <NavLink key={to} to={to} end={end} className={({ isActive }) => `${styles.tab} ${isActive ? styles.tabActive : ''}`}>
            <span className={styles.tabIcon}>
              <Icon />
              {to === '/faturas' && overdue > 0 && <span className={styles.dot} aria-label={`${overdue} fatura(s) vencida(s)`} />}
            </span>
            <span className={styles.tabLabel}>{label}</span>
          </NavLink>
        ))}
      </nav>
    </div>
  );
}

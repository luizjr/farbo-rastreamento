import React, { useEffect, useMemo, useState } from 'react';
import { useQuery } from '@tanstack/react-query';

import { publicApi } from '@/api/resources';
import type { PublicInstaller } from '@/types';

import styles from './InstallersModal.module.css';
import { useModalBehavior } from './useModalBehavior';

type Filter = 'todos' | 'moto' | 'carro';

interface InstallersModalProps {
  isOpen: boolean;
  onClose: () => void;
  /** Filtro inicial (o card clicado na landing, por exemplo). */
  initialFilter?: Filter;
}

const money = new Intl.NumberFormat('pt-BR', { style: 'currency', currency: 'BRL' });

/**
 * Prestadores de instalação recomendados. A instalação é combinada e paga
 * direto com o prestador, pelo WhatsApp dele. A lista é mantida pela central
 * no painel (menu Prestadores).
 */
export const InstallersModal: React.FC<InstallersModalProps> = ({ isOpen, onClose, initialFilter = 'todos' }) => {
  const [filter, setFilter] = useState<Filter>(initialFilter);
  const [search, setSearch] = useState('');

  const installers = useQuery({
    queryKey: ['public', 'installers'],
    queryFn: publicApi.installers,
    enabled: isOpen,
    staleTime: 60_000,
  });

  useModalBehavior(isOpen, onClose);

  useEffect(() => {
    if (!isOpen) return;
    setFilter(initialFilter);
    setSearch('');
  }, [isOpen, initialFilter]);

  const visible = useMemo(() => {
    const term = search.trim().toLowerCase();
    return (installers.data ?? []).filter((i) => {
      if (filter === 'moto' && !i.servesMoto) return false;
      if (filter === 'carro' && !i.servesCar) return false;
      if (!term) return true;
      return [i.name, i.city, i.serviceArea].some((value) => value.toLowerCase().includes(term));
    });
  }, [installers.data, filter, search]);

  if (!isOpen) return null;

  return (
    <div className={styles.overlay} onClick={onClose}>
      <div
        className={styles.modal}
        role="dialog"
        aria-modal="true"
        aria-labelledby="installers-title"
        onClick={(e) => e.stopPropagation()}
      >
        <button className={styles.closeBtn} onClick={onClose} aria-label="Fechar">
          ✕
        </button>

        <div className={styles.header}>
          <div className={styles.badge}>Instalação profissional</div>
          <h2 id="installers-title">Prestadores recomendados</h2>
          <p>
            A instalação é feita por técnicos parceiros e <strong>paga direto a eles</strong>. Chame
            pelo WhatsApp para combinar o horário e o valor.
          </p>
        </div>

        <div className={styles.toolbar}>
          <div className={styles.filters} role="group" aria-label="Tipo de veículo">
            {(['todos', 'moto', 'carro'] as const).map((option) => (
              <button
                key={option}
                type="button"
                className={filter === option ? styles.activeFilter : ''}
                aria-pressed={filter === option}
                onClick={() => setFilter(option)}
              >
                {option === 'todos' ? 'Todos' : option === 'moto' ? '🏍 Moto' : '🚗 Carro'}
              </button>
            ))}
          </div>
          <input
            className={styles.search}
            type="search"
            placeholder="Buscar por cidade ou região"
            aria-label="Buscar por cidade ou região"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>

        <div className={styles.list}>
          {installers.isLoading && <p className={styles.empty}>Carregando prestadores…</p>}
          {installers.isError && (
            <p className={styles.empty}>Não foi possível carregar a lista agora. Tente de novo em instantes.</p>
          )}
          {installers.data && visible.length === 0 && (
            <p className={styles.empty}>
              {(installers.data ?? []).length === 0
                ? 'Estamos cadastrando os prestadores parceiros. Fale com a gente pelo WhatsApp para indicarmos um na sua região.'
                : 'Nenhum prestador com esse filtro. Tente outra cidade ou tipo de veículo.'}
            </p>
          )}
          {visible.map((installer) => (
            <InstallerCard key={installer.id} installer={installer} filter={filter} />
          ))}
        </div>
      </div>
    </div>
  );
};

function InstallerCard({ installer, filter }: { installer: PublicInstaller; filter: Filter }) {
  const vehicle = filter === 'carro' || (!installer.servesMoto && installer.servesCar) ? 'carro' : 'moto';
  const message = encodeURIComponent(
    `Olá, ${installer.name}! Vi sua indicação no site da Farbo Rastreadores e quero agendar ` +
      `a instalação de um rastreador no meu ${vehicle}.`,
  );

  return (
    <article className={styles.card}>
      <div className={styles.cardHead}>
        <div>
          <h3>{installer.name}</h3>
          {installer.city && <span className={styles.city}>📍 {installer.city}</span>}
        </div>
        <div className={styles.services}>
          {installer.servesMoto && <span>Moto</span>}
          {installer.servesCar && <span>Carro</span>}
        </div>
      </div>

      {installer.serviceArea && <p className={styles.area}>Atende: {installer.serviceArea}</p>}
      {installer.description && <p className={styles.description}>{installer.description}</p>}

      <div className={styles.cardFoot}>
        <div className={styles.prices}>
          {installer.servesMoto && installer.priceMotoCents !== null && (
            <span>
              Moto <strong>{money.format(installer.priceMotoCents / 100)}</strong>
            </span>
          )}
          {installer.servesCar && installer.priceCarCents !== null && (
            <span>
              Carro <strong>{money.format(installer.priceCarCents / 100)}</strong>
            </span>
          )}
          {installer.priceMotoCents === null && installer.priceCarCents === null && (
            <span className={styles.combine}>Valor a combinar</span>
          )}
        </div>
        <a
          className={styles.whatsapp}
          href={`https://wa.me/${installer.whatsapp}?text=${message}`}
          target="_blank"
          rel="noopener noreferrer"
        >
          Chamar no WhatsApp
        </a>
      </div>
    </article>
  );
}

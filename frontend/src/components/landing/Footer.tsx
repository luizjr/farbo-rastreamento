import React, { useState } from 'react';
import styles from './Footer.module.css';

export const Footer: React.FC = () => {
  const [logoError, setLogoError] = useState(false);

  return (
    <footer className={styles.footer}>
      <div className={styles.container}>
        <div className={styles.topRow}>
          <div className={styles.brandCol}>
            <a href="#" className={styles.logoLink}>
              {logoError ? (
                <div className={styles.logoText}>
                  <span className={styles.brandTitle}>FARBO</span>
                  <span className={styles.brandSubtitle}>RASTREADORES</span>
                </div>
              ) : (
                <img
                  src="/assets/logo-header.png"
                  alt="Farbo Rastreadores"
                  className={styles.logoImg}
                  onError={() => setLogoError(true)}
                />
              )}
            </a>
            <p className={styles.brandDesc}>
              Tecnologia de rastreamento veicular em tempo real para carros e motos. Liberdade com mais segurança.
            </p>
          </div>

          <div className={styles.linksCol}>
            <h4>Navegação</h4>
            <ul>
              <li><a href="#beneficios">Benefícios</a></li>
              <li><a href="#planos">Planos e Preços</a></li>
              <li><a href="#como-funciona">Como Funciona</a></li>
              <li><a href="#depoimentos">Depoimentos</a></li>
            </ul>
          </div>

          <div className={styles.linksCol}>
            <h4>Parceiros</h4>
            <ul>
              <li>Motoclube Insanos MC</li>
              <li>Chip M2M Multi-operadora</li>
              <li>Rastreador J16 GT06</li>
            </ul>
          </div>

          <div className={styles.linksCol}>
            <h4>Atendimento</h4>
            <ul>
              <li>WhatsApp: (11) 99999-9999</li>
              <li>E-mail: contato@farborastreadores.com.br</li>
              <li>Atendimento Seg-Sáb: 08h às 20h</li>
            </ul>
          </div>
        </div>

        <div className={styles.bottomBar}>
          <span>© {new Date().getFullYear()} Farbo Rastreadores. Todos os direitos reservados.</span>
          <div className={styles.legalLinks}>
            <a href="#">Termos de Uso</a>
            <a href="#">Política de Privacidade</a>
          </div>
        </div>
      </div>
    </footer>
  );
};

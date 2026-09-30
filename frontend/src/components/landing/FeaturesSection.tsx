import React from 'react';
import styles from './FeaturesSection.module.css';

export const FeaturesSection: React.FC = () => {
  return (
    <section className={styles.featuresSection}>
      <div className={styles.container}>
        <h2 className={styles.sectionTitle}>TUDO INCLUSO NO SEU PLANO</h2>

        <div className={styles.grid}>
          {/* Feature 1 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <rect width="16" height="10" x="2" y="3" rx="2" />
                <path d="M12 13v4M8 17h8" />
                <rect width="6" height="10" x="16" y="11" rx="1" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Sistema web e aplicativo</span>
          </div>

          <div className={styles.divider}></div>

          {/* Feature 2 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <rect width="14" height="18" x="5" y="3" rx="2" />
                <path d="M9 7h6M9 11h6M9 15h4" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Chip M2M multi operadora 20 MB/mês incluso</span>
          </div>

          <div className={styles.divider}></div>

          {/* Feature 3 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <path d="M20 10c0 6-8 12-8 12s-8-6-8-12a8 8 0 0 1 16 0Z" />
                <circle cx="12" cy="10" r="3" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Localização em tempo real</span>
          </div>

          <div className={styles.divider}></div>

          {/* Feature 4 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <circle cx="6" cy="19" r="3" />
                <circle cx="18" cy="5" r="3" />
                <path d="M9 19h4.5a3.5 3.5 0 0 0 3.5-3.5v-7" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Histórico de rotas</span>
          </div>

          <div className={styles.divider}></div>

          {/* Feature 5 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <path d="M6 8a6 6 0 0 1 12 0c0 7 3 9 3 9H3s3-2 3-9" />
                <path d="M10.3 21a1.94 1.94 0 0 0 3.4 0" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Alertas inteligentes</span>
          </div>

          <div className={styles.divider}></div>

          {/* Feature 6 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <rect width="18" height="11" x="3" y="11" rx="2" />
                <path d="M7 11V7a5 5 0 0 1 10 0v4" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Mais segurança para o seu veículo</span>
          </div>

          <div className={styles.divider}></div>

          {/* Feature 7 */}
          <div className={styles.featureItem}>
            <div className={styles.iconBox}>
              <svg width="26" height="26" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round">
                <path d="M3 14h3a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-5Z" />
                <path d="M18 14h3a2 2 0 0 1 2 2v3a2 2 0 0 1-2 2h-1a2 2 0 0 1-2-2v-5Z" />
                <path d="M3 14v-3a9 9 0 0 1 18 0v3" />
              </svg>
            </div>
            <span className={styles.featureLabel}>Suporte especializado</span>
          </div>
        </div>
      </div>
    </section>
  );
};

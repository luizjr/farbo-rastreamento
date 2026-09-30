import React from 'react';
import styles from './PricingSection.module.css';

interface PricingSectionProps {
  onOpenModal: (plan?: string) => void;
}

export const PricingSection: React.FC<PricingSectionProps> = ({ onOpenModal }) => {
  return (
    <section className={styles.pricingSection} id="planos">
      <div className={styles.container}>
        <div className={styles.cardsGrid}>
          {/* Card 1: PLANO MENSAL */}
          <div className={styles.card}>
            <div className={styles.cardHeader}>
              <h3 className={styles.cardTitle}>PLANO MENSAL</h3>
              <div className={styles.priceStrikethrough}>
                De R$ <span>119,90</span>
              </div>
              <div className={styles.priceMain}>
                <span className={styles.currency}>R$</span>
                <span className={styles.amount}>69</span>
                <span className={styles.cents}>,90</span>
              </div>
              <div className={styles.priceSub}>/mês por veículo</div>
            </div>

            <ul className={styles.checkList}>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Rastreamento em tempo real
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                App e sistema web inclusos
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Chip M2M multi operadora com 20 MB/mês incluso
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Histórico de rotas
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Alertas e notificações
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Suporte especializado
              </li>
            </ul>

            <button
              className={styles.whiteBtn}
              onClick={() => onOpenModal('Plano Mensal - R$ 69,90')}
            >
              Quero esse plano
            </button>
          </div>

          {/* Card 2: PREÇO ESPECIAL (MOTOCLUBE INSANOS) */}
          <div className={`${styles.card} ${styles.featuredCard}`}>
            <div className={styles.badgeTop}>PREÇO ESPECIAL</div>

            <div className={styles.cardHeader}>
              <div className={styles.insanosSubtitle}>
                INTEGRANTES DO MOTOCLUBE INSANOS
              </div>
              <div className={styles.priceMainFeatured}>
                <span className={styles.currency}>R$</span>
                <span className={styles.amount}>39</span>
                <span className={styles.cents}>,90</span>
              </div>
              <div className={styles.priceSubFeatured}>/mês por veículo</div>
            </div>

            <ul className={styles.checkList}>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Rastreamento em tempo real
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                App e sistema web inclusos
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Chip M2M multi operadora com 20 MB/mês incluso
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Histórico de rotas
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Alertas e notificações
              </li>
              <li>
                <span className={styles.checkIcon}>✓</span>
                Suporte especializado
              </li>
            </ul>

            <button
              className={styles.neonBtn}
              onClick={() => onOpenModal('Preço Especial Insanos MC - R$ 39,90')}
            >
              Quero meu desconto
            </button>

            <div className={styles.partnerFooter}>
              <img
                src="/assets/insanos-skull.png"
                alt="Insanos MC Logo"
                className={styles.partnerLogo}
              />
              <div className={styles.partnerText}>
                <strong>INSANOS MC</strong>
                <span>
                  PARCERIA OFICIAL<br />
                  FARBO RASTREADORES<br />
                  MOTOCLUBE INSANOS
                </span>
              </div>
            </div>
          </div>

          {/* Card 3: EQUIPAMENTO */}
          <div className={`${styles.card} ${styles.equipmentCard}`}>
            <div className={styles.equipmentContent}>
              <div className={styles.cardHeader}>
                <h3 className={styles.cardTitle}>EQUIPAMENTO</h3>
                <div className={styles.deviceSubtitle}>Rastreador J16 GT06</div>
                <div className={styles.priceMain}>
                  <span className={styles.currency}>R$</span>
                  <span className={styles.amount}>150</span>
                  <span className={styles.cents}>,00</span>
                </div>
                <div className={styles.priceSub}>Pagamento único</div>
              </div>

              <ul className={styles.checkList}>
                <li>
                  <span className={styles.checkIcon}>✓</span>
                  Alta precisão GPS
                </li>
                <li>
                  <span className={styles.checkIcon}>✓</span>
                  Bloqueio remoto (opcional)
                </li>
                <li>
                  <span className={styles.checkIcon}>✓</span>
                  Suporte a comandos
                </li>
                <li>
                  <span className={styles.checkIcon}>✓</span>
                  Resistente e confiável
                </li>
                <li>
                  <span className={styles.checkIcon}>✓</span>
                  Desbloqueado
                </li>
              </ul>
            </div>

            <div className={styles.equipmentImageWrapper}>
              <img
                src="/assets/tracker-gt06.png"
                alt="Rastreador J16 GT06"
                className={styles.trackerImg}
              />
            </div>
          </div>
        </div>
      </div>
    </section>
  );
};

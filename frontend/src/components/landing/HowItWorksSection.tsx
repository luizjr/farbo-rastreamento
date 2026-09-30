import React from 'react';
import styles from './HowItWorksSection.module.css';

export const HowItWorksSection: React.FC = () => {
  return (
    <section className={styles.section} id="como-funciona">
      <div className={styles.container}>
        <div className={styles.header}>
          <span className={styles.badge}>SIMPLES E RÁPIDO</span>
          <h2 className={styles.title}>Como Funciona o Rastreamento Farbo</h2>
          <p className={styles.subtitle}>
            Em apenas 3 passos simples você garante a proteção total do seu veículo.
          </p>
        </div>

        <div className={styles.stepsGrid}>
          {/* Step 1 */}
          <div className={styles.stepCard}>
            <div className={styles.stepNumber}>01</div>
            <h3 className={styles.stepTitle}>Escolha o seu Plano</h3>
            <p className={styles.stepDesc}>
              Selecione o plano mensal ideal para seu carro ou moto, ou aproveite os descontos especiais para integrantes do Motoclube Insanos.
            </p>
          </div>

          {/* Step 2 */}
          <div className={styles.stepCard}>
            <div className={styles.stepNumber}>02</div>
            <h3 className={styles.stepTitle}>Instalação ou Envio</h3>
            <p className={styles.stepDesc}>
              Agende a instalação profissional com nossa equipe especializada ou receba o equipamento rastreador J16 GT06 pré-configurado.
            </p>
          </div>

          {/* Step 3 */}
          <div className={styles.stepCard}>
            <div className={styles.stepNumber}>03</div>
            <h3 className={styles.stepTitle}>Monitore em Tempo Real</h3>
            <p className={styles.stepDesc}>
              Baixe nosso aplicativo web/mobile, acesse o mapa com dados de velocidade, histórico e receba alertas de ignição e cercas virtuais.
            </p>
          </div>
        </div>
      </div>
    </section>
  );
};

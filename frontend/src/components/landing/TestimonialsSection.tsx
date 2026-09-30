import React from 'react';
import styles from './TestimonialsSection.module.css';

export const TestimonialsSection: React.FC = () => {
  const reviews = [
    {
      name: 'Carlos "Caveira" Silva',
      role: 'Integrante Insanos MC - Regional SP',
      text: 'O rastreador Farbo é indispensável nas viagens de moto com o clube. A precisão do GPS e o suporte para o Insanos MC são excelentes!',
      rating: 5,
      vehicle: 'Honda CB 500X',
    },
    {
      name: 'Fernando Mendes',
      role: 'Proprietário de Veículo',
      text: 'Super recomendo! O app é muito rápido, consigo ver o mapa na hora e receber alertas no celular se o carro for ligado fora do horário.',
      rating: 5,
      vehicle: 'Toyota Corolla 2023',
    },
    {
      name: 'Renata Albuquerque',
      role: 'Gestora de Frota Urbana',
      text: 'Instalei nos 5 carros da nossa empresa. Histórico de rotas perfeito, relatório detalhado de velocidade e suporte muito atencioso.',
      rating: 5,
      vehicle: 'Frota Comercial (5x)',
    },
  ];

  return (
    <section className={styles.section} id="depoimentos">
      <div className={styles.container}>
        <div className={styles.header}>
          <span className={styles.badge}>AVALIAÇÕES REAIS</span>
          <h2 className={styles.title}>Quem Usa Recomenda</h2>
          <p className={styles.subtitle}>
            Confira o depoimento de motociclistas e motoristas que confiam na segurança Farbo.
          </p>
        </div>

        <div className={styles.grid}>
          {reviews.map((rev, index) => (
            <div key={index} className={styles.card}>
              <div className={styles.stars}>
                {'★'.repeat(rev.rating)}
              </div>
              <p className={styles.text}>"{rev.text}"</p>
              <div className={styles.authorInfo}>
                <div className={styles.avatar}>
                  {rev.name.charAt(0)}
                </div>
                <div>
                  <h4 className={styles.name}>{rev.name}</h4>
                  <span className={styles.role}>{rev.role} • <strong>{rev.vehicle}</strong></span>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
};

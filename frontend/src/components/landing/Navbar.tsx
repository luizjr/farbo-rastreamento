import React, { useState, useEffect } from 'react';
import { Link } from 'react-router-dom';
import styles from './Navbar.module.css';

interface NavbarProps {
  onOpenModal: (plan?: string) => void;
}

export const Navbar: React.FC<NavbarProps> = ({ onOpenModal }) => {
  const [scrolled, setScrolled] = useState(false);
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const [logoError, setLogoError] = useState(false);

  useEffect(() => {
    const handleScroll = () => {
      setScrolled(window.scrollY > 20);
    };
    window.addEventListener('scroll', handleScroll);
    return () => window.removeEventListener('scroll', handleScroll);
  }, []);

  const toggleMobileMenu = () => {
    setMobileMenuOpen(!mobileMenuOpen);
  };

  const closeMenu = () => {
    setMobileMenuOpen(false);
  };

  return (
    <header className={`${styles.header} ${scrolled ? styles.headerScrolled : ''}`}>
      <div className={styles.container}>
        <a href="#" className={styles.logoLink} onClick={closeMenu}>
          {logoError ? (
            <div className={styles.logoFallbackText}>
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

        <nav className={`${styles.nav} ${mobileMenuOpen ? styles.navOpen : ''}`}>
          <a href="#beneficios" onClick={closeMenu}>Benefícios</a>
          <a href="#planos" onClick={closeMenu}>Planos</a>
          <a href="#como-funciona" onClick={closeMenu}>Como funciona</a>
          <a href="#depoimentos" onClick={closeMenu}>Depoimentos</a>
          <a href="#contato" onClick={closeMenu}>Contato</a>
          <Link to="/login" className={styles.mobileLoginLink} onClick={closeMenu}>
            <UserIcon />
            Área do cliente
          </Link>
          <button
            className={styles.mobileCtaBtn}
            onClick={() => {
              closeMenu();
              onOpenModal('Quero meu rastreador');
            }}
          >
            Quero meu rastreador
          </button>
        </nav>

        <div className={styles.rightActions}>
          <Link to="/login" className={styles.loginLink} aria-label="Área do cliente">
            <UserIcon />
            <span>Área do cliente</span>
          </Link>

          <button
            className={styles.ctaButton}
            onClick={() => onOpenModal('Quero meu rastreador')}
          >
            Quero meu rastreador
          </button>

          <button
            className={styles.hamburger}
            onClick={toggleMobileMenu}
            aria-label="Abrir menu"
            aria-expanded={mobileMenuOpen}
          >
            <span className={`${styles.bar} ${mobileMenuOpen ? styles.bar1Open : ''}`}></span>
            <span className={`${styles.bar} ${mobileMenuOpen ? styles.bar2Open : ''}`}></span>
            <span className={`${styles.bar} ${mobileMenuOpen ? styles.bar3Open : ''}`}></span>
          </button>
        </div>
      </div>
    </header>
  );
};

const UserIcon: React.FC = () => (
  <svg
    width="18"
    height="18"
    viewBox="0 0 24 24"
    fill="none"
    stroke="currentColor"
    strokeWidth="2"
    strokeLinecap="round"
    strokeLinejoin="round"
    aria-hidden="true"
  >
    <circle cx="12" cy="8" r="4" />
    <path d="M4 21a8 8 0 0 1 16 0" />
  </svg>
);

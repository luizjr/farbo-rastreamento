import { useState } from 'react';
import type { FormEvent } from 'react';
import { Navigate, useNavigate } from 'react-router-dom';

import { Button } from '@/components/ui/Button';
import { TextField } from '@/components/ui/Field';
import { useAuth } from '@/stores/AuthContext';

import styles from './Screen.module.css';

/** Entrar no app: a mesma conta do painel. */
export function LoginScreen() {
  const { user, login } = useAuth();
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);

  if (user) return <Navigate to="/mapa" replace />;

  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    setBusy(true);
    try {
      await login(email.trim(), password);
      navigate('/mapa', { replace: true });
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Não foi possível entrar.');
    } finally {
      setBusy(false);
    }
  };

  return (
    <div className={styles.login}>
      <img src="/assets/logo-header.png" alt="Farbo Rastreadores" className={styles.loginLogo} />
      <div className={styles.loginIntro}>
        <h1 className={styles.title}>Seus veículos na palma da mão</h1>
        <p className={styles.lead}>Entre com o e-mail e a senha da sua conta Farbo.</p>
      </div>
      <form className={styles.form} onSubmit={submit}>
        <TextField label="E-mail" type="email" autoComplete="email" inputMode="email" required
          value={email} onChange={(e) => setEmail(e.target.value)} />
        <TextField label="Senha" type="password" autoComplete="current-password" required
          value={password} onChange={(e) => setPassword(e.target.value)} />
        {error && <p className={styles.error} role="alert">{error}</p>}
        <Button type="submit" size="large" block loading={busy}>
          Entrar
        </Button>
      </form>
      <a className={styles.link} href="/esqueci-senha">
        Esqueci minha senha
      </a>
    </div>
  );
}

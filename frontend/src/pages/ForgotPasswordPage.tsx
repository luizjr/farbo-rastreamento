import { useState } from 'react';
import type { FormEvent } from 'react';
import { Link, useNavigate } from 'react-router-dom';

import { authApi } from '@/api/resources';
import { AuthLayout, MailIcon, authStyles as styles } from '@/components/layout/AuthLayout';
import { Button } from '@/components/ui/Button';
import { TextField } from '@/components/ui/Field';

/**
 * "Esqueci minha senha": pede o link de redefinição por e-mail.
 *
 * A confirmação é a mesma com ou sem conta cadastrada — o backend também
 * responde igual —, para ninguém usar a tela para descobrir e-mails de
 * clientes.
 */
export function ForgotPasswordPage() {
  const navigate = useNavigate();
  const [email, setEmail] = useState('');
  const [error, setError] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [sentTo, setSentTo] = useState<string | null>(null);

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    setError('');
    setSubmitting(true);
    try {
      await authApi.forgotPassword(email.trim());
      setSentTo(email.trim());
    } catch (err) {
      setError(err instanceof Error ? err.message : 'não foi possível enviar o pedido');
    } finally {
      setSubmitting(false);
    }
  };

  const footer = (
    <div className={styles.footerLinks}>
      <Link to="/login" className={styles.back}>
        ← Voltar para o login
      </Link>
    </div>
  );

  if (sentTo !== null) {
    return (
      <AuthLayout tag="Área do cliente" title="Confira seu e-mail" icon={<MailIcon />} footer={footer}>
        <div className={styles.stack}>
          <p className={styles.text}>
            Se houver uma conta com <strong>{sentTo}</strong>, você vai receber em alguns minutos
            um link para criar uma nova senha.
          </p>
          <p className={styles.text}>
            O link vale por tempo limitado e só pode ser usado uma vez. Não chegou? Confira a
            caixa de spam ou{' '}
            <button type="button" className={styles.textButton} onClick={() => setSentTo(null)}>
              peça de novo
            </button>
            .
          </p>
          <Button variant="primary" size="large" block onClick={() => navigate('/login')}>
            Voltar para o login
          </Button>
        </div>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout
      tag="Área do cliente"
      title="Esqueceu sua senha?"
      subtitle="Informe o e-mail da sua conta e enviaremos um link para criar uma nova senha."
      footer={footer}
    >
      <form className={styles.form} onSubmit={onSubmit}>
        {error && <div className={styles.error}>{error}</div>}

        <TextField
          label="E-mail"
          type="email"
          autoComplete="username"
          autoFocus
          required
          value={email}
          onChange={(event) => setEmail(event.target.value)}
        />

        <Button type="submit" variant="primary" size="large" block loading={submitting}>
          Enviar link
        </Button>
      </form>
    </AuthLayout>
  );
}

import { StrictMode } from 'react';
import { createRoot } from 'react-dom/client';

import '../styles/global.css';
import './app.css';
import { AppRoot } from './AppRoot';
import { setupPwa } from './pwa';

// Antes de renderizar: o convite de instalação do navegador chega cedo.
setupPwa();

const container = document.getElementById('root');
if (!container) {
  throw new Error('elemento #root não encontrado');
}

createRoot(container).render(
  <StrictMode>
    <AppRoot />
  </StrictMode>,
);

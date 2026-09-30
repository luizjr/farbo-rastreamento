import { useSyncExternalStore } from 'react';

/**
 * Peças do app instalável: service worker (offline e atualização), convite de
 * instalação e inscrição nas notificações do celular.
 */

// ---------------------------------------------------------------------------
// Estado compartilhado (useSyncExternalStore)
// ---------------------------------------------------------------------------

interface InstallPromptEvent extends Event {
  prompt: () => Promise<void>;
  userChoice: Promise<{ outcome: 'accepted' | 'dismissed' }>;
}

interface PwaState {
  /** Há versão nova esperando para assumir. */
  updateReady: boolean;
  /** O navegador ofereceu instalar (Chrome/Edge/Android). */
  installEvent: InstallPromptEvent | null;
  installed: boolean;
}

let state: PwaState = { updateReady: false, installEvent: null, installed: false };
const listeners = new Set<() => void>();
let waiting: ServiceWorker | null = null;

function set(patch: Partial<PwaState>) {
  state = { ...state, ...patch };
  listeners.forEach((l) => l());
}

function subscribe(listener: () => void) {
  listeners.add(listener);
  return () => listeners.delete(listener);
}

export function usePwa(): PwaState {
  return useSyncExternalStore(subscribe, () => state);
}

// ---------------------------------------------------------------------------
// Plataforma
// ---------------------------------------------------------------------------

/** Aberto pelo ícone da Tela de Início (e não numa aba do navegador). */
export function isStandalone(): boolean {
  return (
    window.matchMedia?.('(display-mode: standalone)').matches ||
    (navigator as Navigator & { standalone?: boolean }).standalone === true
  );
}

/** iPhone/iPad (inclusive iPad que se apresenta como Mac). */
export function isIos(userAgent = navigator.userAgent, touchPoints = navigator.maxTouchPoints): boolean {
  return /iphone|ipad|ipod/i.test(userAgent) || (/macintosh/i.test(userAgent) && touchPoints > 1);
}

// ---------------------------------------------------------------------------
// Service worker e instalação
// ---------------------------------------------------------------------------

/** Chame uma vez, antes de renderizar: o convite de instalação chega cedo. */
export function setupPwa(): void {
  set({ installed: isStandalone() });
  window.addEventListener('beforeinstallprompt', (event) => {
    event.preventDefault(); // o app mostra o próprio botão "Instalar"
    set({ installEvent: event as InstallPromptEvent });
  });
  window.addEventListener('appinstalled', () => set({ installed: true, installEvent: null }));

  // Em desenvolvimento não há sw.js (ele sai do build) e cache só atrapalha.
  if (!('serviceWorker' in navigator) || !import.meta.env.PROD) return;
  navigator.serviceWorker
    .register('/app/sw.js', { scope: '/app/', updateViaCache: 'none' })
    .then((registration) => {
      const watch = (worker: ServiceWorker | null) => {
        if (!worker) return;
        worker.addEventListener('statechange', () => {
          // Instalado com outro já no controle = versão nova esperando.
          if (worker.state === 'installed' && navigator.serviceWorker.controller) {
            waiting = worker;
            set({ updateReady: true });
          }
        });
      };
      if (registration.waiting && navigator.serviceWorker.controller) {
        waiting = registration.waiting;
        set({ updateReady: true });
      }
      registration.addEventListener('updatefound', () => watch(registration.installing));
      // O app fica aberto por dias: procura versão nova de hora em hora.
      window.setInterval(() => registration.update().catch(() => undefined), 60 * 60 * 1000);
    })
    .catch(() => undefined);

  let reloading = false;
  navigator.serviceWorker.addEventListener('controllerchange', () => {
    if (reloading) return;
    reloading = true;
    window.location.reload();
  });
}

/** Troca para a versão nova (a página recarrega sozinha). */
export function applyUpdate(): void {
  waiting?.postMessage({ type: 'SKIP_WAITING' });
}

/** Abre o convite de instalação do navegador. */
export async function promptInstall(): Promise<boolean> {
  const event = state.installEvent;
  if (!event) return false;
  await event.prompt();
  const { outcome } = await event.userChoice;
  set({ installEvent: null });
  return outcome === 'accepted';
}

/** Apaga os dados da conta guardados para uso offline (ao sair). */
export function clearOfflineData(): void {
  navigator.serviceWorker?.controller?.postMessage({ type: 'CLEAR_API_CACHE' });
}

// ---------------------------------------------------------------------------
// Notificações no celular
// ---------------------------------------------------------------------------

export type PushSupport = 'supported' | 'unsupported' | 'ios-needs-install';

export function pushSupport(): PushSupport {
  const capable = 'serviceWorker' in navigator && 'PushManager' in window && 'Notification' in window;
  if (capable) return 'supported';
  // No iPhone, o push só existe no app instalado na Tela de Início (iOS 16.4+).
  if (isIos() && !isStandalone()) return 'ios-needs-install';
  return 'unsupported';
}

/** applicationServerKey (base64url) → bytes, como o PushManager pede. */
export function urlBase64ToUint8Array(value: string): Uint8Array {
  const padded = value + '='.repeat((4 - (value.length % 4)) % 4);
  const raw = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));
  const bytes = new Uint8Array(raw.length);
  for (let i = 0; i < raw.length; i++) bytes[i] = raw.charCodeAt(i);
  return bytes;
}

async function registration(): Promise<ServiceWorkerRegistration> {
  const reg = await navigator.serviceWorker.getRegistration('/app/');
  if (!reg) throw new Error('O app ainda está sendo preparado neste aparelho. Tente de novo em instantes.');
  return reg;
}

/** Inscrição deste aparelho, se houver. */
export async function currentSubscription(): Promise<PushSubscription | null> {
  if (pushSupport() !== 'supported') return null;
  const reg = await navigator.serviceWorker.getRegistration('/app/');
  return (await reg?.pushManager.getSubscription()) ?? null;
}

/** Pede permissão e inscreve este aparelho. Devolve o JSON para o backend. */
export async function subscribePush(publicKey: string): Promise<PushSubscriptionJSON> {
  const permission = await Notification.requestPermission();
  if (permission !== 'granted') {
    throw new Error('As notificações foram bloqueadas. Libere nas configurações do navegador para este site.');
  }
  const reg = await registration();
  const key = urlBase64ToUint8Array(publicKey);
  let subscription = await reg.pushManager.getSubscription();
  // Inscrição feita com outra chave (a central trocou as chaves): refaz.
  const current = subscription?.options.applicationServerKey;
  if (subscription && current && !sameBytes(new Uint8Array(current), key)) {
    await subscription.unsubscribe();
    subscription = null;
  }
  subscription ??= await reg.pushManager.subscribe({ userVisibleOnly: true, applicationServerKey: key });
  return subscription.toJSON();
}

/** Cancela a inscrição deste aparelho; devolve o endpoint para o backend esquecer. */
export async function unsubscribePush(): Promise<string | null> {
  const subscription = await currentSubscription();
  if (!subscription) return null;
  const endpoint = subscription.endpoint;
  await subscription.unsubscribe();
  return endpoint;
}

function sameBytes(a: Uint8Array, b: Uint8Array): boolean {
  return a.length === b.length && a.every((value, i) => value === b[i]);
}

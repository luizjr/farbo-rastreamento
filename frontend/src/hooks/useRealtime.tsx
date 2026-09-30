import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import type { ReactNode } from 'react';

import { websocketUrl } from '@/api/client';
import { parseRealtimeMessage } from '@/api/realtime';
import { useAuth } from '@/stores/AuthContext';
import type { RealtimeEventType, RealtimeMessage } from '@/types';

type Handler = (message: RealtimeMessage) => void;

interface RealtimeContextValue {
  connected: boolean;
  /** Registra um ouvinte; devolve a função de cancelamento. */
  subscribe: (types: RealtimeEventType[] | 'all', handler: Handler) => () => void;
}

const RealtimeContext = createContext<RealtimeContextValue | null>(null);

/** Backoff de reconexão: cresce até 15 s para não martelar o servidor. */
function backoffDelay(attempt: number): number {
  return Math.min(1000 * 2 ** attempt, 15000);
}

export function RealtimeProvider({ children }: { children: ReactNode }) {
  const { user } = useAuth();
  const [connected, setConnected] = useState(false);

  const socketRef = useRef<WebSocket | null>(null);
  const handlersRef = useRef(new Set<{ types: RealtimeEventType[] | 'all'; handler: Handler }>());
  const attemptRef = useRef(0);
  const reconnectRef = useRef<number | null>(null);
  const closedByUsRef = useRef(false);

  const subscribe = useCallback<RealtimeContextValue['subscribe']>((types, handler) => {
    const entry = { types, handler };
    handlersRef.current.add(entry);
    return () => {
      handlersRef.current.delete(entry);
    };
  }, []);

  useEffect(() => {
    if (!user) {
      // Sem sessão não há o que assinar.
      closedByUsRef.current = true;
      socketRef.current?.close();
      socketRef.current = null;
      setConnected(false);
      return;
    }

    closedByUsRef.current = false;
    let disposed = false;

    const connect = () => {
      if (disposed) return;

      const socket = new WebSocket(websocketUrl());
      socketRef.current = socket;

      socket.onopen = () => {
        attemptRef.current = 0;
        setConnected(true);
      };

      socket.onmessage = (event) => {
        // Evento malformado ou com valor fora da faixa é ignorado: nada que
        // não passou pela conferência chega ao cache nem ao mapa.
        const message = typeof event.data === 'string' ? parseRealtimeMessage(event.data) : null;
        if (!message) return;
        handlersRef.current.forEach(({ types, handler }) => {
          if (types === 'all' || types.includes(message.type)) {
            handler(message);
          }
        });
      };

      socket.onclose = () => {
        setConnected(false);
        socketRef.current = null;
        if (disposed || closedByUsRef.current) return;

        const delay = backoffDelay(attemptRef.current);
        attemptRef.current += 1;
        reconnectRef.current = window.setTimeout(connect, delay);
      };

      socket.onerror = () => {
        // O onclose cuida da reconexão; aqui só encerramos o socket ruim.
        socket.close();
      };
    };

    connect();

    return () => {
      disposed = true;
      closedByUsRef.current = true;
      if (reconnectRef.current) window.clearTimeout(reconnectRef.current);
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [user]);

  const value = useMemo(() => ({ connected, subscribe }), [connected, subscribe]);

  return <RealtimeContext.Provider value={value}>{children}</RealtimeContext.Provider>;
}

export function useRealtime(): RealtimeContextValue {
  const context = useContext(RealtimeContext);
  if (!context) {
    throw new Error('useRealtime precisa estar dentro de RealtimeProvider');
  }
  return context;
}

/** Assina um conjunto de eventos enquanto o componente estiver montado. */
export function useRealtimeEvent(types: RealtimeEventType[] | 'all', handler: Handler): void {
  const { subscribe } = useRealtime();
  const handlerRef = useRef(handler);
  handlerRef.current = handler;

  // A lista de tipos é serializada para não reassinar a cada render quando o
  // chamador passa um array literal.
  const key = types === 'all' ? 'all' : types.join(',');

  useEffect(
    () => subscribe(types === 'all' ? 'all' : (key.split(',') as RealtimeEventType[]), (message) =>
      handlerRef.current(message),
    ),
    [subscribe, key, types],
  );
}

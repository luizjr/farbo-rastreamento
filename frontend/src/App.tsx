import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { BrowserRouter, Navigate, Outlet, Route, Routes } from 'react-router-dom';

import { AppShell } from '@/components/layout/AppShell';
import { CustomerDetailsPage } from '@/pages/admin/CustomerDetailsPage';
import { CustomersPage } from '@/pages/admin/CustomersPage';
import { InstallersPage } from '@/pages/admin/InstallersPage';
import { OrdersPage } from '@/pages/admin/OrdersPage';
import { AlertsPage } from '@/pages/customer/AlertsPage';
import { InvoicesPage } from '@/pages/customer/InvoicesPage';
import { MyVehiclesPage } from '@/pages/customer/MyVehiclesPage';
import { Spinner } from '@/components/ui/Spinner';
import { ToastProvider } from '@/components/ui/Toast';
import { RealtimeProvider } from '@/hooks/useRealtime';
import { DashboardPage } from '@/pages/DashboardPage';
import { DevicesPage } from '@/pages/DevicesPage';
import { DiagnosticsPage } from '@/pages/DiagnosticsPage';
import { EventsPage } from '@/pages/EventsPage';
import { ForgotPasswordPage } from '@/pages/ForgotPasswordPage';
import { GeofencesPage } from '@/pages/GeofencesPage';
import { LandingPage } from '@/pages/LandingPage';
import { LoginPage } from '@/pages/LoginPage';
import { ResetPasswordPage } from '@/pages/ResetPasswordPage';
import { VehicleDetailsPage } from '@/pages/VehicleDetailsPage';
import { AuthProvider, useAuth } from '@/stores/AuthContext';

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // O WebSocket já mantém os dados frescos; o refetch é rede de segurança.
      refetchOnWindowFocus: false,
      retry: 1,
      staleTime: 30_000,
    },
  },
});

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <BrowserRouter>
        <AuthProvider>
          <RealtimeProvider>
            <ToastProvider>
              <Routes>
                <Route path="/" element={<LandingPage />} />
                <Route path="/landing" element={<LandingPage />} />
                <Route path="/login" element={<LoginPage />} />
                <Route path="/esqueci-senha" element={<ForgotPasswordPage />} />
                <Route path="/redefinir-senha" element={<ResetPasswordPage />} />

                <Route element={<RequireAuth />}>
                  <Route element={<AppShell />}>
                    {/* Comuns: a API filtra o que o cliente vê. */}
                    <Route path="dashboard" element={<DashboardPage />} />
                    <Route path="veiculos/:id" element={<VehicleDetailsPage />} />

                    <Route element={<RequireCustomer />}>
                      <Route path="meus-veiculos" element={<MyVehiclesPage />} />
                      <Route path="faturas" element={<InvoicesPage />} />
                      <Route path="alertas" element={<AlertsPage />} />
                    </Route>

                    <Route element={<RequireStaff />}>
                      <Route path="eventos" element={<EventsPage />} />
                      <Route path="cercas" element={<GeofencesPage />} />
                    </Route>

                    <Route element={<RequireOperator />}>
                      <Route path="pedidos" element={<OrdersPage />} />
                    </Route>

                    <Route element={<RequireAdmin />}>
                      <Route path="dispositivos" element={<DevicesPage />} />
                      <Route path="diagnostico" element={<DiagnosticsPage />} />
                      <Route path="clientes" element={<CustomersPage />} />
                      <Route path="clientes/:id" element={<CustomerDetailsPage />} />
                      <Route path="prestadores" element={<InstallersPage />} />
                    </Route>
                  </Route>
                </Route>

                <Route path="*" element={<Navigate to="/" replace />} />
              </Routes>
            </ToastProvider>
          </RealtimeProvider>
        </AuthProvider>
      </BrowserRouter>
    </QueryClientProvider>
  );
}

function RequireAuth() {
  const { user, loading } = useAuth();

  if (loading) return <Spinner label="Carregando" />;
  if (!user) return <Navigate to="/login" replace />;
  return <Outlet />;
}

/**
 * As telas de cadastro e diagnóstico ficam fora do alcance de quem só
 * visualiza. A API repete a verificação: isto aqui é conveniência, não
 * segurança.
 */
function RequireAdmin() {
  const { canManage } = useAuth();
  if (!canManage) return <Navigate to="/dashboard" replace />;
  return <Outlet />;
}

/** Telas operacionais da central (eventos de toda a frota, cercas). */
/** Fila de pedidos: admin e operador. */
function RequireOperator() {
  const { canOperate } = useAuth();
  if (!canOperate) return <Navigate to="/dashboard" replace />;
  return <Outlet />;
}

function RequireStaff() {
  const { isStaff } = useAuth();
  if (!isStaff) return <Navigate to="/dashboard" replace />;
  return <Outlet />;
}

/** Área do cliente: veículos próprios e faturas. */
function RequireCustomer() {
  const { isCustomer } = useAuth();
  if (!isCustomer) return <Navigate to="/dashboard" replace />;
  return <Outlet />;
}

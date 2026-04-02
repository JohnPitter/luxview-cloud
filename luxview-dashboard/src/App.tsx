import { Routes, Route, Navigate } from 'react-router-dom';
import { useEffect } from 'react';
import { useThemeStore } from './stores/theme.store';
import { useAuthStore } from './stores/auth.store';
import { Landing } from './pages/Landing';
import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { NewApp } from './pages/NewApp';
import { AppDetail } from './pages/AppDetail';
import { Admin } from './pages/Admin';
import { Resources } from './pages/Resources';
import { DbExplorer } from './pages/DbExplorer';
import { StorageExplorer } from './pages/StorageExplorer';
import { EmailManager } from './pages/EmailManager';
import { Logs } from './pages/Logs';
import { Settings } from './pages/Settings';
import { Profile } from './pages/Profile';
import { Analytics } from './pages/Analytics';
import { Backups } from './pages/Backups';
import { Status } from './pages/Status';
import { Docs } from './pages/Docs';
import { MainLayout } from './components/layout/MainLayout';

function ProtectedRoute({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated);
  if (!isAuthenticated) return <Navigate to="/" replace />;
  return <>{children}</>;
}

export function App() {
  const theme = useThemeStore((s) => s.theme);

  useEffect(() => {
    document.documentElement.classList.toggle('dark', theme === 'dark');
    document.documentElement.classList.toggle('light', theme === 'light');
  }, [theme]);

  return (
    <Routes>
      <Route path="/" element={<Landing />} />
      <Route path="/status" element={<Status />} />
      <Route path="/docs" element={<Docs />} />
      <Route path="/login" element={<Login />} />
      <Route path="/auth/callback" element={<Login />} />
      <Route
        path="/dashboard"
        element={
          <ProtectedRoute>
            <MainLayout />
          </ProtectedRoute>
        }
      >
        <Route index element={<Dashboard />} />
        <Route path="new" element={<NewApp />} />
        <Route path="apps/:appId" element={<AppDetail />} />
        <Route path="admin" element={<Admin />} />
        <Route path="backups" element={<Backups />} />
        <Route path="resources" element={<Resources />} />
        <Route path="resources/db/:serviceId" element={<DbExplorer />} />
        <Route path="resources/storage/:serviceId" element={<StorageExplorer />} />
        <Route path="resources/email/:serviceId" element={<EmailManager />} />
        <Route path="logs" element={<Logs />} />
        <Route path="analytics" element={<Analytics />} />
        <Route path="settings" element={<Settings />} />
        <Route path="profile" element={<Profile />} />
      </Route>
      <Route path="*" element={<Navigate to="/" replace />} />
    </Routes>
  );
}

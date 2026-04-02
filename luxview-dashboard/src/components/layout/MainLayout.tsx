import { Outlet } from 'react-router-dom';
import { useEffect } from 'react';
import { Toolbar } from './Toolbar';
import { Header } from './Header';
import { Sidebar } from './Sidebar';
import { useAuthStore } from '../../stores/auth.store';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore, type Notification } from '../../stores/notifications.store';
import { X, CheckCircle, AlertCircle, AlertTriangle, Info } from 'lucide-react';

function Toast({ notification, onDismiss }: { notification: Notification; onDismiss: () => void }) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const icons = {
    success: <CheckCircle size={16} className="text-emerald-400" />,
    error: <AlertCircle size={16} className="text-red-400" />,
    warning: <AlertTriangle size={16} className="text-amber-400" />,
    info: <Info size={16} className="text-blue-400" />,
  };

  return (
    <div
      className={`
        flex items-start gap-3 p-4 rounded-xl shadow-lg backdrop-blur-md
        animate-slide-down max-w-sm
        ${isDark ? 'bg-zinc-900/90 border border-zinc-800' : 'bg-white/90 border border-zinc-200'}
      `}
    >
      <span className="flex-shrink-0 mt-0.5">{icons[notification.type]}</span>
      <div className="flex-1 min-w-0">
        <p className={`text-sm font-medium ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {notification.title}
        </p>
        {notification.message && (
          <p className="text-xs text-zinc-500 mt-0.5">{notification.message}</p>
        )}
      </div>
      <button onClick={onDismiss} className="text-zinc-500 hover:text-zinc-300 flex-shrink-0">
        <X size={14} />
      </button>
    </div>
  );
}

export function MainLayout() {
  const initialize = useAuthStore((s) => s.initialize);
  const notifications = useNotificationsStore((s) => s.notifications);
  const markRead = useNotificationsStore((s) => s.markRead);
  const pruneExpired = useNotificationsStore((s) => s.pruneExpired);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  useEffect(() => {
    initialize();
    pruneExpired();
  }, [initialize, pruneExpired]);

  // Only show toasts for recent unread notifications (< 6s old)
  const toasts = notifications.filter(
    (n) => !n.read && Date.now() - n.createdAt < 6000,
  );

  // Auto-mark toast notifications as read after display time
  useEffect(() => {
    if (toasts.length === 0) return;
    const timer = setTimeout(() => {
      toasts.forEach((n) => markRead(n.id));
    }, 5000);
    return () => clearTimeout(timer);
  }, [toasts, markRead]);

  return (
    <div className={`min-h-screen ${isDark ? 'bg-zinc-950' : 'bg-zinc-50'}`}>
      <Toolbar />
      <Header />
      <Sidebar />

      {/* Main Content */}
      <main className="pl-24 pr-8 pt-24 pb-8">
        <Outlet />
      </main>

      {/* Toast Notifications */}
      <div className="fixed top-20 left-1/2 -translate-x-1/2 z-50 flex flex-col items-center gap-2">
        {toasts.map((n) => (
          <Toast key={n.id} notification={n} onDismiss={() => markRead(n.id)} />
        ))}
      </div>
    </div>
  );
}

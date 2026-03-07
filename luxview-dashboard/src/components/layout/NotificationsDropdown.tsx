import { useState, useRef, useEffect } from 'react';
import { Bell, X, CheckCircle, AlertCircle, AlertTriangle, Info, Trash2 } from 'lucide-react';
import { useNotificationsStore, type Notification } from '../../stores/notifications.store';
import { useThemeStore } from '../../stores/theme.store';

const icons: Record<string, React.ReactNode> = {
  success: <CheckCircle size={14} className="text-emerald-400" />,
  error: <AlertCircle size={14} className="text-red-400" />,
  warning: <AlertTriangle size={14} className="text-amber-400" />,
  info: <Info size={14} className="text-blue-400" />,
};

export function NotificationsDropdown() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const notifications = useNotificationsStore((s) => s.notifications);
  const removeNotification = useNotificationsStore((s) => s.remove);
  const clearAll = useNotificationsStore((s) => s.clear);
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClick);
    return () => document.removeEventListener('mousedown', handleClick);
  }, []);

  return (
    <div ref={ref} className="relative">
      <button
        onClick={() => setOpen(!open)}
        className={`
          relative flex items-center justify-center w-10 h-10 rounded-xl
          backdrop-blur-md transition-all duration-200
          ${isDark ? 'bg-zinc-900/60 text-zinc-400 hover:text-white border border-zinc-800/50' : 'bg-white/60 text-zinc-500 hover:text-zinc-900 border border-zinc-200/60'}
        `}
        title="Notifications"
      >
        <Bell size={18} />
        {notifications.length > 0 && (
          <span className="absolute -top-1 -right-1 w-4 h-4 bg-amber-400 text-zinc-950 text-[10px] font-bold rounded-full flex items-center justify-center">
            {notifications.length}
          </span>
        )}
      </button>

      {open && (
        <div
          className={`
            absolute right-0 top-12 w-80 rounded-xl shadow-2xl overflow-hidden z-50
            animate-slide-down
            ${isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'}
          `}
        >
          <div className={`flex items-center justify-between px-4 py-3 border-b ${isDark ? 'border-zinc-800' : 'border-zinc-100'}`}>
            <span className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              Notifications
            </span>
            {notifications.length > 0 && (
              <button
                onClick={clearAll}
                className="flex items-center gap-1 text-[11px] text-zinc-500 hover:text-red-400 transition-colors"
              >
                <Trash2 size={12} />
                Clear all
              </button>
            )}
          </div>

          <div className="max-h-72 overflow-y-auto">
            {notifications.length === 0 ? (
              <div className="py-10 text-center">
                <Bell size={24} className="mx-auto mb-2 text-zinc-600" />
                <p className="text-sm text-zinc-500">No notifications</p>
              </div>
            ) : (
              <div className={`divide-y ${isDark ? 'divide-zinc-800/50' : 'divide-zinc-100'}`}>
                {notifications.map((n) => (
                  <div
                    key={n.id}
                    className={`
                      flex items-start gap-2.5 px-4 py-3 transition-colors
                      ${isDark ? 'hover:bg-zinc-800/30' : 'hover:bg-zinc-50'}
                    `}
                  >
                    <span className="flex-shrink-0 mt-0.5">{icons[n.type]}</span>
                    <div className="flex-1 min-w-0">
                      <p className={`text-xs font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                        {n.title}
                      </p>
                      {n.message && (
                        <p className="text-[11px] text-zinc-500 mt-0.5 truncate">{n.message}</p>
                      )}
                    </div>
                    <button
                      onClick={() => removeNotification(n.id)}
                      className="flex-shrink-0 text-zinc-600 hover:text-zinc-300 transition-colors"
                    >
                      <X size={12} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

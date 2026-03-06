import { LogOut, Bell } from 'lucide-react';
import { useAuthStore } from '../../stores/auth.store';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';

export function Header() {
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const notifications = useNotificationsStore((s) => s.notifications);

  return (
    <header
      className={`
        fixed top-6 right-8 z-40
        flex items-center gap-3
      `}
    >
      {/* Notifications */}
      <button
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

      {/* User */}
      {user && (
        <div
          className={`
            flex items-center gap-3 h-10 pl-1 pr-4 rounded-xl
            backdrop-blur-md transition-all duration-200
            ${isDark ? 'bg-zinc-900/60 border border-zinc-800/50' : 'bg-white/60 border border-zinc-200/60'}
          `}
        >
          <img
            src={user.avatarUrl || `https://github.com/${user.username}.png`}
            alt={user.username}
            className="w-8 h-8 rounded-lg object-cover"
          />
          <span
            className={`text-sm font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}
          >
            {user.username}
          </span>
          <button
            onClick={logout}
            className={`
              ml-1 p-1 rounded-lg transition-all duration-200
              ${isDark ? 'text-zinc-500 hover:text-red-400 hover:bg-zinc-800/50' : 'text-zinc-400 hover:text-red-500 hover:bg-zinc-100'}
            `}
            title="Logout"
          >
            <LogOut size={16} />
          </button>
        </div>
      )}
    </header>
  );
}

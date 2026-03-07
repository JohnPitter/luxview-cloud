import { LogOut } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useAuthStore } from '../../stores/auth.store';
import { useThemeStore } from '../../stores/theme.store';
import { NotificationsDropdown } from './NotificationsDropdown';

export function Header() {
  const navigate = useNavigate();
  const user = useAuthStore((s) => s.user);
  const logout = useAuthStore((s) => s.logout);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  return (
    <header
      className={`
        fixed top-6 right-8 z-40
        flex items-center gap-3
      `}
    >
      {/* Notifications */}
      <NotificationsDropdown />

      {/* User */}
      {user && (
        <div
          className={`
            flex items-center gap-3 h-10 pl-1 pr-4 rounded-xl
            backdrop-blur-md transition-all duration-200
            ${isDark ? 'bg-zinc-900/60 border border-zinc-800/50' : 'bg-white/60 border border-zinc-200/60'}
          `}
        >
          <button
            onClick={() => navigate('/dashboard/profile')}
            className="flex items-center gap-3 hover:opacity-80 transition-opacity"
            title="View Profile"
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
          </button>
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

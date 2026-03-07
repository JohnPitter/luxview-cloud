import { Settings as SettingsIcon, Moon, Sun, Bell, Monitor } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { useThemeStore } from '../stores/theme.store';
import { useAuthStore } from '../stores/auth.store';

export function Settings() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { theme, toggleTheme } = useThemeStore();
  const user = useAuthStore((s) => s.user);

  const themeOptions = [
    { value: 'dark', label: 'Dark', icon: Moon },
    { value: 'light', label: 'Light', icon: Sun },
  ] as const;

  return (
    <div className="animate-fade-in max-w-2xl">
      <div className="flex items-center gap-3 mb-8">
        <SettingsIcon size={24} className="text-amber-400" />
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            Settings
          </h1>
          <p className="text-sm text-zinc-500">Manage your preferences</p>
        </div>
      </div>

      {/* Appearance */}
      <GlassCard className="mb-4">
        <div className="flex items-center gap-3 mb-4">
          <Monitor size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            Appearance
          </h3>
        </div>
        <div className="flex gap-3">
          {themeOptions.map((opt) => {
            const Icon = opt.icon;
            const isActive = theme === opt.value;
            return (
              <button
                key={opt.value}
                onClick={isActive ? undefined : toggleTheme}
                className={`
                  flex items-center gap-2 px-4 py-2.5 rounded-xl text-sm font-medium
                  transition-all duration-200
                  ${
                    isActive
                      ? 'bg-amber-400/10 text-amber-400 ring-1 ring-amber-400/30'
                      : isDark
                        ? 'bg-zinc-800/50 text-zinc-400 hover:bg-zinc-800 hover:text-zinc-200'
                        : 'bg-zinc-100 text-zinc-500 hover:bg-zinc-200 hover:text-zinc-700'
                  }
                `}
              >
                <Icon size={16} />
                {opt.label}
              </button>
            );
          })}
        </div>
      </GlassCard>

      {/* Account Info */}
      <GlassCard className="mb-4">
        <div className="flex items-center gap-3 mb-4">
          <Bell size={18} className="text-zinc-400" />
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            Account
          </h3>
        </div>
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">Username</span>
            <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {user?.username || '-'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">Email</span>
            <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {user?.email || '-'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">Role</span>
            <span
              className={`
                text-xs font-medium px-2 py-0.5 rounded-full
                ${user?.role === 'admin' ? 'bg-amber-400/10 text-amber-400' : 'bg-zinc-400/10 text-zinc-400'}
              `}
            >
              {user?.role || 'user'}
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-sm text-zinc-500">Member since</span>
            <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {user?.createdAt
                ? new Date(user.createdAt).toLocaleDateString('en-US', {
                    year: 'numeric',
                    month: 'long',
                    day: 'numeric',
                  })
                : '-'}
            </span>
          </div>
        </div>
      </GlassCard>

      {/* GitHub Connection */}
      <GlassCard>
        <div className="flex items-center gap-3 mb-4">
          <svg viewBox="0 0 24 24" width={18} height={18} className="text-zinc-400 fill-current">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
          </svg>
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            GitHub Connection
          </h3>
        </div>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            {user?.avatarUrl && (
              <img src={user.avatarUrl} alt="" className="w-8 h-8 rounded-lg" />
            )}
            <div>
              <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {user?.username}
              </p>
              <p className="text-[11px] text-emerald-400">Connected</p>
            </div>
          </div>
        </div>
      </GlassCard>
    </div>
  );
}

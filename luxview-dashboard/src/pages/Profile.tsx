import { User, Github, Calendar, Shield, ExternalLink } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { useThemeStore } from '../stores/theme.store';
import { useAuthStore } from '../stores/auth.store';
import { useAppsStore } from '../stores/apps.store';
import { useEffect } from 'react';

export function Profile() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const user = useAuthStore((s) => s.user);
  const { apps, fetchApps } = useAppsStore();

  useEffect(() => {
    fetchApps();
  }, [fetchApps]);

  const runningApps = apps.filter((a) => a.status === 'running').length;

  return (
    <div className="animate-fade-in max-w-2xl">
      <div className="flex items-center gap-3 mb-8">
        <User size={24} className="text-amber-400" />
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            Profile
          </h1>
          <p className="text-sm text-zinc-500">Your account information</p>
        </div>
      </div>

      {/* Profile Card */}
      <GlassCard className="mb-4">
        <div className="flex items-center gap-4">
          <img
            src={user?.avatarUrl || `https://github.com/${user?.username}.png`}
            alt={user?.username}
            className="w-16 h-16 rounded-2xl ring-2 ring-amber-400/20"
          />
          <div className="flex-1">
            <h2
              className={`text-lg font-bold tracking-tight ${
                isDark ? 'text-zinc-100' : 'text-zinc-900'
              }`}
            >
              {user?.username}
            </h2>
            <p className="text-sm text-zinc-500">{user?.email}</p>
            <div className="flex items-center gap-3 mt-2">
              <span
                className={`
                  inline-flex items-center gap-1 text-xs font-medium px-2 py-0.5 rounded-full
                  ${user?.role === 'admin' ? 'bg-amber-400/10 text-amber-400' : 'bg-zinc-400/10 text-zinc-400'}
                `}
              >
                <Shield size={10} />
                {user?.role}
              </span>
              {user?.createdAt && (
                <span className="inline-flex items-center gap-1 text-[11px] text-zinc-500">
                  <Calendar size={10} />
                  Joined{' '}
                  {new Date(user.createdAt).toLocaleDateString('en-US', {
                    month: 'short',
                    year: 'numeric',
                  })}
                </span>
              )}
            </div>
          </div>
        </div>
      </GlassCard>

      {/* Stats */}
      <div className="grid grid-cols-3 gap-4 mb-4">
        <GlassCard>
          <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1">Total Apps</p>
          <p className={`text-2xl font-bold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {apps.length}
          </p>
        </GlassCard>
        <GlassCard>
          <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1">Running</p>
          <p className="text-2xl font-bold text-emerald-400">{runningApps}</p>
        </GlassCard>
        <GlassCard>
          <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-1">Role</p>
          <p className="text-2xl font-bold text-amber-400 capitalize">{user?.role}</p>
        </GlassCard>
      </div>

      {/* GitHub */}
      <GlassCard>
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-3">
            <Github size={20} className="text-zinc-400" />
            <div>
              <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                GitHub Account
              </p>
              <p className="text-[11px] text-zinc-500">
                github.com/{user?.username}
              </p>
            </div>
          </div>
          <a
            href={`https://github.com/${user?.username}`}
            target="_blank"
            rel="noopener noreferrer"
            className={`
              flex items-center gap-1 text-xs font-medium px-3 py-1.5 rounded-lg
              transition-all duration-200
              ${isDark ? 'text-zinc-400 hover:text-white hover:bg-zinc-800' : 'text-zinc-500 hover:text-zinc-900 hover:bg-zinc-100'}
            `}
          >
            View Profile
            <ExternalLink size={12} />
          </a>
        </div>
      </GlassCard>
    </div>
  );
}

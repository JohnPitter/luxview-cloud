import { useNavigate } from 'react-router-dom';
import { ExternalLink, GitBranch, Clock } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { StatusDot } from '../common/StatusDot';
import { AppStatusBadge } from './AppStatusBadge';
import { useThemeStore } from '../../stores/theme.store';
import { formatRelativeTime, formatPercent, formatBytes } from '../../lib/format';
import type { App } from '../../api/apps';

interface AppCardProps {
  app: App;
}

const stackIcons: Record<string, string> = {
  node: 'JS',
  python: 'PY',
  go: 'GO',
  rust: 'RS',
  static: 'WEB',
  docker: 'DKR',
};

const stackColors: Record<string, string> = {
  node: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  python: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  go: 'bg-cyan-500/10 text-cyan-400 border-cyan-500/20',
  rust: 'bg-orange-500/10 text-orange-400 border-orange-500/20',
  static: 'bg-violet-500/10 text-violet-400 border-violet-500/20',
  docker: 'bg-sky-500/10 text-sky-400 border-sky-500/20',
};

export function AppCard({ app }: AppCardProps) {
  const navigate = useNavigate();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const cpuPercent = app.cpuPercent ?? 0;
  const memoryMB = app.memoryBytes ? app.memoryBytes / (1024 * 1024) : 0;
  const memoryLimit = parseInt(app.resourceLimits?.memory || '512') || 512;
  const memoryPercent = (memoryMB / memoryLimit) * 100;

  return (
    <GlassCard
      hover
      onClick={() => navigate(`/dashboard/apps/${app.id}`)}
      className="group relative overflow-hidden"
    >
      {/* Top row */}
      <div className="flex items-start justify-between mb-4">
        <div className="flex items-center gap-3">
          <StatusDot status={app.status} size="lg" />
          <div>
            <h3
              className={`text-base font-semibold tracking-tight ${
                isDark ? 'text-zinc-100' : 'text-zinc-900'
              }`}
            >
              {app.name}
            </h3>
            <a
              href={`https://${app.subdomain}.luxview.cloud`}
              onClick={(e) => e.stopPropagation()}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center gap-1 text-xs text-zinc-500 hover:text-amber-400 transition-colors"
            >
              {app.subdomain}.luxview.cloud
              <ExternalLink size={10} />
            </a>
          </div>
        </div>

        {/* Stack badge */}
        <span
          className={`
            text-[10px] font-bold px-2 py-0.5 rounded-md border
            ${stackColors[app.stack] || 'bg-zinc-800 text-zinc-400 border-zinc-700'}
          `}
        >
          {stackIcons[app.stack] || app.stack.toUpperCase()}
        </span>
      </div>

      {/* Status badge */}
      <div className="mb-4">
        <AppStatusBadge status={app.status} />
      </div>

      {/* Resource bars */}
      <div className="space-y-2 mb-4">
        <div>
          <div className="flex items-center justify-between text-[11px] mb-1">
            <span className="text-zinc-500">CPU</span>
            <span className={isDark ? 'text-zinc-400' : 'text-zinc-600'}>
              {formatPercent(cpuPercent)}
            </span>
          </div>
          <div className={`h-1.5 rounded-full overflow-hidden ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                cpuPercent > 80 ? 'bg-red-500' : cpuPercent > 50 ? 'bg-amber-400' : 'bg-emerald-400'
              }`}
              style={{ width: `${Math.min(cpuPercent, 100)}%` }}
            />
          </div>
        </div>
        <div>
          <div className="flex items-center justify-between text-[11px] mb-1">
            <span className="text-zinc-500">RAM</span>
            <span className={isDark ? 'text-zinc-400' : 'text-zinc-600'}>
              {formatBytes(app.memoryBytes ?? 0)}
            </span>
          </div>
          <div className={`h-1.5 rounded-full overflow-hidden ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
            <div
              className={`h-full rounded-full transition-all duration-500 ${
                memoryPercent > 80 ? 'bg-red-500' : memoryPercent > 50 ? 'bg-amber-400' : 'bg-emerald-400'
              }`}
              style={{ width: `${Math.min(memoryPercent, 100)}%` }}
            />
          </div>
        </div>
      </div>

      {/* Footer */}
      <div className="flex items-center justify-between text-[11px] text-zinc-500">
        <div className="flex items-center gap-1">
          <GitBranch size={12} />
          <span>{app.repoBranch}</span>
        </div>
        {app.lastDeployAt && (
          <div className="flex items-center gap-1">
            <Clock size={12} />
            <span>{formatRelativeTime(app.lastDeployAt)}</span>
          </div>
        )}
      </div>

      {/* Hover glow */}
      <div
        className="absolute inset-0 rounded-2xl opacity-0 group-hover:opacity-100 transition-opacity duration-300 pointer-events-none"
        style={{
          background:
            'radial-gradient(600px circle at var(--mouse-x, 50%) var(--mouse-y, 50%), rgba(251, 191, 36, 0.03), transparent 40%)',
        }}
      />
    </GlassCard>
  );
}

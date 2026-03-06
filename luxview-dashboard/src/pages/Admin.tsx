import { useState, useEffect } from 'react';
import {
  Users,
  Server,
  Cpu,
  HardDrive,
  Activity,
  Shield,
  RefreshCw,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { StatusDot } from '../components/common/StatusDot';
import { AppStatusBadge } from '../components/apps/AppStatusBadge';
import { useAppsStore } from '../stores/apps.store';
import { useThemeStore } from '../stores/theme.store';
import { useAuthStore } from '../stores/auth.store';
import { formatBytes, formatRelativeTime } from '../lib/format';

interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  sub?: string;
  color: string;
}

function StatCard({ icon, label, value, sub, color }: StatCardProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  return (
    <GlassCard>
      <div className="flex items-center gap-3 mb-3">
        <div className={`flex items-center justify-center w-10 h-10 rounded-xl ${color}`}>
          {icon}
        </div>
        <span className="text-xs font-medium text-zinc-500 uppercase tracking-wider">
          {label}
        </span>
      </div>
      <p className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
        {value}
      </p>
      {sub && <p className="text-[11px] text-zinc-500 mt-1">{sub}</p>}
    </GlassCard>
  );
}

export function Admin() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const user = useAuthStore((s) => s.user);
  const { apps, fetchApps } = useAppsStore();

  useEffect(() => {
    fetchApps();
  }, [fetchApps]);

  const runningApps = apps.filter((a) => a.status === 'running');
  const errorApps = apps.filter((a) => a.status === 'error');

  return (
    <div className="animate-fade-in">
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <Shield size={24} className="text-amber-400" />
          <div>
            <h1
              className={`text-2xl font-bold tracking-tight ${
                isDark ? 'text-zinc-100' : 'text-zinc-900'
              }`}
            >
              Admin Panel
            </h1>
            <p className="text-sm text-zinc-500">Platform overview and management</p>
          </div>
        </div>
        <PillButton
          variant="secondary"
          size="sm"
          onClick={() => fetchApps()}
          icon={<RefreshCw size={14} />}
        >
          Refresh
        </PillButton>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <StatCard
          icon={<Users size={20} className="text-blue-400" />}
          label="Users"
          value={1}
          sub="Active users"
          color="bg-blue-500/10"
        />
        <StatCard
          icon={<Server size={20} className="text-emerald-400" />}
          label="Apps"
          value={apps.length}
          sub={`${runningApps.length} running`}
          color="bg-emerald-500/10"
        />
        <StatCard
          icon={<Cpu size={20} className="text-amber-400" />}
          label="CPU Usage"
          value="12%"
          sub="Platform total"
          color="bg-amber-500/10"
        />
        <StatCard
          icon={<HardDrive size={20} className="text-violet-400" />}
          label="Memory"
          value="2.1 GB"
          sub="of 8 GB allocated"
          color="bg-violet-500/10"
        />
      </div>

      {/* Apps Table */}
      <GlassCard padding="none">
        <div className="px-6 py-4 border-b border-zinc-800/50">
          <h3
            className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
          >
            All Applications ({apps.length})
          </h3>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr className={isDark ? 'text-zinc-500' : 'text-zinc-400'}>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  App
                </th>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  Status
                </th>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  Stack
                </th>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  CPU
                </th>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  Memory
                </th>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  Created
                </th>
                <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">
                  Actions
                </th>
              </tr>
            </thead>
            <tbody>
              {apps.length === 0 ? (
                <tr>
                  <td colSpan={7} className="text-center py-12 text-sm text-zinc-500">
                    No applications deployed
                  </td>
                </tr>
              ) : (
                apps.map((app) => (
                  <tr
                    key={app.id}
                    className={`
                      border-t transition-colors
                      ${isDark ? 'border-zinc-800/50 hover:bg-zinc-800/20' : 'border-zinc-100 hover:bg-zinc-50'}
                    `}
                  >
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-2">
                        <StatusDot status={app.status} size="sm" />
                        <div>
                          <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                            {app.name}
                          </p>
                          <p className="text-[11px] text-zinc-500">
                            {app.subdomain}.luxview.cloud
                          </p>
                        </div>
                      </div>
                    </td>
                    <td className="px-6 py-3">
                      <AppStatusBadge status={app.status} />
                    </td>
                    <td className="px-6 py-3 text-xs text-zinc-400 uppercase">{app.stack}</td>
                    <td className="px-6 py-3 text-xs text-zinc-400">
                      {app.cpuPercent?.toFixed(1) ?? '0.0'}%
                    </td>
                    <td className="px-6 py-3 text-xs text-zinc-400">
                      {formatBytes(app.memoryBytes ?? 0)}
                    </td>
                    <td className="px-6 py-3 text-xs text-zinc-500">
                      {formatRelativeTime(app.createdAt)}
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex items-center gap-1">
                        <PillButton variant="ghost" size="sm">
                          Manage
                        </PillButton>
                      </div>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </GlassCard>

      {/* Error Apps */}
      {errorApps.length > 0 && (
        <GlassCard className="mt-6 !border-red-500/20">
          <div className="flex items-center gap-2 mb-3">
            <Activity size={16} className="text-red-400" />
            <h3 className="text-sm font-semibold text-red-400">
              Apps with Errors ({errorApps.length})
            </h3>
          </div>
          <div className="space-y-2">
            {errorApps.map((app) => (
              <div
                key={app.id}
                className="flex items-center justify-between py-2 border-b border-zinc-800/30 last:border-0"
              >
                <span className={`text-sm ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                  {app.name}
                </span>
                <PillButton variant="danger" size="sm">
                  Investigate
                </PillButton>
              </div>
            ))}
          </div>
        </GlassCard>
      )}
    </div>
  );
}

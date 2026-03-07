import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { Plus, Rocket, Server } from 'lucide-react';
import { AppCard } from '../components/apps/AppCard';
import { EmptyState } from '../components/common/EmptyState';
import { PillButton } from '../components/common/PillButton';
import { useAppsStore } from '../stores/apps.store';
import { useThemeStore } from '../stores/theme.store';
import { metricsApi, type LatestMetric } from '../api/metrics';

export function Dashboard() {
  const navigate = useNavigate();
  const { apps, loading, fetchApps } = useAppsStore();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const [latestMetrics, setLatestMetrics] = useState<Record<string, LatestMetric>>({});

  useEffect(() => {
    fetchApps();
  }, [fetchApps]);

  useEffect(() => {
    if (apps.length === 0) return;
    metricsApi.getLatestAll().then(setLatestMetrics).catch(() => {});
    const interval = setInterval(() => {
      metricsApi.getLatestAll().then(setLatestMetrics).catch(() => {});
    }, 30000);
    return () => clearInterval(interval);
  }, [apps.length]);

  const runningCount = apps.filter((a) => a.status === 'running').length;
  const totalCount = apps.length;

  return (
    <div className="animate-fade-in">
      {/* Page Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            Your Apps
          </h1>
          <p className="text-sm text-zinc-500 mt-1">
            {totalCount > 0
              ? `${runningCount} running of ${totalCount} total`
              : 'Deploy your first application'}
          </p>
        </div>
        <PillButton
          variant="primary"
          size="md"
          onClick={() => navigate('/dashboard/new')}
          icon={<Plus size={16} />}
        >
          New App
        </PillButton>
      </div>

      {/* Loading skeleton */}
      {loading && apps.length === 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {Array.from({ length: 6 }).map((_, i) => (
            <div
              key={i}
              className={`h-52 rounded-2xl animate-pulse ${
                isDark ? 'bg-zinc-900/50' : 'bg-zinc-100'
              }`}
            />
          ))}
        </div>
      )}

      {/* Empty state */}
      {!loading && apps.length === 0 && (
        <EmptyState
          icon={<Server size={28} />}
          title="No apps yet"
          description="Deploy your first application from GitHub in just a few clicks. We auto-detect your stack and handle the rest."
          action={
            <PillButton
              variant="primary"
              size="md"
              onClick={() => navigate('/dashboard/new')}
              icon={<Rocket size={16} />}
            >
              Deploy Your First App
            </PillButton>
          }
        />
      )}

      {/* App Grid */}
      {apps.length > 0 && (
        <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
          {apps.map((app) => (
            <AppCard key={app.id} app={app} metrics={latestMetrics[app.id]} />
          ))}
        </div>
      )}
    </div>
  );
}

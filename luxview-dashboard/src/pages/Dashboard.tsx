import { useEffect, useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Plus,
  Rocket,
  Server,
  Activity,
  Database,
  GitBranch,
  ArrowUpRight,
  Zap,
} from 'lucide-react';
import { AppCard } from '../components/apps/AppCard';
import { EmptyState } from '../components/common/EmptyState';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { StatusDot } from '../components/common/StatusDot';
import { PageTour, isOnboardingComplete } from '../components/common/PageTour';
import { useAppsStore } from '../stores/apps.store';
import { useAuthStore } from '../stores/auth.store';
import { useThemeStore } from '../stores/theme.store';
import { metricsApi, type LatestMetric } from '../api/metrics';
import { servicesApi } from '../api/services';
import { deploymentsApi } from '../api/deployments';
import { formatRelativeTime } from '../lib/format';
import { dashboardTourSteps } from '../tours/dashboard';

export function Dashboard() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const { apps, loading, fetchApps } = useAppsStore();
  const user = useAuthStore((s) => s.user);
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const [latestMetrics, setLatestMetrics] = useState<Record<string, LatestMetric>>({});
  const [resourceCount, setResourceCount] = useState(0);
  const [recentDeploys, setRecentDeploys] = useState<
    Array<{ id: string; appName: string; appId: string; status: string; createdAt: string }>
  >([]);

  useEffect(() => {
    fetchApps();
    servicesApi
      .listAll()
      .then((s) => setResourceCount(s.length))
      .catch(() => {});
  }, [fetchApps]);

  // Fetch recent deploys across all apps
  useEffect(() => {
    if (apps.length === 0) return;
    const fetchDeploys = async () => {
      const allDeploys: typeof recentDeploys = [];
      for (const app of apps.slice(0, 10)) {
        try {
          const deploys = await deploymentsApi.list(app.id);
          for (const d of deploys.slice(0, 3)) {
            allDeploys.push({
              id: d.id,
              appName: app.name,
              appId: app.id,
              status: d.status,
              createdAt: d.createdAt,
            });
          }
        } catch {
          // ignore
        }
      }
      allDeploys.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime());
      setRecentDeploys(allDeploys.slice(0, 5));
    };
    fetchDeploys();
  }, [apps]);

  const hasTransitional = apps.some((a) => ['building', 'deploying'].includes(a.status));

  useEffect(() => {
    if (apps.length === 0) return;
    const poll = () => {
      fetchApps();
      metricsApi.getLatestAll().then(setLatestMetrics).catch(() => {});
    };
    const intervalMs = hasTransitional ? 5000 : 30000;
    metricsApi.getLatestAll().then(setLatestMetrics).catch(() => {});
    const interval = setInterval(poll, intervalMs);
    return () => clearInterval(interval);
  }, [apps.length, hasTransitional, fetchApps]);

  const runningCount = apps.filter((a) => a.status === 'running').length;
  const totalCount = apps.length;
  const firstName = user?.username || 'Developer';

  const deployStatusColor: Record<string, string> = {
    success: 'text-emerald-400',
    failed: 'text-red-400',
    building: 'text-amber-400',
    deploying: 'text-blue-400',
    pending: 'text-zinc-400',
  };

  return (
    <div className="animate-fade-in">
      <PageTour tourId="dashboard" steps={dashboardTourSteps} autoStart={!isOnboardingComplete()} />

      {/* Hero Header */}
      <div className="mb-8">
        <h1
          className={`text-3xl font-bold tracking-tight ${
            isDark ? 'text-zinc-100' : 'text-zinc-900'
          }`}
        >
          {t('dashboard.welcomeBack', { name: firstName })}
        </h1>
        <p className="text-sm text-zinc-500 mt-1">
          {t('dashboard.subtitle')}
        </p>
      </div>

      {/* Stats Grid */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8" data-tour="stats-grid">
        {[
          {
            label: t('dashboard.stats.totalApps'),
            value: totalCount,
            icon: Server,
            color: 'text-amber-400',
            bg: 'bg-amber-500/10',
          },
          {
            label: t('dashboard.stats.running'),
            value: runningCount,
            icon: Activity,
            color: 'text-emerald-400',
            bg: 'bg-emerald-500/10',
          },
          {
            label: t('dashboard.stats.resources'),
            value: resourceCount,
            icon: Database,
            color: 'text-blue-400',
            bg: 'bg-blue-500/10',
          },
          {
            label: t('dashboard.stats.deploys'),
            value: recentDeploys.length > 0 ? recentDeploys.length + '+' : '0',
            icon: Rocket,
            color: 'text-violet-400',
            bg: 'bg-violet-500/10',
          },
        ].map((stat) => (
          <GlassCard key={stat.label} className="!p-4 group hover:scale-[1.02] transition-transform duration-200">
            <div className="flex items-center gap-3">
              <div
                className={`w-10 h-10 rounded-xl flex items-center justify-center ${stat.bg}`}
              >
                <stat.icon size={18} className={stat.color} />
              </div>
              <div>
                <p
                  className={`text-2xl font-bold tabular-nums ${
                    isDark ? 'text-zinc-100' : 'text-zinc-900'
                  }`}
                >
                  {loading ? '-' : stat.value}
                </p>
                <p className="text-[11px] text-zinc-500 font-medium">{stat.label}</p>
              </div>
            </div>
          </GlassCard>
        ))}
      </div>

      {/* Quick Actions */}
      <div className="flex items-center gap-3 mb-6" data-tour="new-app-btn">
        <PillButton
          variant="primary"
          size="md"
          onClick={() => navigate('/dashboard/new')}
          icon={<Plus size={16} />}
        >
          {t('common.newApp')}
        </PillButton>
        <PillButton
          variant="ghost"
          size="md"
          onClick={() => navigate('/dashboard/resources')}
          icon={<Database size={16} />}
        >
          {t('common.resources')}
        </PillButton>
      </div>

      {/* Main Content */}
      <div className="grid grid-cols-1 xl:grid-cols-3 gap-6">
        {/* Apps Column - 2/3 */}
        <div className="xl:col-span-2" data-tour="apps-list">
          <div className="flex items-center justify-between mb-4">
            <h2
              className={`text-lg font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
            >
              {t('dashboard.yourApps')}
            </h2>
            {totalCount > 0 && (
              <span className="text-xs text-zinc-500">
                {t('dashboard.runningCount', { running: runningCount, total: totalCount })}
              </span>
            )}
          </div>

          {/* Loading skeleton */}
          {loading && apps.length === 0 && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {Array.from({ length: 4 }).map((_, i) => (
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
              title={t('dashboard.emptyState.title')}
              description={t('dashboard.emptyState.description')}
              action={
                <PillButton
                  variant="primary"
                  size="md"
                  onClick={() => navigate('/dashboard/new')}
                  icon={<Rocket size={16} />}
                >
                  {t('dashboard.emptyState.cta')}
                </PillButton>
              }
            />
          )}

          {/* App Grid */}
          {apps.length > 0 && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {apps.map((app) => (
                <AppCard key={app.id} app={app} metrics={latestMetrics[app.id]} />
              ))}
            </div>
          )}
        </div>

        {/* Sidebar - 1/3 */}
        <div className="space-y-6">
          {/* Recent Deploys */}
          <GlassCard data-tour="recent-deploys">
            <div className="flex items-center justify-between mb-4">
              <h3
                className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                {t('dashboard.recentDeploys')}
              </h3>
              <Zap size={14} className="text-zinc-500" />
            </div>
            {recentDeploys.length === 0 ? (
              <p className="text-xs text-zinc-500 py-4 text-center">{t('dashboard.noDeploysYet')}</p>
            ) : (
              <div className="space-y-3">
                {recentDeploys.map((d) => (
                  <button
                    key={d.id}
                    onClick={() => navigate(`/dashboard/apps/${d.appId}`)}
                    className={`
                      w-full flex items-center gap-3 p-2 -mx-2 rounded-lg text-left
                      transition-all duration-150
                      ${isDark ? 'hover:bg-zinc-800/50' : 'hover:bg-zinc-50'}
                    `}
                  >
                    <StatusDot
                      status={
                        d.status === 'success'
                          ? 'running'
                          : d.status === 'failed'
                            ? 'error'
                            : 'building'
                      }
                      size="sm"
                    />
                    <div className="flex-1 min-w-0">
                      <p
                        className={`text-xs font-medium truncate ${
                          isDark ? 'text-zinc-200' : 'text-zinc-800'
                        }`}
                      >
                        {d.appName}
                      </p>
                      <p className="text-[10px] text-zinc-500">
                        {formatRelativeTime(d.createdAt)}
                      </p>
                    </div>
                    <span
                      className={`text-[10px] font-medium ${
                        deployStatusColor[d.status] || 'text-zinc-500'
                      }`}
                    >
                      {d.status}
                    </span>
                  </button>
                ))}
              </div>
            )}
          </GlassCard>

          {/* Quick Links */}
          <GlassCard>
            <h3
              className={`text-sm font-semibold mb-3 ${
                isDark ? 'text-zinc-200' : 'text-zinc-800'
              }`}
            >
              {t('dashboard.quickLinks')}
            </h3>
            <div className="space-y-1">
              {[
                { label: t('dashboard.quickLinks.deployNewApp'), path: '/dashboard/new', icon: Rocket },
                { label: t('dashboard.quickLinks.viewResources'), path: '/dashboard/resources', icon: Database },
                { label: t('dashboard.quickLinks.monitoring'), path: '/dashboard/admin', icon: Activity },
                { label: t('dashboard.quickLinks.buildLogs'), path: '/dashboard/logs', icon: GitBranch },
              ].map((link) => (
                <button
                  key={link.path}
                  onClick={() => navigate(link.path)}
                  className={`
                    w-full flex items-center gap-2 px-2 py-2 rounded-lg text-left text-xs
                    transition-all duration-150
                    ${
                      isDark
                        ? 'text-zinc-400 hover:text-zinc-200 hover:bg-zinc-800/50'
                        : 'text-zinc-500 hover:text-zinc-800 hover:bg-zinc-50'
                    }
                  `}
                >
                  <link.icon size={14} />
                  <span className="flex-1">{link.label}</span>
                  <ArrowUpRight size={12} className="opacity-0 group-hover:opacity-100" />
                </button>
              ))}
            </div>
          </GlassCard>
        </div>
      </div>
    </div>
  );
}

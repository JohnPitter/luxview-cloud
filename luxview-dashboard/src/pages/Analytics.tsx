import { useEffect, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Users,
  Eye,
  Timer,
  TrendingUp,
  TrendingDown,
  Globe,
  Monitor,
  Smartphone,
  Tablet,
  Chrome,
  ExternalLink,
  Activity,
} from 'lucide-react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  PieChart,
  Pie,
  Cell,
} from 'recharts';
import { GlassCard } from '../components/common/GlassCard';
import { useAppsStore } from '../stores/apps.store';
import { useAuthStore } from '../stores/auth.store';
import { useThemeStore } from '../stores/theme.store';
import { analyticsApi, type OverviewData, type RankedItem } from '../api/analytics';
import { formatPercent } from '../lib/format';

type DateRange = '24h' | '7d' | '30d' | '90d';

const RANGE_MS: Record<DateRange, number> = {
  '24h': 24 * 60 * 60 * 1000,
  '7d': 7 * 24 * 60 * 60 * 1000,
  '30d': 30 * 24 * 60 * 60 * 1000,
  '90d': 90 * 24 * 60 * 60 * 1000,
};

const PIE_COLORS = ['#f59e0b', '#3b82f6', '#10b981', '#8b5cf6', '#ef4444', '#6366f1', '#ec4899', '#14b8a6'];

const COUNTRY_FLAGS: Record<string, string> = {
  BR: '🇧🇷', US: '🇺🇸', DE: '🇩🇪', GB: '🇬🇧', FR: '🇫🇷', JP: '🇯🇵', CN: '🇨🇳', IN: '🇮🇳',
  CA: '🇨🇦', AU: '🇦🇺', ES: '🇪🇸', IT: '🇮🇹', PT: '🇵🇹', MX: '🇲🇽', AR: '🇦🇷', KR: '🇰🇷',
  NL: '🇳🇱', SE: '🇸🇪', NO: '🇳🇴', DK: '🇩🇰', FI: '🇫🇮', PL: '🇵🇱', RU: '🇷🇺', CL: '🇨🇱',
  CO: '🇨🇴', PE: '🇵🇪', UY: '🇺🇾', unknown: '🌐',
};

const DEVICE_ICONS: Record<string, typeof Monitor> = {
  desktop: Monitor,
  mobile: Smartphone,
  tablet: Tablet,
};

export function Analytics() {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const user = useAuthStore((s) => s.user);
  const { apps, fetchApps } = useAppsStore();
  const isAdmin = user?.role === 'admin';

  const [selectedAppId, setSelectedAppId] = useState<string>('');
  const [range, setRange] = useState<DateRange>('7d');
  const [granularity, setGranularity] = useState<'hour' | 'day'>('day');
  const [loading, setLoading] = useState(true);

  const [overview, setOverview] = useState<OverviewData | null>(null);
  const [pages, setPages] = useState<RankedItem[]>([]);
  const [geo, setGeo] = useState<RankedItem[]>([]);
  const [browsers, setBrowsers] = useState<RankedItem[]>([]);
  const [osList, setOsList] = useState<RankedItem[]>([]);
  const [devices, setDevices] = useState<RankedItem[]>([]);
  const [referers, setReferers] = useState<RankedItem[]>([]);
  const [liveCount, setLiveCount] = useState(0);

  useEffect(() => {
    fetchApps();
  }, [fetchApps]);

  // Auto-select first app for non-admin users
  useEffect(() => {
    if (!isAdmin && apps.length > 0 && !selectedAppId) {
      setSelectedAppId(apps[0].id);
    }
  }, [apps, isAdmin, selectedAppId]);

  const fetchData = useCallback(async () => {
    // Non-admin must have an app selected
    if (!isAdmin && !selectedAppId) return;

    setLoading(true);
    const end = new Date().toISOString();
    const start = new Date(Date.now() - RANGE_MS[range]).toISOString();
    const params = {
      appId: selectedAppId || undefined,
      start,
      end,
      granularity,
    };

    try {
      const [ov, pg, ge, br, os, dv, rf, lv] = await Promise.all([
        analyticsApi.overview(params),
        analyticsApi.pages({ ...params, limit: 10 }),
        analyticsApi.geo({ ...params, limit: 10 }),
        analyticsApi.browsers(params),
        analyticsApi.os(params),
        analyticsApi.devices(params),
        analyticsApi.referers({ ...params, limit: 10 }),
        analyticsApi.live(selectedAppId || undefined),
      ]);
      setOverview(ov);
      setPages(pg);
      setGeo(ge);
      setBrowsers(br);
      setOsList(os);
      setDevices(dv);
      setReferers(rf);
      setLiveCount(lv.visitors);
    } catch {
      // silently fail
    } finally {
      setLoading(false);
    }
  }, [selectedAppId, range, granularity, isAdmin]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

  // Poll live visitors every 30s
  useEffect(() => {
    const interval = setInterval(() => {
      analyticsApi.live(selectedAppId || undefined).then((d) => setLiveCount(d.visitors)).catch(() => {});
    }, 30000);
    return () => clearInterval(interval);
  }, [selectedAppId]);

  const percentChange = (current: number, previous: number) => {
    if (previous === 0) return current > 0 ? 100 : 0;
    return ((current - previous) / previous) * 100;
  };

  const visitorsChange = overview ? percentChange(overview.visitors, overview.prevVisitors) : 0;
  const pageviewsChange = overview ? percentChange(overview.pageviews, overview.prevPageviews) : 0;

  const textPrimary = isDark ? 'text-zinc-100' : 'text-zinc-900';
  const textSecondary = isDark ? 'text-zinc-400' : 'text-zinc-500';
  const textMuted = isDark ? 'text-zinc-500' : 'text-zinc-400';
  const chartGridColor = isDark ? '#27272a' : '#e4e4e7';
  const chartTextColor = isDark ? '#71717a' : '#a1a1aa';

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-8">
        <div>
          <h1 className={`text-3xl font-bold tracking-tight ${textPrimary}`}>
            {t('analytics.title')}
          </h1>
          <p className={`text-sm mt-1 ${textSecondary}`}>{t('analytics.subtitle')}</p>
        </div>

        {/* Live Counter */}
        <div className={`flex items-center gap-2 px-4 py-2 rounded-full ${isDark ? 'bg-emerald-500/10' : 'bg-emerald-50'}`}>
          <span className="relative flex h-2.5 w-2.5">
            <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-emerald-400 opacity-75" />
            <span className="relative inline-flex rounded-full h-2.5 w-2.5 bg-emerald-500" />
          </span>
          <span className={`text-sm font-medium ${isDark ? 'text-emerald-400' : 'text-emerald-600'}`}>
            {liveCount} {t('analytics.visitorsNow')}
          </span>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3 mb-6">
        {/* App Selector */}
        <select
          value={selectedAppId}
          onChange={(e) => setSelectedAppId(e.target.value)}
          className={`
            px-3 py-2 rounded-xl text-sm border transition-colors
            ${isDark
              ? 'bg-zinc-900/50 border-zinc-700 text-zinc-200 focus:border-amber-500'
              : 'bg-white border-zinc-300 text-zinc-800 focus:border-amber-500'
            }
            focus:outline-none focus:ring-1 focus:ring-amber-500/30
          `}
        >
          {isAdmin && <option value="">{t('analytics.platform')}</option>}
          {apps.map((app) => (
            <option key={app.id} value={app.id}>{app.name}</option>
          ))}
        </select>

        {/* Date Range */}
        <div className={`flex rounded-xl overflow-hidden border ${isDark ? 'border-zinc-700' : 'border-zinc-300'}`}>
          {(['24h', '7d', '30d', '90d'] as DateRange[]).map((r) => (
            <button
              key={r}
              onClick={() => {
                setRange(r);
                setGranularity(r === '24h' ? 'hour' : 'day');
              }}
              className={`
                px-3 py-1.5 text-xs font-medium transition-colors
                ${range === r
                  ? 'bg-amber-500 text-white'
                  : isDark
                    ? 'bg-zinc-900/50 text-zinc-400 hover:text-zinc-200'
                    : 'bg-white text-zinc-500 hover:text-zinc-800'
                }
              `}
            >
              {r}
            </button>
          ))}
        </div>

        {/* Granularity */}
        {range !== '24h' && (
          <div className={`flex rounded-xl overflow-hidden border ${isDark ? 'border-zinc-700' : 'border-zinc-300'}`}>
            {(['hour', 'day'] as const).map((g) => (
              <button
                key={g}
                onClick={() => setGranularity(g)}
                className={`
                  px-3 py-1.5 text-xs font-medium transition-colors
                  ${granularity === g
                    ? 'bg-amber-500 text-white'
                    : isDark
                      ? 'bg-zinc-900/50 text-zinc-400 hover:text-zinc-200'
                      : 'bg-white text-zinc-500 hover:text-zinc-800'
                  }
                `}
              >
                {t(`analytics.${g}`)}
              </button>
            ))}
          </div>
        )}
      </div>

      {/* KPI Cards */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
        <KPICard
          label={t('analytics.uniqueVisitors')}
          value={overview?.visitors ?? 0}
          change={visitorsChange}
          icon={Users}
          color="text-amber-400"
          bg="bg-amber-500/10"
          loading={loading}
          isDark={isDark}
        />
        <KPICard
          label={t('analytics.totalPageviews')}
          value={overview?.pageviews ?? 0}
          change={pageviewsChange}
          icon={Eye}
          color="text-blue-400"
          bg="bg-blue-500/10"
          loading={loading}
          isDark={isDark}
        />
        <KPICard
          label={t('analytics.bounceRate')}
          value={formatPercent(overview?.bounceRate ?? 0)}
          icon={Activity}
          color="text-violet-400"
          bg="bg-violet-500/10"
          loading={loading}
          isDark={isDark}
        />
        <KPICard
          label={t('analytics.avgDuration')}
          value={overview ? formatMs(overview.avgDurationMs) : '0ms'}
          icon={Timer}
          color="text-emerald-400"
          bg="bg-emerald-500/10"
          loading={loading}
          isDark={isDark}
        />
      </div>

      {/* Main Chart */}
      <GlassCard className="mb-8">
        <h3 className={`text-sm font-semibold mb-4 ${textPrimary}`}>{t('analytics.traffic')}</h3>
        <div className="h-72">
          {loading ? (
            <div className="h-full animate-pulse rounded-xl bg-zinc-800/30" />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <LineChart data={overview?.timeSeries ?? []}>
                <CartesianGrid strokeDasharray="3 3" stroke={chartGridColor} />
                <XAxis
                  dataKey="bucket"
                  tick={{ fontSize: 11, fill: chartTextColor }}
                  tickFormatter={(v) => formatBucket(v, granularity)}
                />
                <YAxis tick={{ fontSize: 11, fill: chartTextColor }} />
                <Tooltip
                  contentStyle={{
                    backgroundColor: isDark ? '#18181b' : '#fff',
                    border: isDark ? '1px solid #27272a' : '1px solid #e4e4e7',
                    borderRadius: '12px',
                    fontSize: '12px',
                  }}
                  labelFormatter={(v) => formatBucket(v as string, granularity)}
                />
                <Line type="monotone" dataKey="visitors" stroke="#f59e0b" strokeWidth={2} dot={false} name={t('analytics.visitors')} />
                <Line type="monotone" dataKey="views" stroke="#3b82f6" strokeWidth={2} dot={false} name={t('analytics.pageviews')} />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </GlassCard>

      {/* Row: Geo + Top Pages */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-8">
        {/* Countries */}
        <GlassCard>
          <h3 className={`text-sm font-semibold mb-4 ${textPrimary}`}>
            <Globe size={16} className="inline mr-2" />
            {t('analytics.topCountries')}
          </h3>
          {loading ? <SkeletonList /> : (
            <div className="space-y-3">
              {geo.length === 0 && <EmptyLabel isDark={isDark} t={t} />}
              {geo.map((item, i) => {
                const maxViews = geo[0]?.visitors ?? 1;
                const flag = COUNTRY_FLAGS[item.name] ?? '🏳️';
                return (
                  <div key={i} className="flex items-center gap-3">
                    <span className="text-lg">{flag}</span>
                    <div className="flex-1 min-w-0">
                      <div className="flex items-center justify-between mb-1">
                        <span className={`text-xs font-medium truncate ${textPrimary}`}>{item.name}</span>
                        <span className={`text-xs tabular-nums ${textMuted}`}>{item.visitors}</span>
                      </div>
                      <div className={`h-1.5 rounded-full ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                        <div
                          className="h-full rounded-full bg-amber-500 transition-all duration-500"
                          style={{ width: `${(item.visitors / maxViews) * 100}%` }}
                        />
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </GlassCard>

        {/* Top Pages */}
        <GlassCard>
          <h3 className={`text-sm font-semibold mb-4 ${textPrimary}`}>
            <Eye size={16} className="inline mr-2" />
            {t('analytics.topPages')}
          </h3>
          {loading ? <SkeletonList /> : (
            <div className="space-y-2">
              {pages.length === 0 && <EmptyLabel isDark={isDark} t={t} />}
              {pages.map((item, i) => (
                <div
                  key={i}
                  className={`flex items-center justify-between px-3 py-2 rounded-xl transition-colors ${isDark ? 'hover:bg-zinc-800/50' : 'hover:bg-zinc-100'}`}
                >
                  <span className={`text-xs font-mono truncate flex-1 ${textPrimary}`}>{item.name}</span>
                  <div className="flex items-center gap-4 ml-4 shrink-0">
                    <span className={`text-xs tabular-nums ${textMuted}`}>{item.views} views</span>
                    <span className={`text-xs tabular-nums ${textMuted}`}>{item.visitors} <Users size={10} className="inline" /></span>
                  </div>
                </div>
              ))}
            </div>
          )}
        </GlassCard>
      </div>

      {/* Row: Browser + OS + Devices (donut charts) */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-8">
        <DonutCard
          title={t('analytics.browsers')}
          icon={Chrome}
          data={browsers}
          loading={loading}
          isDark={isDark}
          textPrimary={textPrimary}
        />
        <DonutCard
          title={t('analytics.operatingSystems')}
          icon={Monitor}
          data={osList}
          loading={loading}
          isDark={isDark}
          textPrimary={textPrimary}
        />
        <DevicesCard
          title={t('analytics.devices')}
          data={devices}
          loading={loading}
          isDark={isDark}
          textPrimary={textPrimary}
          textMuted={textMuted}
        />
      </div>

      {/* Row: Referrers */}
      <GlassCard>
        <h3 className={`text-sm font-semibold mb-4 ${textPrimary}`}>
          <ExternalLink size={16} className="inline mr-2" />
          {t('analytics.referers')}
        </h3>
        {loading ? <SkeletonList /> : (
          <div className="space-y-2">
            {referers.length === 0 && <EmptyLabel isDark={isDark} t={t} />}
            {referers.map((item, i) => (
              <div
                key={i}
                className={`flex items-center justify-between px-3 py-2 rounded-xl transition-colors ${isDark ? 'hover:bg-zinc-800/50' : 'hover:bg-zinc-100'}`}
              >
                <div className="flex items-center gap-2 flex-1 min-w-0">
                  {item.name !== 'direct' && (
                    <img
                      src={`https://www.google.com/s2/favicons?domain=${item.name}&sz=16`}
                      alt=""
                      className="w-4 h-4 rounded-sm"
                      loading="lazy"
                    />
                  )}
                  <span className={`text-xs truncate ${textPrimary}`}>
                    {item.name === 'direct' ? t('analytics.directTraffic') : item.name}
                  </span>
                </div>
                <div className="flex items-center gap-4 ml-4 shrink-0">
                  <span className={`text-xs tabular-nums ${textMuted}`}>{item.visitors} <Users size={10} className="inline" /></span>
                  <span className={`text-xs tabular-nums ${textMuted}`}>{item.views} views</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </GlassCard>
    </div>
  );
}

// ---- Subcomponents ----

interface KPICardProps {
  label: string;
  value: number | string;
  change?: number;
  icon: typeof Users;
  color: string;
  bg: string;
  loading: boolean;
  isDark: boolean;
}

function KPICard({ label, value, change, icon: Icon, color, bg, loading, isDark }: KPICardProps) {
  return (
    <GlassCard className="!p-4 group hover:scale-[1.02] transition-transform duration-200">
      <div className="flex items-center gap-3">
        <div className={`w-10 h-10 rounded-xl flex items-center justify-center ${bg}`}>
          <Icon size={18} className={color} />
        </div>
        <div className="flex-1 min-w-0">
          <p className={`text-2xl font-bold tabular-nums ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {loading ? <span className="inline-block w-12 h-6 animate-pulse bg-zinc-700/30 rounded" /> : value}
          </p>
          <div className="flex items-center gap-2">
            <p className="text-[11px] text-zinc-500 font-medium truncate">{label}</p>
            {change !== undefined && !loading && (
              <span className={`text-[10px] font-semibold flex items-center gap-0.5 ${change >= 0 ? 'text-emerald-400' : 'text-red-400'}`}>
                {change >= 0 ? <TrendingUp size={10} /> : <TrendingDown size={10} />}
                {Math.abs(Math.round(change))}%
              </span>
            )}
          </div>
        </div>
      </div>
    </GlassCard>
  );
}

interface DonutCardProps {
  title: string;
  icon: typeof Chrome;
  data: RankedItem[];
  loading: boolean;
  isDark: boolean;
  textPrimary: string;
}

function DonutCard({ title, icon: Icon, data, loading, isDark, textPrimary }: DonutCardProps) {
  const total = data.reduce((sum, d) => sum + d.visitors, 0);

  return (
    <GlassCard>
      <h3 className={`text-sm font-semibold mb-4 ${textPrimary}`}>
        <Icon size={16} className="inline mr-2" />
        {title}
      </h3>
      {loading ? (
        <div className="h-40 animate-pulse rounded-xl bg-zinc-800/30" />
      ) : data.length === 0 ? (
        <p className="text-xs text-zinc-500 text-center py-8">—</p>
      ) : (
        <div className="flex items-center gap-4">
          <div className="w-28 h-28 shrink-0">
            <ResponsiveContainer width="100%" height="100%">
              <PieChart>
                <Pie
                  data={data.map((d) => ({ name: d.name, value: d.visitors }))}
                  dataKey="value"
                  cx="50%"
                  cy="50%"
                  innerRadius={30}
                  outerRadius={50}
                  paddingAngle={2}
                  stroke="none"
                >
                  {data.map((_, i) => (
                    <Cell key={i} fill={PIE_COLORS[i % PIE_COLORS.length]} />
                  ))}
                </Pie>
              </PieChart>
            </ResponsiveContainer>
          </div>
          <div className="flex-1 space-y-1.5">
            {data.slice(0, 5).map((item, i) => (
              <div key={i} className="flex items-center gap-2">
                <div className="w-2 h-2 rounded-full shrink-0" style={{ backgroundColor: PIE_COLORS[i % PIE_COLORS.length] }} />
                <span className={`text-[11px] truncate flex-1 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{item.name}</span>
                <span className="text-[10px] text-zinc-500 tabular-nums">{total > 0 ? Math.round((item.visitors / total) * 100) : 0}%</span>
              </div>
            ))}
          </div>
        </div>
      )}
    </GlassCard>
  );
}

interface DevicesCardProps {
  title: string;
  data: RankedItem[];
  loading: boolean;
  isDark: boolean;
  textPrimary: string;
  textMuted: string;
}

function DevicesCard({ title, data, loading, isDark, textPrimary, textMuted }: DevicesCardProps) {
  const total = data.reduce((sum, d) => sum + d.visitors, 0);

  return (
    <GlassCard>
      <h3 className={`text-sm font-semibold mb-4 ${textPrimary}`}>
        <Monitor size={16} className="inline mr-2" />
        {title}
      </h3>
      {loading ? (
        <div className="h-40 animate-pulse rounded-xl bg-zinc-800/30" />
      ) : data.length === 0 ? (
        <p className="text-xs text-zinc-500 text-center py-8">—</p>
      ) : (
        <div className="space-y-4 py-2">
          {data.map((item, i) => {
            const Icon = DEVICE_ICONS[item.name] ?? Monitor;
            const pct = total > 0 ? Math.round((item.visitors / total) * 100) : 0;
            return (
              <div key={i} className="flex items-center gap-3">
                <div className={`w-8 h-8 rounded-lg flex items-center justify-center ${isDark ? 'bg-zinc-800' : 'bg-zinc-100'}`}>
                  <Icon size={16} className={isDark ? 'text-zinc-400' : 'text-zinc-600'} />
                </div>
                <div className="flex-1">
                  <div className="flex items-center justify-between mb-1">
                    <span className={`text-xs font-medium capitalize ${textPrimary}`}>{item.name}</span>
                    <span className={`text-[11px] tabular-nums ${textMuted}`}>{pct}%</span>
                  </div>
                  <div className={`h-1.5 rounded-full ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                    <div
                      className="h-full rounded-full transition-all duration-500"
                      style={{
                        width: `${pct}%`,
                        backgroundColor: PIE_COLORS[i % PIE_COLORS.length],
                      }}
                    />
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </GlassCard>
  );
}

function SkeletonList() {
  return (
    <div className="space-y-3">
      {[1, 2, 3, 4, 5].map((i) => (
        <div key={i} className="h-6 animate-pulse bg-zinc-800/20 rounded-lg" />
      ))}
    </div>
  );
}

function EmptyLabel({ isDark, t }: { isDark: boolean; t: (k: string) => string }) {
  return <p className={`text-xs text-center py-6 ${isDark ? 'text-zinc-600' : 'text-zinc-400'}`}>{t('analytics.noData')}</p>;
}

function formatBucket(bucket: string, granularity: 'hour' | 'day'): string {
  const d = new Date(bucket);
  if (granularity === 'hour') {
    return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  }
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
}

function formatMs(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  return `${(ms / 1000).toFixed(1)}s`;
}

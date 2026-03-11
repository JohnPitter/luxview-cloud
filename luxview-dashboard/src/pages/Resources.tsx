import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Database,
  Server,
  Eye,
  EyeOff,
  Copy,
  Check,
  ExternalLink,
  RefreshCw,
  HardDrive,
  Inbox,
  HardDriveIcon,
  Terminal,
  FolderOpen,
  Crown,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PageTour } from '../components/common/PageTour';
import { useThemeStore } from '../stores/theme.store';
import { useAuthStore } from '../stores/auth.store';
import { servicesApi, type AppServiceWithApp, type ServiceType, type StorageUsageInfo } from '../api/services';
import { resourcesTourSteps } from '../tours/resources';

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

const serviceConfig: Record<
  ServiceType,
  { labelKey: string; icon: string; color: string; bgColor: string; borderColor: string; explorable?: boolean }
> = {
  postgres: {
    labelKey: 'resources.service.postgresql',
    icon: 'PG',
    color: 'text-blue-400',
    bgColor: 'bg-blue-500/10',
    borderColor: 'border-blue-500/20',
    explorable: true,
  },
  redis: {
    labelKey: 'resources.service.redis',
    icon: 'RD',
    color: 'text-red-400',
    bgColor: 'bg-red-500/10',
    borderColor: 'border-red-500/20',
  },
  mongodb: {
    labelKey: 'resources.service.mongodb',
    icon: 'MG',
    color: 'text-emerald-400',
    bgColor: 'bg-emerald-500/10',
    borderColor: 'border-emerald-500/20',
  },
  rabbitmq: {
    labelKey: 'resources.service.rabbitmq',
    icon: 'RQ',
    color: 'text-orange-400',
    bgColor: 'bg-orange-500/10',
    borderColor: 'border-orange-500/20',
  },
  storage: {
    labelKey: 'resources.service.storage',
    icon: 'ST',
    color: 'text-purple-400',
    bgColor: 'bg-purple-500/10',
    borderColor: 'border-purple-500/20',
    explorable: true,
  },
};

export function Resources() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const navigate = useNavigate();
  const { t } = useTranslation();

  const user = useAuthStore((s) => s.user);
  const plan = user?.plan;

  const [services, setServices] = useState<AppServiceWithApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});
  const [copied, setCopied] = useState<string | null>(null);
  const [serviceUsages, setServiceUsages] = useState<Record<string, StorageUsageInfo>>({});

  const categoryConfig = [
    {
      key: 'databases',
      label: t('resources.categories.databases'),
      description: t('resources.categories.databasesDescription'),
      icon: Database,
      types: ['postgres', 'mongodb'] as ServiceType[],
    },
    {
      key: 'cache',
      label: t('resources.categories.cacheQueues'),
      description: t('resources.categories.cacheQueuesDescription'),
      icon: Server,
      types: ['redis', 'rabbitmq'] as ServiceType[],
    },
    {
      key: 'storage',
      label: t('resources.categories.objectStorage'),
      description: t('resources.categories.objectStorageDescription'),
      icon: HardDriveIcon,
      types: ['storage'] as ServiceType[],
    },
  ];

  const fetchServices = useCallback(() => {
    setLoading(true);
    servicesApi
      .listAll()
      .then(setServices)
      .catch(() => {})
      .finally(() => setLoading(false));
  }, []);

  useEffect(() => {
    fetchServices();
    // Auto-refresh services every 30s
    const interval = setInterval(() => {
      servicesApi.listAll().then(setServices).catch(() => {});
    }, 30000);
    return () => clearInterval(interval);
  }, [fetchServices]);

  // Fetch usage for all services
  useEffect(() => {
    if (services.length === 0) return;
    services.forEach((svc) => {
      servicesApi.getServiceUsage(svc.id).then((usage) => {
        setServiceUsages((prev) => ({ ...prev, [svc.id]: usage }));
      }).catch(() => {});
    });
  }, [services]);

  const togglePassword = (id: string) =>
    setShowPasswords((prev) => ({ ...prev, [id]: !prev[id] }));

  const copyToClipboard = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopied(id);
    setTimeout(() => setCopied(null), 2000);
  };

  const totalByType = (types: ServiceType[]) =>
    services.filter((s) => types.includes(s.serviceType)).length;

  return (
    <div className="animate-fade-in">
      <PageTour tourId="resources" steps={resourcesTourSteps} autoStart />

      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            {t('resources.title')}
          </h1>
          <p className="text-sm text-zinc-500 mt-1">
            {services.length > 0
              ? t('resources.subtitle.withCount', { count: services.length })
              : t('resources.subtitle.empty')}
          </p>
        </div>
        <button
          onClick={fetchServices}
          className={`
            flex items-center gap-2 px-3 py-2 rounded-xl text-sm font-medium
            transition-all duration-200
            ${
              isDark
                ? 'bg-zinc-800/60 text-zinc-300 hover:bg-zinc-800 border border-zinc-700/50'
                : 'bg-white text-zinc-600 hover:bg-zinc-50 border border-zinc-200'
            }
          `}
        >
          <RefreshCw size={14} className={loading ? 'animate-spin' : ''} />
          {t('common.refresh')}
        </button>
      </div>

      {/* Stats Overview */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-8">
        {[
          { label: t('resources.stats.totalResources'), value: services.length, icon: HardDrive, color: 'text-amber-400' },
          {
            label: t('resources.stats.databases'),
            value: totalByType(['postgres', 'mongodb']),
            icon: Database,
            color: 'text-blue-400',
          },
          { label: t('resources.stats.caches'), value: totalByType(['redis']), icon: Server, color: 'text-red-400' },
          {
            label: t('resources.stats.queues'),
            value: totalByType(['rabbitmq']),
            icon: Inbox,
            color: 'text-orange-400',
          },
          {
            label: t('resources.stats.storage'),
            value: totalByType(['storage']),
            icon: HardDriveIcon,
            color: 'text-purple-400',
          },
        ].map((stat) => (
          <GlassCard key={stat.label} className="!p-4">
            <div className="flex items-center gap-3">
              <div
                className={`w-10 h-10 rounded-xl flex items-center justify-center ${
                  isDark ? 'bg-zinc-800/80' : 'bg-zinc-100'
                }`}
              >
                <stat.icon size={18} className={stat.color} />
              </div>
              <div>
                <p className={`text-xl font-bold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                  {loading ? '-' : stat.value}
                </p>
                <p className="text-[11px] text-zinc-500 font-medium">{stat.label}</p>
              </div>
            </div>
          </GlassCard>
        ))}
      </div>

      {/* Plan Limits */}
      {plan && (
        <GlassCard className="!p-4 mb-8">
          <div className="flex items-center gap-2 mb-3">
            <Crown size={16} className="text-amber-400" />
            <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {t('resources.planLimits.title', { plan: plan.name })}
            </h3>
          </div>
          <div className="grid grid-cols-2 md:grid-cols-4 gap-4">
            <div>
              <p className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider mb-0.5">
                {t('resources.planLimits.maxApps')}
              </p>
              <p className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {plan.maxApps}
              </p>
            </div>
            <div>
              <p className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider mb-0.5">
                {t('resources.planLimits.maxServicesPerApp')}
              </p>
              <p className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {plan.maxServicesPerApp}
              </p>
            </div>
            <div>
              <p className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider mb-0.5">
                {t('resources.planLimits.maxDiskPerApp')}
              </p>
              <p className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {plan.maxDiskPerApp}
              </p>
            </div>
            <div>
              <p className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider mb-0.5">
                {t('resources.planLimits.maxResources')}
              </p>
              <p className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {plan.maxCpuPerApp} CPU · {plan.maxMemoryPerApp} RAM
              </p>
            </div>
          </div>
        </GlassCard>
      )}

      {/* Loading */}
      {loading && services.length === 0 && (
        <div className="space-y-6">
          {[1, 2].map((i) => (
            <div key={i} className="space-y-3">
              <div
                className={`h-6 w-40 rounded-lg animate-pulse ${
                  isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'
                }`}
              />
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {[1, 2].map((j) => (
                  <div
                    key={j}
                    className={`h-48 rounded-2xl animate-pulse ${
                      isDark ? 'bg-zinc-900/50' : 'bg-zinc-100'
                    }`}
                  />
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      {/* Empty state */}
      {!loading && services.length === 0 && (
        <GlassCard className="!py-16 text-center">
          <Database size={40} className="mx-auto text-zinc-500 mb-4" />
          <h3
            className={`text-lg font-semibold mb-2 ${
              isDark ? 'text-zinc-200' : 'text-zinc-800'
            }`}
          >
            {t('resources.emptyState.title')}
          </h3>
          <p className="text-sm text-zinc-500 max-w-md mx-auto">
            {t('resources.emptyState.description')}
          </p>
        </GlassCard>
      )}

      {/* Categories */}
      {!loading &&
        services.length > 0 && (
          <div data-tour="services-list">
            {categoryConfig.map((cat) => {
              const catServices = services.filter((s) => cat.types.includes(s.serviceType));
              if (catServices.length === 0) return null;
              const CatIcon = cat.icon;
              return (
                <div key={cat.key} className="mb-8">
                  <div className="flex items-center gap-2 mb-1">
                    <CatIcon size={16} className="text-zinc-500" />
                    <h2
                      className={`text-sm font-semibold uppercase tracking-wider ${
                        isDark ? 'text-zinc-400' : 'text-zinc-600'
                      }`}
                    >
                      {cat.label}
                    </h2>
                    <span
                      className={`text-[11px] font-medium px-2 py-0.5 rounded-full ${
                        isDark ? 'bg-zinc-800 text-zinc-400' : 'bg-zinc-200 text-zinc-500'
                      }`}
                    >
                      {catServices.length}
                    </span>
                  </div>
                  <p className="text-xs text-zinc-500 mb-4">{cat.description}</p>

                  <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                    {catServices.map((svc) => {
                      const cfg = serviceConfig[svc.serviceType];
                      const creds = svc.credentials;
                      const urlKey = `url-${svc.id}`;
                      const pwKey = `pw-${svc.id}`;

                      return (
                        <GlassCard key={svc.id} className="group relative overflow-hidden">
                          {/* Header */}
                          <div className="flex items-start justify-between mb-4">
                            <div className="flex items-center gap-3">
                              <div
                                className={`w-10 h-10 rounded-xl flex items-center justify-center font-bold text-xs border ${cfg.bgColor} ${cfg.color} ${cfg.borderColor}`}
                              >
                                {cfg.icon}
                              </div>
                              <div>
                                <h3
                                  className={`text-sm font-semibold ${
                                    isDark ? 'text-zinc-100' : 'text-zinc-900'
                                  }`}
                                >
                                  {t(cfg.labelKey)}
                                </h3>
                                <button
                                  onClick={() => navigate(`/dashboard/apps/${svc.appId}`)}
                                  className="flex items-center gap-1 text-xs text-zinc-500 hover:text-amber-400 transition-colors"
                                >
                                  {svc.appName || t('resources.service.unknownApp')}
                                  <ExternalLink size={10} />
                                </button>
                              </div>
                            </div>
                            <span
                              className={`text-[10px] font-medium px-2 py-0.5 rounded-full ${
                                isDark
                                  ? 'bg-emerald-500/10 text-emerald-400'
                                  : 'bg-emerald-100 text-emerald-700'
                              }`}
                            >
                              {t('common.status.active')}
                            </span>
                          </div>

                          {/* Connection URL */}
                          {creds?.url && (
                            <div className="mb-3">
                              <label className="text-[11px] text-zinc-500 font-medium uppercase tracking-wider mb-1 block">
                                {t('common.field.connectionUrl')}
                              </label>
                              <div
                                className={`flex items-center gap-2 px-3 py-2 rounded-lg font-mono text-xs ${
                                  isDark
                                    ? 'bg-zinc-900/60 border border-zinc-800/50'
                                    : 'bg-zinc-50 border border-zinc-200/60'
                                }`}
                              >
                                <span
                                  className={`flex-1 truncate ${
                                    isDark ? 'text-zinc-300' : 'text-zinc-700'
                                  }`}
                                >
                                  {showPasswords[urlKey]
                                    ? creds.url
                                    : creds.url.replace(/\/\/[^@]+@/, '//***:***@')}
                                </span>
                                <button
                                  onClick={() => togglePassword(urlKey)}
                                  className="text-zinc-500 hover:text-zinc-300 transition-colors"
                                >
                                  {showPasswords[urlKey] ? <EyeOff size={13} /> : <Eye size={13} />}
                                </button>
                                <button
                                  onClick={() => copyToClipboard(creds.url, urlKey)}
                                  className="text-zinc-500 hover:text-amber-400 transition-colors"
                                >
                                  {copied === urlKey ? (
                                    <Check size={13} className="text-emerald-400" />
                                  ) : (
                                    <Copy size={13} />
                                  )}
                                </button>
                              </div>
                            </div>
                          )}

                          {/* Explorer action */}
                          {cfg.explorable && (
                            <div className="mb-3">
                              <button
                                onClick={() => {
                                  if (svc.serviceType === 'storage') {
                                    navigate(`/dashboard/resources/storage/${svc.id}`);
                                  } else {
                                    navigate(`/dashboard/resources/db/${svc.id}`);
                                  }
                                }}
                                className={`
                                  w-full flex items-center justify-center gap-2 px-3 py-2 rounded-xl text-xs font-medium
                                  transition-all duration-200
                                  ${
                                    isDark
                                      ? 'bg-amber-400/10 text-amber-400 hover:bg-amber-400/20 border border-amber-400/20'
                                      : 'bg-amber-100 text-amber-700 hover:bg-amber-200 border border-amber-300'
                                  }
                                `}
                              >
                                {svc.serviceType === 'storage' ? (
                                  <>
                                    <FolderOpen size={14} />
                                    {t('resources.explorer.browseFiles')}
                                  </>
                                ) : (
                                  <>
                                    <Terminal size={14} />
                                    {t('resources.explorer.openExplorer')}
                                  </>
                                )}
                              </button>
                            </div>
                          )}

                          {/* Credentials Grid */}
                          <div className="grid grid-cols-2 gap-2">
                            {(svc.serviceType === 'storage'
                              ? [
                                  { label: t('common.field.endpoint'), value: creds?.endpoint },
                                  { label: t('common.field.bucket'), value: creds?.bucket },
                                  { label: t('common.field.accessKey'), value: creds?.accessKey },
                                ]
                              : [
                                  { label: t('common.field.host'), value: creds?.host },
                                  { label: t('common.field.port'), value: creds?.port },
                                  { label: t('common.field.user'), value: creds?.username },
                                  { label: t('common.field.database'), value: creds?.database },
                                ]
                            )
                              .filter((c) => c.value)
                              .map((cred) => (
                                <div key={cred.label}>
                                  <label className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider">
                                    {cred.label}
                                  </label>
                                  <div className="flex items-center gap-1">
                                    <span
                                      className={`text-xs font-mono truncate ${
                                        isDark ? 'text-zinc-300' : 'text-zinc-700'
                                      }`}
                                    >
                                      {cred.value}
                                    </span>
                                    <button
                                      onClick={() =>
                                        copyToClipboard(cred.value!, `${cred.label}-${svc.id}`)
                                      }
                                      className="text-zinc-500 hover:text-amber-400 transition-colors flex-shrink-0"
                                    >
                                      {copied === `${cred.label}-${svc.id}` ? (
                                        <Check size={11} className="text-emerald-400" />
                                      ) : (
                                        <Copy size={11} />
                                      )}
                                    </button>
                                  </div>
                                </div>
                              ))}
                            {/* Disk Usage */}
                            {(() => {
                              const usage = serviceUsages[svc.id];
                              if (!usage || usage.limit <= 0) return null;
                              const pct = usage.limit > 0 ? usage.used / usage.limit : 0;
                              return (
                                <div className="col-span-2 mt-1">
                                  <label className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider">
                                    {t('resources.service.diskUsage')}
                                  </label>
                                  <div className="flex items-center gap-2 mt-1">
                                    <div className={`flex-1 h-2 rounded-full overflow-hidden ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                                      <div
                                        className={`h-full rounded-full transition-all duration-500 ${
                                          pct > 0.9 ? 'bg-red-500' : pct > 0.7 ? 'bg-amber-500' : cfg.color.replace('text-', 'bg-')
                                        }`}
                                        style={{ width: `${Math.min(pct * 100, 100)}%` }}
                                      />
                                    </div>
                                    <span className={`text-[10px] font-mono flex-shrink-0 ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                                      {usage.used > 0 ? formatSize(usage.used) : '-'} / {usage.limitStr}
                                    </span>
                                  </div>
                                </div>
                              );
                            })()}

                            {/* Password / Secret Key */}
                            {(creds?.password || creds?.secretKey) && (() => {
                              const secretValue = creds?.password || creds?.secretKey || '';
                              const secretLabel = svc.serviceType === 'storage' ? t('common.field.secretKey') : t('common.field.password');
                              return (
                              <div>
                                <label className="text-[10px] text-zinc-500 font-medium uppercase tracking-wider">
                                  {secretLabel}
                                </label>
                                <div className="flex items-center gap-1">
                                  <span
                                    className={`text-xs font-mono truncate ${
                                      isDark ? 'text-zinc-300' : 'text-zinc-700'
                                    }`}
                                  >
                                    {showPasswords[pwKey] ? secretValue : '••••••••'}
                                  </span>
                                  <button
                                    onClick={() => togglePassword(pwKey)}
                                    className="text-zinc-500 hover:text-zinc-300 transition-colors flex-shrink-0"
                                  >
                                    {showPasswords[pwKey] ? (
                                      <EyeOff size={11} />
                                    ) : (
                                      <Eye size={11} />
                                    )}
                                  </button>
                                  <button
                                    onClick={() => copyToClipboard(secretValue, pwKey)}
                                    className="text-zinc-500 hover:text-amber-400 transition-colors flex-shrink-0"
                                  >
                                    {copied === pwKey ? (
                                      <Check size={11} className="text-emerald-400" />
                                    ) : (
                                      <Copy size={11} />
                                    )}
                                  </button>
                                </div>
                              </div>
                              );
                            })()}
                          </div>
                        </GlassCard>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        )}
    </div>
  );
}

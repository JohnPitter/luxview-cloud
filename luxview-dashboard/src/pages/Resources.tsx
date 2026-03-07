import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
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
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { useThemeStore } from '../stores/theme.store';
import { servicesApi, type AppServiceWithApp, type ServiceType } from '../api/services';

const serviceConfig: Record<
  ServiceType,
  { label: string; icon: string; color: string; bgColor: string; borderColor: string; explorable?: boolean }
> = {
  postgres: {
    label: 'PostgreSQL',
    icon: 'PG',
    color: 'text-blue-400',
    bgColor: 'bg-blue-500/10',
    borderColor: 'border-blue-500/20',
    explorable: true,
  },
  redis: {
    label: 'Redis',
    icon: 'RD',
    color: 'text-red-400',
    bgColor: 'bg-red-500/10',
    borderColor: 'border-red-500/20',
  },
  mongodb: {
    label: 'MongoDB',
    icon: 'MG',
    color: 'text-emerald-400',
    bgColor: 'bg-emerald-500/10',
    borderColor: 'border-emerald-500/20',
  },
  rabbitmq: {
    label: 'RabbitMQ',
    icon: 'RQ',
    color: 'text-orange-400',
    bgColor: 'bg-orange-500/10',
    borderColor: 'border-orange-500/20',
  },
  s3: {
    label: 'Object Storage',
    icon: 'S3',
    color: 'text-purple-400',
    bgColor: 'bg-purple-500/10',
    borderColor: 'border-purple-500/20',
    explorable: true,
  },
};

const categoryConfig = [
  {
    key: 'databases',
    label: 'Databases',
    description: 'PostgreSQL and MongoDB instances attached to your apps',
    icon: Database,
    types: ['postgres', 'mongodb'] as ServiceType[],
  },
  {
    key: 'cache',
    label: 'Cache & Queues',
    description: 'Redis caches and RabbitMQ message brokers',
    icon: Server,
    types: ['redis', 'rabbitmq'] as ServiceType[],
  },
  {
    key: 'storage',
    label: 'Object Storage',
    description: 'S3-compatible file storage buckets',
    icon: HardDriveIcon,
    types: ['s3'] as ServiceType[],
  },
];

export function Resources() {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const navigate = useNavigate();

  const [services, setServices] = useState<AppServiceWithApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [showPasswords, setShowPasswords] = useState<Record<string, boolean>>({});
  const [copied, setCopied] = useState<string | null>(null);

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
  }, [fetchServices]);

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
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            Resources
          </h1>
          <p className="text-sm text-zinc-500 mt-1">
            {services.length > 0
              ? `${services.length} resource${services.length !== 1 ? 's' : ''} across your apps`
              : 'Manage databases, caches and message brokers'}
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
          Refresh
        </button>
      </div>

      {/* Stats Overview */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4 mb-8">
        {[
          { label: 'Total Resources', value: services.length, icon: HardDrive, color: 'text-amber-400' },
          {
            label: 'Databases',
            value: totalByType(['postgres', 'mongodb']),
            icon: Database,
            color: 'text-blue-400',
          },
          { label: 'Caches', value: totalByType(['redis']), icon: Server, color: 'text-red-400' },
          {
            label: 'Queues',
            value: totalByType(['rabbitmq']),
            icon: Inbox,
            color: 'text-orange-400',
          },
          {
            label: 'Storage',
            value: totalByType(['s3']),
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
            No resources provisioned
          </h3>
          <p className="text-sm text-zinc-500 max-w-md mx-auto">
            Resources are created from within your app's settings. Go to an app, open the Services
            tab, and add a database or cache.
          </p>
        </GlassCard>
      )}

      {/* Categories */}
      {!loading &&
        services.length > 0 &&
        categoryConfig.map((cat) => {
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
                              {cfg.label}
                            </h3>
                            <button
                              onClick={() => navigate(`/dashboard/apps/${svc.appId}`)}
                              className="flex items-center gap-1 text-xs text-zinc-500 hover:text-amber-400 transition-colors"
                            >
                              {svc.appName || 'Unknown App'}
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
                          Active
                        </span>
                      </div>

                      {/* Connection URL */}
                      {creds?.url && (
                        <div className="mb-3">
                          <label className="text-[11px] text-zinc-500 font-medium uppercase tracking-wider mb-1 block">
                            Connection URL
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
                              if (svc.serviceType === 's3') {
                                navigate(`/dashboard/resources/s3/${svc.id}`);
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
                            {svc.serviceType === 's3' ? (
                              <>
                                <FolderOpen size={14} />
                                Browse Files
                              </>
                            ) : (
                              <>
                                <Terminal size={14} />
                                Open Explorer
                              </>
                            )}
                          </button>
                        </div>
                      )}

                      {/* Credentials Grid */}
                      <div className="grid grid-cols-2 gap-2">
                        {(svc.serviceType === 's3'
                          ? [
                              { label: 'Endpoint', value: creds?.endpoint },
                              { label: 'Bucket', value: creds?.bucket },
                              { label: 'Access Key', value: creds?.accessKey },
                            ]
                          : [
                              { label: 'Host', value: creds?.host },
                              { label: 'Port', value: creds?.port },
                              { label: 'User', value: creds?.username },
                              { label: 'Database', value: creds?.database },
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
                        {/* Password / Secret Key */}
                        {(creds?.password || creds?.secretKey) && (() => {
                          const secretValue = creds?.password || creds?.secretKey || '';
                          const secretLabel = svc.serviceType === 's3' ? 'Secret Key' : 'Password';
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
  );
}

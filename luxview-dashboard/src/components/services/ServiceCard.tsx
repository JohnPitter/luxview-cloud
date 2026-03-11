import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Eye, EyeOff, Copy, Trash2, Check, HardDrive } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { useThemeStore } from '../../stores/theme.store';
import { servicesApi, type AppService, type ServiceType, type StorageUsageInfo } from '../../api/services';

interface ServiceCardProps {
  service: AppService;
  onDelete: (serviceId: string) => void;
}

const serviceConfig: Record<ServiceType, { labelKey: string; color: string; icon: string }> = {
  postgres: { labelKey: 'resources.service.postgresql', color: 'text-blue-400', icon: 'PG' },
  redis: { labelKey: 'resources.service.redis', color: 'text-red-400', icon: 'RD' },
  mongodb: { labelKey: 'resources.service.mongodb', color: 'text-emerald-400', icon: 'MG' },
  rabbitmq: { labelKey: 'resources.service.rabbitmq', color: 'text-orange-400', icon: 'RQ' },
  storage: { labelKey: 'resources.service.storage', color: 'text-purple-400', icon: 'ST' },
};

function formatStorageSize(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`;
}

export function ServiceCard({ service, onDelete }: ServiceCardProps) {
  const [showCreds, setShowCreds] = useState(false);
  const [copied, setCopied] = useState(false);
  const [storageUsage, setStorageUsage] = useState<StorageUsageInfo | null>(null);
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const { t } = useTranslation();
  const config = serviceConfig[service.serviceType];

  useEffect(() => {
    servicesApi.getServiceUsage(service.id).then(setStorageUsage).catch(() => {});
  }, [service.id]);

  const copyUrl = async () => {
    await navigator.clipboard.writeText((service.credentials?.url || ""));
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  return (
    <GlassCard>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center gap-3">
          <div
            className={`
              flex items-center justify-center w-10 h-10 rounded-xl
              ${isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'}
            `}
          >
            <span className={`text-xs font-bold ${config.color}`}>{config.icon}</span>
          </div>
          <div>
            <h4
              className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
            >
              {t(config.labelKey)}
            </h4>
            <p className="text-[11px] text-zinc-500">{service.dbName}</p>
          </div>
        </div>
        <button
          onClick={() => onDelete(service.id)}
          className="text-zinc-500 hover:text-red-400 transition-colors p-1"
          title={t('resources.service.removeService')}
        >
          <Trash2 size={14} />
        </button>
      </div>

      {service.serviceType === 'storage' ? (
        /* Storage-specific layout */
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <HardDrive size={14} className="text-purple-400" />
            <span className="text-[11px] font-medium text-zinc-500 uppercase tracking-wider">
              {t('services.card.storagePath')}
            </span>
          </div>
          <div
            className={`
              px-3 py-2 rounded-lg font-mono text-xs
              ${isDark ? 'bg-zinc-900/50 border border-zinc-800 text-zinc-300' : 'bg-zinc-50 border border-zinc-200 text-zinc-700'}
            `}
          >
            {service.credentials?.container_path || '/storage'}
          </div>

          {/* Storage usage bar */}
          {storageUsage && storageUsage.limit > 0 && (
            <div className="space-y-1.5">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-medium text-zinc-500 uppercase tracking-wider">
                  {t('services.card.storageUsage')}
                </span>
                <span className={`text-[11px] font-mono ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                  {formatStorageSize(storageUsage.used)} / {storageUsage.limitStr}
                </span>
              </div>
              <div className={`w-full h-2 rounded-full overflow-hidden ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                <div
                  className={`h-full rounded-full transition-all duration-500 ${
                    storageUsage.used / storageUsage.limit > 0.9
                      ? 'bg-red-500'
                      : storageUsage.used / storageUsage.limit > 0.7
                        ? 'bg-amber-500'
                        : 'bg-purple-500'
                  }`}
                  style={{ width: `${Math.min((storageUsage.used / storageUsage.limit) * 100, 100)}%` }}
                />
              </div>
              {storageUsage.used / storageUsage.limit > 0.9 && (
                <p className="text-[11px] text-red-400">
                  {t('services.card.storageAlmostFull')}
                </p>
              )}
            </div>
          )}

          <p className={`text-[11px] ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
            {t('services.card.storageDescription')}
          </p>
        </div>
      ) : (
        /* Database-type services */
        <>
          <div className="space-y-2">
            <div className="flex items-center justify-between">
              <span className="text-[11px] font-medium text-zinc-500 uppercase tracking-wider">
                {t('services.card.connectionUrl')}
              </span>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setShowCreds(!showCreds)}
                  className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors"
                  title={showCreds ? t('services.card.hideCredentials') : t('services.card.showCredentials')}
                >
                  {showCreds ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
                <button
                  onClick={copyUrl}
                  className="p-1 text-zinc-500 hover:text-amber-400 transition-colors"
                  title={t('services.card.copyToClipboard')}
                >
                  {copied ? <Check size={14} className="text-emerald-400" /> : <Copy size={14} />}
                </button>
              </div>
            </div>
            <div
              className={`
                px-3 py-2 rounded-lg font-mono text-xs break-all
                ${isDark ? 'bg-zinc-900/50 border border-zinc-800' : 'bg-zinc-50 border border-zinc-200'}
                ${showCreds ? (isDark ? 'text-zinc-300' : 'text-zinc-700') : 'text-zinc-600'}
              `}
            >
              {showCreds
                ? (service.credentials?.url || "")
                : (service.credentials?.url || '').replace(
                    /\/\/.*@/,
                    '//****:****@',
                  )}
            </div>
          </div>

          {showCreds && (
            <div className="grid grid-cols-2 gap-2 mt-3">
              {[
                { label: t('services.card.host'), value: service.credentials.host },
                { label: t('services.card.port'), value: String(service.credentials.port) },
                { label: t('services.card.user'), value: service.credentials.username },
                { label: t('services.card.database'), value: service.credentials.database },
              ].map(({ label, value }) => (
                <div key={label}>
                  <span className="text-[10px] text-zinc-500 uppercase tracking-wider">
                    {label}
                  </span>
                  <p className={`text-xs font-mono ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                    {value}
                  </p>
                </div>
              ))}
            </div>
          )}

          {/* Usage bar for database services */}
          {storageUsage && storageUsage.limit > 0 && (
            <div className="space-y-1.5 mt-3">
              <div className="flex items-center justify-between">
                <span className="text-[11px] font-medium text-zinc-500 uppercase tracking-wider">
                  {t('services.card.diskUsage')}
                </span>
                <span className={`text-[11px] font-mono ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                  {storageUsage.used > 0 ? formatStorageSize(storageUsage.used) : '-'} / {storageUsage.limitStr}
                </span>
              </div>
              {storageUsage.used > 0 && (
                <div className={`w-full h-2 rounded-full overflow-hidden ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                  <div
                    className={`h-full rounded-full transition-all duration-500 ${
                      storageUsage.used / storageUsage.limit > 0.9
                        ? 'bg-red-500'
                        : storageUsage.used / storageUsage.limit > 0.7
                          ? 'bg-amber-500'
                          : 'bg-blue-500'
                    }`}
                    style={{ width: `${Math.min((storageUsage.used / storageUsage.limit) * 100, 100)}%` }}
                  />
                </div>
              )}
            </div>
          )}
        </>
      )}
    </GlassCard>
  );
}

import { useState } from 'react';
import { Database, Eye, EyeOff, Copy, Trash2, Check } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { useThemeStore } from '../../stores/theme.store';
import type { AppService, ServiceType } from '../../api/services';

interface ServiceCardProps {
  service: AppService;
  onDelete: (serviceId: string) => void;
}

const serviceConfig: Record<ServiceType, { label: string; color: string; icon: string }> = {
  postgres: { label: 'PostgreSQL', color: 'text-blue-400', icon: 'PG' },
  redis: { label: 'Redis', color: 'text-red-400', icon: 'RD' },
  mongodb: { label: 'MongoDB', color: 'text-emerald-400', icon: 'MG' },
  rabbitmq: { label: 'RabbitMQ', color: 'text-orange-400', icon: 'RQ' },
};

export function ServiceCard({ service, onDelete }: ServiceCardProps) {
  const [showCreds, setShowCreds] = useState(false);
  const [copied, setCopied] = useState(false);
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const config = serviceConfig[service.serviceType];

  const copyUrl = async () => {
    await navigator.clipboard.writeText(service.credentials.connectionUrl);
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
              {config.label}
            </h4>
            <p className="text-[11px] text-zinc-500">{service.dbName}</p>
          </div>
        </div>
        <button
          onClick={() => onDelete(service.id)}
          className="text-zinc-500 hover:text-red-400 transition-colors p-1"
          title="Remove service"
        >
          <Trash2 size={14} />
        </button>
      </div>

      {/* Connection URL */}
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <span className="text-[11px] font-medium text-zinc-500 uppercase tracking-wider">
            Connection URL
          </span>
          <div className="flex items-center gap-1">
            <button
              onClick={() => setShowCreds(!showCreds)}
              className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors"
              title={showCreds ? 'Hide credentials' : 'Show credentials'}
            >
              {showCreds ? <EyeOff size={14} /> : <Eye size={14} />}
            </button>
            <button
              onClick={copyUrl}
              className="p-1 text-zinc-500 hover:text-amber-400 transition-colors"
              title="Copy to clipboard"
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
            ? service.credentials.connectionUrl
            : service.credentials.connectionUrl.replace(
                /\/\/.*@/,
                '//****:****@',
              )}
        </div>
      </div>

      {/* Individual creds */}
      {showCreds && (
        <div className="grid grid-cols-2 gap-2 mt-3">
          {[
            { label: 'Host', value: service.credentials.host },
            { label: 'Port', value: String(service.credentials.port) },
            { label: 'User', value: service.credentials.username },
            { label: 'Database', value: service.credentials.database },
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
    </GlassCard>
  );
}

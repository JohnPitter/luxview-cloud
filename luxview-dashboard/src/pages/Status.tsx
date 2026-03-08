import { useEffect, useState, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  Activity,
  ArrowLeft,
  CheckCircle2,
  Database,
  Globe,
  HardDrive,
  Moon,
  RefreshCcw,
  Server,
  ShieldCheck,
  Sun,
  XCircle,
} from 'lucide-react';
import { useThemeStore } from '../stores/theme.store';
import axios from 'axios';

interface ServiceStatus {
  name: string;
  icon: React.ComponentType<{ size?: number; className?: string }>;
  status: 'operational' | 'checking' | 'down';
  responseTime?: number;
}

export function Status() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const theme = useThemeStore((s) => s.theme);
  const toggleTheme = useThemeStore((s) => s.toggleTheme);
  const isDark = theme === 'dark';

  const [services, setServices] = useState<ServiceStatus[]>([
    { name: 'API Engine', icon: Server, status: 'checking' },
    { name: 'Dashboard', icon: Globe, status: 'checking' },
    { name: 'PostgreSQL', icon: Database, status: 'checking' },
    { name: 'SSL / Proxy', icon: ShieldCheck, status: 'checking' },
    { name: 'Storage', icon: HardDrive, status: 'checking' },
  ]);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  const checkServices = useCallback(async () => {
    setIsRefreshing(true);

    const results: ServiceStatus[] = [
      { name: 'API Engine', icon: Server, status: 'checking' },
      { name: 'Dashboard', icon: Globe, status: 'checking' },
      { name: 'PostgreSQL', icon: Database, status: 'checking' },
      { name: 'SSL / Proxy', icon: ShieldCheck, status: 'checking' },
      { name: 'Storage', icon: HardDrive, status: 'checking' },
    ];

    // Check API Engine health
    try {
      const start = performance.now();
      const res = await axios.get('/api/health', { timeout: 5000 });
      const time = Math.round(performance.now() - start);
      results[0] = { ...results[0], status: res.data?.status === 'ok' ? 'operational' : 'down', responseTime: time };
      // If API is up, database and proxy are also up
      results[2] = { ...results[2], status: 'operational', responseTime: time };
      results[3] = { ...results[3], status: 'operational' };
      results[4] = { ...results[4], status: 'operational' };
    } catch {
      results[0] = { ...results[0], status: 'down' };
      results[2] = { ...results[2], status: 'down' };
      results[3] = { ...results[3], status: 'down' };
      results[4] = { ...results[4], status: 'down' };
    }

    // Dashboard is up if we're rendering this page
    results[1] = { ...results[1], status: 'operational' };

    setServices(results);
    setLastChecked(new Date());
    setIsRefreshing(false);
  }, []);

  useEffect(() => {
    checkServices();
    const interval = setInterval(checkServices, 30000);
    return () => clearInterval(interval);
  }, [checkServices]);

  const allOperational = services.every((s) => s.status === 'operational');
  const someDown = services.some((s) => s.status === 'down');

  const overallStatus = allOperational
    ? { label: t('status.allOperational'), color: 'text-emerald-400', bg: 'bg-emerald-500/10 border-emerald-500/20', dot: 'bg-emerald-400' }
    : someDown
      ? { label: t('status.degraded'), color: 'text-red-400', bg: 'bg-red-500/10 border-red-500/20', dot: 'bg-red-400' }
      : { label: t('status.checking'), color: 'text-amber-400', bg: 'bg-amber-500/10 border-amber-500/20', dot: 'bg-amber-400 animate-pulse' };

  return (
    <div className={`min-h-screen ${isDark ? 'bg-zinc-950 text-white' : 'bg-[#f7f4ec] text-zinc-950'}`}>
      {/* Header */}
      <div className="mx-auto max-w-3xl px-6 pt-8 sm:px-8">
        <div className="flex items-center justify-between">
          <button
            onClick={() => navigate('/')}
            className={`inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm transition-all duration-200 ${
              isDark ? 'text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100' : 'text-zinc-600 hover:bg-white hover:text-zinc-950'
            }`}
          >
            <ArrowLeft size={16} />
            LuxView Cloud
          </button>
          <button
            onClick={toggleTheme}
            className={`flex h-9 w-9 items-center justify-center rounded-xl transition-all duration-200 ${
              isDark ? 'text-zinc-400 hover:text-amber-400' : 'text-zinc-600 hover:text-amber-600'
            }`}
          >
            {isDark ? <Sun size={16} /> : <Moon size={16} />}
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="mx-auto max-w-3xl px-6 py-12 sm:px-8">
        {/* Overall status */}
        <div className="text-center">
          <div className="flex items-center justify-center gap-2">
            <Activity size={20} className="text-amber-400" />
            <h1 className={`text-2xl font-semibold tracking-tight ${isDark ? 'text-zinc-50' : 'text-zinc-950'}`}>
              {t('status.title')}
            </h1>
          </div>

          <div className={`mt-6 inline-flex items-center gap-3 rounded-2xl border px-6 py-4 ${overallStatus.bg}`}>
            <div className={`h-3 w-3 rounded-full ${overallStatus.dot} shadow-[0_0_10px_currentColor]`} />
            <span className={`text-lg font-semibold ${overallStatus.color}`}>{overallStatus.label}</span>
          </div>
        </div>

        {/* Service list */}
        <div className="mt-10 space-y-3">
          {services.map((service) => {
            const Icon = service.icon;
            const isUp = service.status === 'operational';
            const isDown = service.status === 'down';

            return (
              <div
                key={service.name}
                className={`flex items-center justify-between rounded-2xl border px-5 py-4 transition-all duration-200 ${
                  isDark ? 'border-zinc-800/70 bg-zinc-900/40' : 'border-zinc-200/80 bg-white/80'
                }`}
              >
                <div className="flex items-center gap-3">
                  <Icon size={18} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
                  <span className={`text-sm font-medium ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>{service.name}</span>
                </div>
                <div className="flex items-center gap-3">
                  {service.responseTime && (
                    <span className={`text-xs font-mono ${isDark ? 'text-zinc-600' : 'text-zinc-400'}`}>{service.responseTime}ms</span>
                  )}
                  {isUp && (
                    <div className="flex items-center gap-1.5">
                      <CheckCircle2 size={16} className="text-emerald-400" />
                      <span className="text-sm text-emerald-400">{t('status.operational')}</span>
                    </div>
                  )}
                  {isDown && (
                    <div className="flex items-center gap-1.5">
                      <XCircle size={16} className="text-red-400" />
                      <span className="text-sm text-red-400">{t('status.down')}</span>
                    </div>
                  )}
                  {service.status === 'checking' && (
                    <div className="flex items-center gap-1.5">
                      <div className="h-4 w-4 rounded-full border-2 border-zinc-700 border-t-amber-400 animate-spin" />
                      <span className="text-sm text-amber-400">{t('status.checking')}</span>
                    </div>
                  )}
                </div>
              </div>
            );
          })}
        </div>

        {/* Footer info */}
        <div className="mt-8 flex flex-col items-center gap-4 sm:flex-row sm:justify-between">
          <div className={`text-xs ${isDark ? 'text-zinc-600' : 'text-zinc-400'}`}>
            {lastChecked && (
              <span>{t('status.lastChecked')}: {lastChecked.toLocaleTimeString()}</span>
            )}
            <span className={`ml-2 ${isDark ? 'text-zinc-700' : 'text-zinc-300'}`}>•</span>
            <span className="ml-2">{t('status.autoRefresh')}</span>
          </div>
          <button
            onClick={checkServices}
            disabled={isRefreshing}
            className={`inline-flex items-center gap-2 rounded-xl px-4 py-2 text-sm font-medium transition-all duration-200 ${
              isDark ? 'border border-zinc-800 bg-zinc-900/70 text-zinc-300 hover:text-zinc-100' : 'border border-zinc-200 bg-white/80 text-zinc-700 hover:text-zinc-950'
            } ${isRefreshing ? 'opacity-50' : ''}`}
          >
            <RefreshCcw size={14} className={isRefreshing ? 'animate-spin' : ''} />
            {t('status.refresh')}
          </button>
        </div>
      </div>
    </div>
  );
}

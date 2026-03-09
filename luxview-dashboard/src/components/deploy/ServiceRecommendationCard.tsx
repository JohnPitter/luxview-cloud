import {
  Database,
  HardDrive,
  Radio,
  X,
  Check,
} from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useThemeStore } from '../../stores/theme.store';
import type { ServiceRecommendation } from '../../api/analyze';

export type MigrationMode = 'provision' | 'ignore';

interface ServiceRecommendationCardProps {
  recommendation: ServiceRecommendation;
  mode: MigrationMode;
  onModeChange: (mode: MigrationMode) => void;
}

const serviceIcons: Record<string, React.ReactNode> = {
  postgres: <Database size={18} className="text-blue-400" />,
  redis: <Database size={18} className="text-red-400" />,
  mongodb: <Database size={18} className="text-emerald-400" />,
  rabbitmq: <Radio size={18} className="text-orange-400" />,
  s3: <HardDrive size={18} className="text-purple-400" />,
};

export function ServiceRecommendationCard({
  recommendation,
  mode,
  onModeChange,
}: ServiceRecommendationCardProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const serviceKey = recommendation.recommendedService as string;
  const serviceName = t(`analyze.serviceNames.${serviceKey}`);

  const btnClass = (value: MigrationMode) =>
    `flex-1 px-3 py-2.5 text-xs rounded-lg border transition-all cursor-pointer text-center flex items-center justify-center gap-1.5 ${
      mode === value
        ? value === 'provision'
          ? 'border-emerald-500 bg-emerald-500/10 text-emerald-500 font-medium'
          : 'border-zinc-500 bg-zinc-500/10 text-zinc-400 font-medium'
        : isDark
          ? 'border-zinc-700 bg-zinc-800/50 text-zinc-400 hover:border-zinc-600'
          : 'border-zinc-200 bg-white text-zinc-600 hover:border-zinc-300'
    }`;

  return (
    <div
      className={`rounded-xl border p-4 space-y-3 ${
        isDark ? 'border-zinc-700/50 bg-zinc-800/30' : 'border-zinc-200 bg-zinc-50/50'
      }`}
    >
      {/* Header: Current -> Recommended */}
      <div className="flex items-center gap-3">
        {serviceIcons[serviceKey] ?? <Database size={18} className="text-zinc-400" />}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {recommendation.currentService}
            </span>
            <span className="text-xs text-zinc-500">&rarr;</span>
            <span className="text-sm font-semibold text-primary">{serviceName}</span>
          </div>
          <p className={`text-[11px] mt-0.5 ${isDark ? 'text-zinc-500' : 'text-zinc-500'}`}>
            {t('analyze.evidence')}: {recommendation.currentEvidence}
          </p>
        </div>
      </div>

      {/* Reason */}
      <p className={`text-xs leading-relaxed ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
        {recommendation.reason}
      </p>

      {/* Mode selector */}
      <div className="flex gap-2">
        <button
          type="button"
          onClick={() => onModeChange('provision')}
          className={btnClass('provision')}
        >
          <Check size={12} />
          {t('analyze.migrationMode.provision')}
        </button>
        <button
          type="button"
          onClick={() => onModeChange('ignore')}
          className={btnClass('ignore')}
        >
          <X size={12} />
          {t('analyze.migrationMode.skip')}
        </button>
      </div>
    </div>
  );
}

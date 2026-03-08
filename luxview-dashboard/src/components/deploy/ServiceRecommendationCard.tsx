import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Database,
  HardDrive,
  Radio,
  ChevronDown,
  ChevronUp,
  Zap,
  BookOpen,
  X,
} from 'lucide-react';
import { useThemeStore } from '../../stores/theme.store';
import type { ServiceRecommendation } from '../../api/analyze';

export type MigrationMode = 'auto' | 'manual' | 'ignore';

interface ServiceRecommendationCardProps {
  recommendation: ServiceRecommendation;
  mode: MigrationMode;
  onModeChange: (mode: MigrationMode) => void;
}

const serviceIcons: Record<string, React.ReactNode> = {
  postgres: <Database size={18} className="text-blue-400" />,
  redis: <Zap size={18} className="text-red-400" />,
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
  const [showSteps, setShowSteps] = useState(false);

  const serviceKey = recommendation.recommendedService as string;
  const serviceName = t(`analyze.serviceNames.${serviceKey}`);

  const radioClass = (value: MigrationMode) =>
    `flex-1 px-3 py-2.5 text-xs rounded-lg border transition-all cursor-pointer text-center ${
      mode === value
        ? 'border-amber-500 bg-amber-500/10 text-amber-500 font-medium'
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
      {/* Header: Current → Recommended */}
      <div className="flex items-center gap-3">
        {serviceIcons[serviceKey] ?? <Database size={18} className="text-zinc-400" />}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
              {recommendation.currentService}
            </span>
            <span className="text-xs text-zinc-500">→</span>
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
          onClick={() => onModeChange('auto')}
          className={radioClass('auto')}
        >
          <Zap size={12} className="inline mr-1" />
          {t('analyze.migrationMode.auto')}
          <span className="block text-[10px] opacity-60 mt-0.5">{t('analyze.autoDescription')}</span>
        </button>
        <button
          type="button"
          onClick={() => onModeChange('manual')}
          className={radioClass('manual')}
        >
          <BookOpen size={12} className="inline mr-1" />
          {t('analyze.migrationMode.manual')}
        </button>
        <button
          type="button"
          onClick={() => onModeChange('ignore')}
          className={radioClass('ignore')}
        >
          <X size={12} className="inline mr-1" />
          {t('analyze.migrationMode.ignore')}
        </button>
      </div>

      {/* Manual steps (collapsible, shown when manual is selected) */}
      {mode === 'manual' && recommendation.manualSteps.length > 0 && (
        <div>
          <button
            type="button"
            onClick={() => setShowSteps(!showSteps)}
            className={`flex items-center gap-1.5 text-xs font-medium transition-colors ${
              isDark ? 'text-zinc-400 hover:text-zinc-300' : 'text-zinc-600 hover:text-zinc-800'
            }`}
          >
            {showSteps ? <ChevronUp size={14} /> : <ChevronDown size={14} />}
            {t('analyze.manualSteps')}
          </button>
          {showSteps && (
            <ol
              className={`mt-2 ml-4 space-y-1.5 list-decimal text-xs leading-relaxed ${
                isDark ? 'text-zinc-400' : 'text-zinc-600'
              }`}
            >
              {recommendation.manualSteps.map((step, i) => (
                <li key={i}>{step}</li>
              ))}
            </ol>
          )}
        </div>
      )}
    </div>
  );
}

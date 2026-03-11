import { useState, useMemo, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Sparkles,
  Stethoscope,
  AlertCircle,
  AlertTriangle,
  Info,
  Loader2,
  Pencil,
  Check,
  Server,
  Database,
  GitBranch,
  FileSearch,
  FileCode,
  CheckCircle,
} from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { useThemeStore } from '../../stores/theme.store';
import type { AnalysisResult, Suggestion } from '../../api/analyze';
import { ServiceRecommendationCard, type MigrationMode } from './ServiceRecommendationCard';

interface DeployAnalysisProps {
  result: AnalysisResult;
  loading?: boolean;
  mode: 'first-deploy' | 'failure';
  onApprove: (dockerfile: string, envVars: Record<string, string>, services: string[]) => void;
  onSkip: () => void;
}

const severityIcon: Record<Suggestion['type'], React.ReactNode> = {
  error: <AlertCircle size={16} className="text-red-400 shrink-0" />,
  warning: <AlertTriangle size={16} className="text-amber-400 shrink-0" />,
  info: <Info size={16} className="text-blue-400 shrink-0" />,
};

const severityBorder: Record<Suggestion['type'], string> = {
  error: 'border-red-500/30 bg-red-500/5',
  warning: 'border-amber-500/30 bg-amber-500/5',
  info: 'border-blue-500/30 bg-blue-500/5',
};

function AnalysisLoadingAnimation({ isDark, isFailure }: { isDark: boolean; isFailure: boolean }) {
  const { t } = useTranslation();
  const [activeStep, setActiveStep] = useState(0);

  const steps = isFailure
    ? [
        { icon: <GitBranch size={16} />, label: t('analyze.loading.cloningRepo') },
        { icon: <FileSearch size={16} />, label: t('analyze.loading.readingLogs') },
        { icon: <Sparkles size={16} />, label: t('analyze.loading.diagnosing') },
        { icon: <FileCode size={16} />, label: t('analyze.loading.generatingFix') },
      ]
    : [
        { icon: <GitBranch size={16} />, label: t('analyze.loading.cloningRepo') },
        { icon: <FileSearch size={16} />, label: t('analyze.loading.scanningFiles') },
        { icon: <Database size={16} />, label: t('analyze.loading.detectingServices') },
        { icon: <Sparkles size={16} />, label: t('analyze.loading.generatingDockerfile') },
        { icon: <CheckCircle size={16} />, label: t('analyze.loading.preparingResults') },
      ];

  useEffect(() => {
    const interval = setInterval(() => {
      setActiveStep((prev) => (prev < steps.length - 1 ? prev + 1 : prev));
    }, 8000);
    return () => clearInterval(interval);
  }, [steps.length]);

  return (
    <div className="flex flex-col items-center gap-6 py-8">
      <div className="relative">
        <div className="w-16 h-16 rounded-2xl bg-primary/10 flex items-center justify-center">
          <Sparkles size={28} className="text-primary animate-pulse" />
        </div>
      </div>
      <div className="text-center space-y-1">
        <h3 className={`text-base font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {isFailure ? t('analyze.diagnosingFailure') : t('analyze.analyzingRepo')}
        </h3>
        <p className={`text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-500'}`}>
          {t('analyze.loading.patience')}
        </p>
      </div>
      <div className="w-full max-w-xs space-y-2">
        {steps.map((step, i) => (
          <div
            key={i}
            className={`flex items-center gap-3 px-3 py-2 rounded-lg transition-all duration-500 ${
              i < activeStep
                ? 'opacity-60'
                : i === activeStep
                  ? isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'
                  : 'opacity-30'
            }`}
          >
            <span
              className={`shrink-0 ${
                i < activeStep
                  ? 'text-emerald-400'
                  : i === activeStep
                    ? 'text-primary animate-pulse'
                    : isDark ? 'text-zinc-600' : 'text-zinc-400'
              }`}
            >
              {i < activeStep ? <CheckCircle size={16} /> : step.icon}
            </span>
            <span
              className={`text-sm ${
                i === activeStep
                  ? isDark ? 'text-zinc-200 font-medium' : 'text-zinc-800 font-medium'
                  : isDark ? 'text-zinc-400' : 'text-zinc-600'
              }`}
            >
              {step.label}
            </span>
            {i === activeStep && (
              <Loader2 size={14} className="ml-auto text-primary animate-spin" />
            )}
          </div>
        ))}
      </div>
    </div>
  );
}

export function DeployAnalysis({
  result,
  loading = false,
  mode,
  onApprove,
  onSkip,
}: DeployAnalysisProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [dockerfile, setDockerfile] = useState(result.dockerfile);
  const [editing, setEditing] = useState(false);

  // Sync dockerfile state when result prop changes (e.g., loading -> real result)
  useEffect(() => {
    if (result.dockerfile && result.dockerfile !== dockerfile) {
      setDockerfile(result.dockerfile);
    }
  }, [result.dockerfile]); // eslint-disable-line react-hooks/exhaustive-deps
  const [envValues, setEnvValues] = useState<Record<string, string>>(() => {
    const initial: Record<string, string> = {};
    for (const hint of result.envHints) {
      initial[hint.key] = '';
    }
    return initial;
  });

  const [serviceModes, setServiceModes] = useState<Record<string, MigrationMode>>(() => {
    const initial: Record<string, MigrationMode> = {};
    for (const rec of result.serviceRecommendations ?? []) {
      initial[rec.recommendedService] = 'provision';
    }
    return initial;
  });

  const hasRecommendations = (result.serviceRecommendations ?? []).length > 0;

  const isFailure = mode === 'failure';

  const title = isFailure ? t('analyze.failureTitle') : t('analyze.title');

  const sortedSuggestions = useMemo(
    () =>
      [...result.suggestions].sort((a, b) => {
        const order = { error: 0, warning: 1, info: 2 };
        return order[a.type] - order[b.type];
      }),
    [result.suggestions],
  );

  if (loading) {
    return (
      <GlassCard padding="lg" className="animate-fade-in">
        <AnalysisLoadingAnimation isDark={isDark} isFailure={isFailure} />
      </GlassCard>
    );
  }

  return (
    <GlassCard padding="lg" className="animate-fade-in space-y-6">
      {/* Header */}
      <div className="flex items-center gap-3">
        {isFailure ? (
          <span className="flex items-center justify-center w-10 h-10 rounded-xl bg-red-500/10">
            <Stethoscope size={20} className="text-red-400" />
          </span>
        ) : (
          <span className="flex items-center justify-center w-10 h-10 rounded-xl bg-primary/10">
            <Sparkles size={20} className="text-primary" />
          </span>
        )}
        <div>
          <h3
            className={`text-lg font-semibold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            {title}
          </h3>
          {result.stack && (
            <span className="inline-block mt-1 text-[11px] font-medium px-2 py-0.5 rounded-md bg-primary/10 text-primary uppercase tracking-wider">
              {result.stack}
            </span>
          )}
        </div>
      </div>

      {/* Diagnosis (failure mode only) */}
      {isFailure && result.diagnosis && (
        <div className="rounded-xl border border-red-500/30 bg-red-500/5 p-4">
          <h4
            className={`text-sm font-semibold mb-2 ${
              isDark ? 'text-red-300' : 'text-red-700'
            }`}
          >
            {t('analyze.diagnosis')}
          </h4>
          <p
            className={`text-sm leading-relaxed ${
              isDark ? 'text-red-200/80' : 'text-red-800/80'
            }`}
          >
            {result.diagnosis}
          </p>
        </div>
      )}

      {/* Suggestions */}
      {sortedSuggestions.length > 0 && (
        <div className="space-y-2">
          <h4
            className={`text-sm font-semibold ${
              isDark ? 'text-zinc-300' : 'text-zinc-700'
            }`}
          >
            {t('analyze.suggestions')}
          </h4>
          <div className="space-y-2">
            {sortedSuggestions.map((s, i) => (
              <div
                key={i}
                className={`flex items-start gap-3 rounded-lg border p-3 ${severityBorder[s.type]}`}
              >
                {severityIcon[s.type]}
                <span
                  className={`text-sm leading-relaxed ${
                    isDark ? 'text-zinc-300' : 'text-zinc-700'
                  }`}
                >
                  {s.message}
                </span>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Dockerfile */}
      {dockerfile && (
        <div className="space-y-2">
          <div className="flex items-center justify-between">
            <h4
              className={`text-sm font-semibold ${
                isDark ? 'text-zinc-300' : 'text-zinc-700'
              }`}
            >
              {t('analyze.dockerfile')}
            </h4>
            <button
              type="button"
              onClick={() => setEditing(!editing)}
              className="flex items-center gap-1.5 text-xs text-muted-foreground hover:text-primary transition-colors"
            >
              {editing ? (
                <>
                  <Check size={12} />
                  {t('analyze.doneEditing')}
                </>
              ) : (
                <>
                  <Pencil size={12} />
                  {t('analyze.edit')}
                </>
              )}
            </button>
          </div>
          {editing ? (
            <textarea
              value={dockerfile}
              onChange={(e) => setDockerfile(e.target.value)}
              className={`w-full font-mono text-xs rounded-xl border p-4 min-h-[200px] resize-y focus:outline-none focus:ring-2 focus:ring-primary/50 ${
                isDark
                  ? 'bg-zinc-950 border-zinc-700 text-zinc-300'
                  : 'bg-zinc-50 border-zinc-200 text-zinc-800'
              }`}
              spellCheck={false}
            />
          ) : (
            <pre
              className={`font-mono text-xs rounded-xl border p-4 overflow-x-auto ${
                isDark
                  ? 'bg-zinc-950 border-zinc-700 text-zinc-300'
                  : 'bg-zinc-50 border-zinc-200 text-zinc-800'
              }`}
            >
              {dockerfile}
            </pre>
          )}
        </div>
      )}

      {/* Port */}
      {result.port > 0 && (
        <div className="flex items-center gap-2">
          <Server size={14} className="text-zinc-500" />
          <span
            className={`text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}
          >
            {t('analyze.port')}:
          </span>
          <span
            className={`text-sm font-mono font-semibold ${
              isDark ? 'text-zinc-200' : 'text-zinc-800'
            }`}
          >
            {result.port}
          </span>
        </div>
      )}

      {/* Env Hints */}
      {result.envHints.length > 0 && (
        <div className="space-y-3">
          <h4
            className={`text-sm font-semibold ${
              isDark ? 'text-zinc-300' : 'text-zinc-700'
            }`}
          >
            {t('analyze.envVars')}
          </h4>
          <div className="space-y-2">
            {result.envHints.map((hint) => (
              <div
                key={hint.key}
                className={`flex flex-col sm:flex-row sm:items-center gap-2 rounded-lg border p-3 ${
                  isDark
                    ? 'border-zinc-700/50 bg-zinc-800/30'
                    : 'border-zinc-200 bg-zinc-50/50'
                }`}
              >
                <div className="flex items-center gap-2 sm:w-1/3">
                  <span className="font-mono text-xs font-semibold text-primary">
                    {hint.key}
                  </span>
                  {hint.required && (
                    <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-red-500/10 text-red-400 border border-red-500/20">
                      required
                    </span>
                  )}
                </div>
                <p
                  className={`text-[12px] sm:w-1/3 ${
                    isDark ? 'text-zinc-500' : 'text-zinc-500'
                  }`}
                >
                  {hint.description}
                </p>
                <input
                  type="text"
                  value={envValues[hint.key] ?? ''}
                  onChange={(e) =>
                    setEnvValues((prev) => ({
                      ...prev,
                      [hint.key]: e.target.value,
                    }))
                  }
                  placeholder={hint.key}
                  className={`sm:w-1/3 font-mono text-xs rounded-lg border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary/50 ${
                    isDark
                      ? 'bg-zinc-900 border-zinc-700 text-zinc-300 placeholder:text-zinc-600'
                      : 'bg-white border-zinc-200 text-zinc-800 placeholder:text-zinc-400'
                  }`}
                />
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Service Recommendations */}
      {!isFailure && hasRecommendations && (
        <div className="space-y-3">
          <div className="flex items-center gap-2">
            <Database size={16} className="text-primary" />
            <h4
              className={`text-sm font-semibold ${
                isDark ? 'text-zinc-300' : 'text-zinc-700'
              }`}
            >
              {t('analyze.serviceRecommendations')}
            </h4>
          </div>
          <p className={`text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-500'}`}>
            {t('analyze.serviceRecommendationsDescription')}
          </p>
          <div className="space-y-3">
            {(result.serviceRecommendations ?? []).map((rec) => (
              <ServiceRecommendationCard
                key={rec.recommendedService}
                recommendation={rec}
                mode={serviceModes[rec.recommendedService] ?? 'ignore'}
                onModeChange={(m) =>
                  setServiceModes((prev) => ({ ...prev, [rec.recommendedService]: m }))
                }
              />
            ))}
          </div>
        </div>
      )}

      {/* Actions */}
      <div className="flex items-center justify-end gap-3 pt-2">
        <button
          type="button"
          onClick={onSkip}
          className="text-muted-foreground hover:bg-muted rounded-lg px-4 py-2 transition-colors text-sm disabled:opacity-50"
        >
          {isFailure ? t('analyze.dismiss') : t('analyze.skipAnalysis')}
        </button>
        <button
          type="button"
          onClick={() => {
            const selectedServices = Object.entries(serviceModes)
              .filter(([, m]) => m === 'provision')
              .map(([service]) => service);
            onApprove(dockerfile, envValues, selectedServices);
          }}
          className="bg-primary text-white rounded-lg px-4 py-2 hover:brightness-110 active:scale-[0.98] transition-all text-sm font-medium disabled:opacity-70 disabled:cursor-not-allowed flex items-center gap-2"
        >
          {isFailure
            ? t('analyze.applyAndRedeploy')
            : t('analyze.approveAndDeploy')}
        </button>
      </div>
    </GlassCard>
  );
}

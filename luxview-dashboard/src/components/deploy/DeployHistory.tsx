import { useTranslation } from 'react-i18next';
import { GitCommit, Clock, RotateCcw, Check, X, Loader2, Bot, Cog } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { formatRelativeTime, formatDuration } from '../../lib/format';
import type { Deployment, DeployStatus } from '../../api/deployments';

interface DeployHistoryProps {
  deployments: Deployment[];
  onRollback: (deployId: string) => void;
  onViewLog: (deployId: string) => void;
}

const statusIcons: Record<DeployStatus, React.ReactNode> = {
  live: <Check size={14} className="text-emerald-400" />,
  failed: <X size={14} className="text-red-400" />,
  building: <Loader2 size={14} className="text-amber-400 animate-spin" />,
  deploying: <Loader2 size={14} className="text-blue-400 animate-spin" />,
  pending: <Clock size={14} className="text-zinc-400" />,
  rolled_back: <RotateCcw size={14} className="text-violet-400" />,
};

const statusColors: Record<DeployStatus, string> = {
  live: 'border-emerald-500/30 bg-emerald-500/5',
  failed: 'border-red-500/30 bg-red-500/5',
  building: 'border-amber-500/30 bg-amber-500/5',
  deploying: 'border-blue-500/30 bg-blue-500/5',
  pending: 'border-zinc-500/30 bg-zinc-500/5',
  rolled_back: 'border-violet-500/30 bg-violet-500/5',
};

export function DeployHistory({ deployments, onRollback, onViewLog }: DeployHistoryProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  if (deployments.length === 0) {
    return (
      <div className="text-center py-12 text-zinc-500 text-sm">{t('deploy.history.noDeployments')}</div>
    );
  }

  return (
    <div className="space-y-3">
      {deployments.map((deploy, i) => (
        <GlassCard
          key={deploy.id}
          padding="sm"
          className={`animate-fade-in border-l-2 ${statusColors[deploy.status]}`}
          hover
          onClick={() => onViewLog(deploy.id)}
        >
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <span className="flex items-center justify-center w-7 h-7 rounded-lg bg-zinc-800/50">
                {statusIcons[deploy.status]}
              </span>
              <div>
                <div className="flex items-center gap-2">
                  <GitCommit size={12} className="text-zinc-500" />
                  <span className="text-xs font-mono text-zinc-400">
                    {deploy.commitSha.slice(0, 7)}
                  </span>
                  {deploy.source === 'ai' ? (
                    <span className="inline-flex items-center gap-1 text-[10px] font-medium px-1.5 py-0.5 rounded bg-purple-500/10 text-purple-400 border border-purple-500/20">
                      <Bot size={10} />
                      AI
                    </span>
                  ) : (
                    <span className="inline-flex items-center gap-1 text-[10px] font-medium px-1.5 py-0.5 rounded bg-zinc-500/10 text-zinc-400 border border-zinc-500/20">
                      <Cog size={10} />
                      Auto
                    </span>
                  )}
                  {i === 0 && deploy.status === 'live' && (
                    <span className="text-[10px] font-bold px-1.5 py-0.5 rounded bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
                      {t('deploy.history.current')}
                    </span>
                  )}
                </div>
                <p
                  className={`text-sm mt-0.5 truncate max-w-md ${
                    isDark ? 'text-zinc-200' : 'text-zinc-800'
                  }`}
                >
                  {deploy.commitMessage || t('deploy.history.noCommitMessage')}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-4">
              <div className="text-right">
                <div className="text-[11px] text-zinc-500">
                  {formatRelativeTime(deploy.createdAt)}
                </div>
                {deploy.durationMs > 0 && (
                  <div className="text-[11px] text-zinc-600">
                    {formatDuration(deploy.durationMs)}
                  </div>
                )}
              </div>
              {deploy.status === 'live' && i > 0 && (
                <PillButton
                  variant="ghost"
                  size="sm"
                  onClick={(e) => {
                    e.stopPropagation();
                    onRollback(deploy.id);
                  }}
                  icon={<RotateCcw size={12} />}
                >
                  {t('deploy.history.rollback')}
                </PillButton>
              )}
            </div>
          </div>
        </GlassCard>
      ))}
    </div>
  );
}

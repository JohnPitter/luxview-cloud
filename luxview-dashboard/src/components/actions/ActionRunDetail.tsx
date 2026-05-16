import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft,
  Check,
  X,
  Loader2,
  Clock,
  ChevronDown,
  ChevronRight,
  Package,
  GitBranch,
  RefreshCw,
} from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { formatRelativeTime } from '../../lib/format';
import { actionsApi, type ActionRunDetail as Detail, type ActionStatus, type ActionArtifact } from '../../api/actions';

interface ActionRunDetailProps {
  runId: string;
  onBack: () => void;
}

const statusIcon: Record<ActionStatus, React.ReactNode> = {
  queued: <Clock size={13} className="text-zinc-400" />,
  running: <Loader2 size={13} className="text-blue-400 animate-spin" />,
  success: <Check size={13} className="text-emerald-400" />,
  failed: <X size={13} className="text-red-400" />,
  cancelled: <X size={13} className="text-zinc-400" />,
  skipped: <ChevronRight size={13} className="text-zinc-400" />,
};

const statusLabel: Record<ActionStatus, string> = {
  queued: 'queued',
  running: 'running',
  success: 'success',
  failed: 'failed',
  cancelled: 'cancelled',
  skipped: 'skipped',
};

function formatDurationMs(start?: string, end?: string): string {
  if (!start) return '—';
  const s = new Date(start).getTime();
  const e = end ? new Date(end).getTime() : Date.now();
  const diff = Math.max(0, e - s);
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s`;
  return `${Math.floor(secs / 60)}m ${secs % 60}s`;
}

function StepRow({ step, isDark }: { step: Detail['steps'][number]; isDark: boolean }) {
  const [open, setOpen] = useState(step.status === 'failed');

  return (
    <div className={`border rounded-lg overflow-hidden ${isDark ? 'border-white/10' : 'border-black/10'}`}>
      <button
        className={`w-full flex items-center gap-3 px-3 py-2 text-left transition-colors ${
          isDark ? 'hover:bg-white/5' : 'hover:bg-black/5'
        }`}
        onClick={() => setOpen((o) => !o)}
      >
        <span className="flex-shrink-0">{statusIcon[step.status]}</span>
        <span className={`flex-1 text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'} truncate`}>
          {step.name || (step.kind === 'uses' ? step.uses : step.command?.split('\n')[0])}
        </span>
        <span className="text-xs text-zinc-500 flex-shrink-0">
          {formatDurationMs(step.startedAt, step.finishedAt)}
        </span>
        {open ? (
          <ChevronDown size={13} className="text-zinc-500 flex-shrink-0" />
        ) : (
          <ChevronRight size={13} className="text-zinc-500 flex-shrink-0" />
        )}
      </button>

      {open && (
        <div className={`px-3 pb-3 ${isDark ? 'bg-black/20' : 'bg-black/5'}`}>
          {step.log ? (
            <pre
              className={`text-xs font-mono whitespace-pre-wrap break-words p-3 rounded-lg mt-2 max-h-64 overflow-y-auto ${
                isDark ? 'bg-black/40 text-zinc-300' : 'bg-white/60 text-zinc-700'
              }`}
            >
              {step.log}
            </pre>
          ) : (
            <p className="text-xs text-zinc-500 mt-2 italic">No output</p>
          )}
          {step.exitCode !== 0 && (
            <p className="text-xs text-red-400 mt-1">Exit code: {step.exitCode}</p>
          )}
        </div>
      )}
    </div>
  );
}

export function ActionRunDetail({ runId, onBack }: ActionRunDetailProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [detail, setDetail] = useState<Detail | null>(null);
  const [artifacts, setArtifacts] = useState<ActionArtifact[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchDetail = useCallback(async () => {
    try {
      const d = await actionsApi.getRun(runId);
      setDetail(d);
      if (d.run.status === 'success' || d.run.status === 'failed') {
        const arts = await actionsApi.listArtifacts(runId);
        setArtifacts(arts);
      }
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, [runId]);

  useEffect(() => {
    fetchDetail();
  }, [fetchDetail]);

  // Poll while running
  useEffect(() => {
    if (!detail) return;
    const active = detail.run.status === 'queued' || detail.run.status === 'running';
    if (!active) return;
    const interval = setInterval(fetchDetail, 3000);
    return () => clearInterval(interval);
  }, [detail, fetchDetail]);

  if (loading) {
    return (
      <div className="flex justify-center py-16">
        <Loader2 className="animate-spin text-zinc-400" size={24} />
      </div>
    );
  }

  if (!detail) {
    return (
      <div className="text-center py-12 text-zinc-500 text-sm">{t('actions.runNotFound')}</div>
    );
  }

  const { run, jobs, steps } = detail;

  return (
    <div className="space-y-4">
      {/* Back + header */}
      <div className="flex items-center gap-3">
        <button
          onClick={onBack}
          className="p-1.5 rounded-lg text-zinc-500 hover:text-zinc-300 transition-colors"
        >
          <ArrowLeft size={16} />
        </button>
        <div className="flex-1 min-w-0">
          <p className={`font-semibold text-sm truncate ${isDark ? 'text-white' : 'text-zinc-900'}`}>
            {run.workflow || run.workflowPath}
          </p>
          <div className="flex items-center gap-3 mt-0.5">
            <span className="flex items-center gap-1 text-xs text-zinc-500">
              <GitBranch size={11} />
              {run.branch}
            </span>
            {run.commitSha && (
              <span className="text-xs text-zinc-500 font-mono">{run.commitSha.slice(0, 7)}</span>
            )}
            <span className="text-xs text-zinc-500 capitalize">{run.trigger}</span>
          </div>
        </div>
        <div className="flex items-center gap-2 flex-shrink-0">
          <span className="flex items-center gap-1.5">
            {statusIcon[run.status]}
            <span className={`text-sm font-medium capitalize ${
              run.status === 'success' ? 'text-emerald-400' :
              run.status === 'failed' ? 'text-red-400' :
              run.status === 'running' ? 'text-blue-400' : 'text-zinc-400'
            }`}>
              {statusLabel[run.status]}
            </span>
          </span>
          <PillButton variant="ghost" size="sm" icon={<RefreshCw size={13} />} onClick={fetchDetail}>
            {t('common.refresh')}
          </PillButton>
        </div>
      </div>

      {/* Run meta */}
      <GlassCard padding="sm">
        <div className="flex gap-6 text-xs text-zinc-500">
          <span>{t('actions.started')}: {run.startedAt ? formatRelativeTime(run.startedAt) : '—'}</span>
          <span>{t('actions.duration')}: {formatDurationMs(run.startedAt, run.finishedAt)}</span>
          <span>{t('actions.created')}: {formatRelativeTime(run.createdAt)}</span>
        </div>
      </GlassCard>

      {/* Jobs + Steps */}
      {jobs.map((job) => {
        const jobSteps = steps.filter((s) => s.jobId === job.id);
        return (
          <GlassCard key={job.id} padding="md">
            <div className="flex items-center gap-2 mb-3">
              {statusIcon[job.status]}
              <span className={`font-medium text-sm ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                {job.name}
              </span>
              <span className="text-xs text-zinc-500 ml-auto font-mono">{job.image}</span>
              <span className="text-xs text-zinc-500">
                {formatDurationMs(job.startedAt, job.finishedAt)}
              </span>
            </div>
            <div className="space-y-1.5">
              {jobSteps.length === 0 ? (
                <p className="text-xs text-zinc-500 italic">{t('actions.noSteps')}</p>
              ) : (
                jobSteps.map((step) => <StepRow key={step.id} step={step} isDark={isDark} />)
              )}
            </div>
          </GlassCard>
        );
      })}

      {/* Artifacts */}
      {artifacts.length > 0 && (
        <GlassCard padding="md">
          <div className="flex items-center gap-2 mb-3">
            <Package size={14} className="text-violet-400" />
            <span className={`font-medium text-sm ${isDark ? 'text-white' : 'text-zinc-900'}`}>
              {t('actions.artifacts')}
            </span>
          </div>
          <div className="space-y-1.5">
            {artifacts.map((art) => (
              <div key={art.id} className="flex items-center justify-between text-sm">
                <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>{art.name}</span>
                <span className="text-xs text-zinc-500">
                  {(art.sizeBytes / 1024).toFixed(1)} KB
                </span>
              </div>
            ))}
          </div>
        </GlassCard>
      )}
    </div>
  );
}

import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Play,
  Check,
  X,
  Loader2,
  Clock,
  ChevronRight,
  RefreshCw,
  GitBranch,
  Zap,
  Key,
  Plus,
  Trash2,
  Eye,
  EyeOff,
  FileCode2,
} from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { formatRelativeTime } from '../../lib/format';
import { actionsApi, type ActionRun, type ActionStatus, type ActionSecret, type WorkflowSummary } from '../../api/actions';
import { ActionRunDetail } from './ActionRunDetail';

interface ActionRunListProps {
  appId: string;
}

const statusIcon: Record<ActionStatus, React.ReactNode> = {
  queued: <Clock size={14} className="text-zinc-400" />,
  running: <Loader2 size={14} className="text-blue-400 animate-spin" />,
  success: <Check size={14} className="text-emerald-400" />,
  failed: <X size={14} className="text-red-400" />,
  cancelled: <X size={14} className="text-zinc-400" />,
  skipped: <ChevronRight size={14} className="text-zinc-400" />,
};

const statusBorder: Record<ActionStatus, string> = {
  queued: 'border-zinc-500/30 bg-zinc-500/5',
  running: 'border-blue-500/30 bg-blue-500/5',
  success: 'border-emerald-500/30 bg-emerald-500/5',
  failed: 'border-red-500/30 bg-red-500/5',
  cancelled: 'border-zinc-500/30 bg-zinc-500/5',
  skipped: 'border-zinc-500/30 bg-zinc-500/5',
};

type PanelTab = 'workflows' | 'runs' | 'secrets';

export function ActionRunList({ appId }: ActionRunListProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [panel, setPanel] = useState<PanelTab>('workflows');
  const [runs, setRuns] = useState<ActionRun[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [triggering, setTriggering] = useState<string | null>(null); // stores workflow path being triggered
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);

  const [workflows, setWorkflows] = useState<WorkflowSummary[]>([]);
  const [workflowsLoading, setWorkflowsLoading] = useState(false);

  const [secrets, setSecrets] = useState<ActionSecret[]>([]);
  const [secretsLoading, setSecretsLoading] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newValue, setNewValue] = useState('');
  const [showValues, setShowValues] = useState(false);
  const [savingSecret, setSavingSecret] = useState(false);
  const [deletingKey, setDeletingKey] = useState<string | null>(null);

  const fetchWorkflows = useCallback(async () => {
    setWorkflowsLoading(true);
    try {
      const result = await actionsApi.listWorkflows(appId);
      setWorkflows(result);
    } catch {
      // silent
    } finally {
      setWorkflowsLoading(false);
    }
  }, [appId]);

  const fetchRuns = useCallback(async () => {
    try {
      const result = await actionsApi.listRuns(appId);
      setRuns(result.runs);
      setTotal(result.total);
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, [appId]);

  useEffect(() => {
    fetchWorkflows();
    fetchRuns();
  }, [fetchWorkflows, fetchRuns]);

  // Poll while any run is active
  useEffect(() => {
    const hasActive = runs.some((r) => r.status === 'queued' || r.status === 'running');
    if (!hasActive) return;
    const interval = setInterval(fetchRuns, 4000);
    return () => clearInterval(interval);
  }, [runs, fetchRuns]);

  const fetchSecrets = useCallback(async () => {
    setSecretsLoading(true);
    try {
      const result = await actionsApi.listSecrets(appId);
      setSecrets(result);
    } catch {
      // silent
    } finally {
      setSecretsLoading(false);
    }
  }, [appId]);

  useEffect(() => {
    if (panel === 'secrets') fetchSecrets();
  }, [panel, fetchSecrets]);

  async function handleTrigger(workflowPath?: string) {
    const key = workflowPath ?? '';
    setTriggering(key);
    try {
      const run = await actionsApi.triggerRun(appId, workflowPath);
      setRuns((prev) => [run, ...prev]);
      setTotal((tVal) => tVal + 1);
      setPanel('runs');
      addNotification({ type: 'success', title: t('actions.triggered') });
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('actions.triggerFailed') });
    } finally {
      setTriggering(null);
    }
  }

  async function handleAddSecret() {
    if (!newKey || !newValue) return;
    setSavingSecret(true);
    try {
      const secret = await actionsApi.upsertSecret(appId, newKey, newValue);
      setSecrets((prev) => {
        const idx = prev.findIndex((s) => s.key === secret.key);
        if (idx >= 0) {
          const next = [...prev];
          next[idx] = secret;
          return next;
        }
        return [...prev, secret].sort((a, b) => a.key.localeCompare(b.key));
      });
      setNewKey('');
      setNewValue('');
      addNotification({ type: 'success', title: t('actions.secrets.saved') });
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('actions.secrets.saveFailed') });
    } finally {
      setSavingSecret(false);
    }
  }

  async function handleDeleteSecret(key: string) {
    setDeletingKey(key);
    try {
      await actionsApi.deleteSecret(appId, key);
      setSecrets((prev) => prev.filter((s) => s.key !== key));
      addNotification({ type: 'success', title: t('actions.secrets.deleted') });
    } catch {
      addNotification({ type: 'error', title: t('actions.secrets.deleteFailed') });
    } finally {
      setDeletingKey(null);
    }
  }

  if (selectedRunId) {
    return (
      <ActionRunDetail
        runId={selectedRunId}
        onBack={() => setSelectedRunId(null)}
      />
    );
  }

  const tabBase = 'px-4 py-2 text-sm font-medium rounded-lg transition-colors';
  const tabActive = isDark
    ? 'bg-white/10 text-white'
    : 'bg-black/10 text-zinc-900';
  const tabInactive = 'text-zinc-500 hover:text-zinc-300';

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex gap-2">
          <button
            className={`${tabBase} ${panel === 'workflows' ? tabActive : tabInactive}`}
            onClick={() => setPanel('workflows')}
          >
            <span className="flex items-center gap-2">
              <FileCode2 size={14} />
              {t('actions.workflows')}
              {workflows.length > 0 && (
                <span className="text-xs bg-white/10 px-1.5 py-0.5 rounded-full">{workflows.length}</span>
              )}
            </span>
          </button>
          <button
            className={`${tabBase} ${panel === 'runs' ? tabActive : tabInactive}`}
            onClick={() => setPanel('runs')}
          >
            <span className="flex items-center gap-2">
              <Zap size={14} />
              {t('actions.runs')}
              {total > 0 && (
                <span className="text-xs bg-white/10 px-1.5 py-0.5 rounded-full">{total}</span>
              )}
            </span>
          </button>
          <button
            className={`${tabBase} ${panel === 'secrets' ? tabActive : tabInactive}`}
            onClick={() => setPanel('secrets')}
          >
            <span className="flex items-center gap-2">
              <Key size={14} />
              {t('actions.secrets.title')}
            </span>
          </button>
        </div>

        {panel === 'runs' && (
          <div className="flex gap-2">
            <PillButton
              variant="ghost"
              size="sm"
              icon={<RefreshCw size={13} />}
              onClick={fetchRuns}
            >
              {t('common.refresh')}
            </PillButton>
          </div>
        )}
      </div>

      {/* Workflows list */}
      {panel === 'workflows' && (
        <>
          {workflowsLoading ? (
            <div className="flex justify-center py-12">
              <Loader2 className="animate-spin text-zinc-400" size={24} />
            </div>
          ) : workflows.length === 0 ? (
            <div className="text-center py-12 text-zinc-500 text-sm">{t('actions.noWorkflows')}</div>
          ) : (
            <div className="space-y-2">
              {workflows.map((wf) => (
                <GlassCard key={wf.path} padding="sm" className="flex items-center justify-between">
                  <div>
                    <p className={`text-sm font-medium ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                      {wf.name || wf.path}
                    </p>
                    <p className="text-xs text-zinc-500 font-mono mt-0.5">{wf.path}</p>
                  </div>
                  <PillButton
                    variant="primary"
                    size="sm"
                    icon={triggering === wf.path ? <Loader2 size={13} className="animate-spin" /> : <Play size={13} />}
                    onClick={() => handleTrigger(wf.path)}
                    disabled={triggering !== null}
                  >
                    {t('actions.triggerRun')}
                  </PillButton>
                </GlassCard>
              ))}
            </div>
          )}
        </>
      )}

      {/* Runs list */}
      {panel === 'runs' && (
        <>
          {loading ? (
            <div className="flex justify-center py-12">
              <Loader2 className="animate-spin text-zinc-400" size={24} />
            </div>
          ) : runs.length === 0 ? (
            <div className="text-center py-12 text-zinc-500 text-sm">{t('actions.noRuns')}</div>
          ) : (
            <div className="space-y-2">
              {runs.map((run) => (
                <GlassCard
                  key={run.id}
                  padding="sm"
                  className={`border-l-2 cursor-pointer ${statusBorder[run.status]}`}
                  hover
                  onClick={() => setSelectedRunId(run.id)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <span className="flex-shrink-0">{statusIcon[run.status]}</span>
                      <div>
                        <p className={`text-sm font-medium ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                          {run.workflow || run.workflowPath}
                        </p>
                        <div className="flex items-center gap-3 mt-0.5">
                          <span className="flex items-center gap-1 text-xs text-zinc-500">
                            <GitBranch size={11} />
                            {run.branch}
                          </span>
                          {run.commitSha && (
                            <span className="text-xs text-zinc-500 font-mono">
                              {run.commitSha.slice(0, 7)}
                            </span>
                          )}
                          <span className="text-xs text-zinc-500 capitalize">{run.trigger}</span>
                        </div>
                      </div>
                    </div>
                    <div className="flex items-center gap-3">
                      <span className="text-xs text-zinc-500">{formatRelativeTime(run.createdAt)}</span>
                      <ChevronRight size={14} className="text-zinc-500" />
                    </div>
                  </div>
                </GlassCard>
              ))}
            </div>
          )}
        </>
      )}

      {/* Secrets panel */}
      {panel === 'secrets' && (
        <div className="space-y-4">
          <GlassCard padding="md">
            <p className="text-xs text-zinc-500 mb-3">{t('actions.secrets.description')}</p>
            <div className="flex gap-2">
              <input
                type="text"
                placeholder={t('actions.secrets.keyPlaceholder')}
                value={newKey}
                onChange={(e) => setNewKey(e.target.value.toUpperCase().replace(/[^A-Z0-9_]/g, ''))}
                className={`flex-1 px-3 py-2 text-sm rounded-lg border font-mono ${
                  isDark
                    ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
                    : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
                }`}
              />
              <input
                type={showValues ? 'text' : 'password'}
                placeholder={t('actions.secrets.valuePlaceholder')}
                value={newValue}
                onChange={(e) => setNewValue(e.target.value)}
                className={`flex-1 px-3 py-2 text-sm rounded-lg border ${
                  isDark
                    ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
                    : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
                }`}
              />
              <PillButton
                variant="primary"
                size="sm"
                icon={savingSecret ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
                onClick={handleAddSecret}
                disabled={savingSecret || !newKey || !newValue}
              >
                {t('common.add')}
              </PillButton>
            </div>
          </GlassCard>

          {secretsLoading ? (
            <div className="flex justify-center py-8">
              <Loader2 className="animate-spin text-zinc-400" size={20} />
            </div>
          ) : secrets.length === 0 ? (
            <div className="text-center py-8 text-zinc-500 text-sm">{t('actions.secrets.empty')}</div>
          ) : (
            <div className="space-y-2">
              <div className="flex justify-end">
                <button
                  onClick={() => setShowValues((v) => !v)}
                  className="flex items-center gap-1.5 text-xs text-zinc-500 hover:text-zinc-300 transition-colors"
                >
                  {showValues ? <EyeOff size={12} /> : <Eye size={12} />}
                  {showValues ? t('actions.secrets.hide') : t('actions.secrets.show')}
                </button>
              </div>
              {secrets.map((secret) => (
                <GlassCard key={secret.key} padding="sm" className="flex items-center justify-between">
                  <span className={`text-sm font-mono font-medium ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                    {secret.key}
                  </span>
                  <div className="flex items-center gap-2">
                    <span className="text-xs text-zinc-500 font-mono">
                      {showValues ? '(set)' : '••••••••'}
                    </span>
                    <button
                      onClick={() => handleDeleteSecret(secret.key)}
                      disabled={deletingKey === secret.key}
                      className="p-1 text-zinc-500 hover:text-red-400 transition-colors"
                    >
                      {deletingKey === secret.key ? (
                        <Loader2 size={13} className="animate-spin" />
                      ) : (
                        <Trash2 size={13} />
                      )}
                    </button>
                  </div>
                </GlassCard>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

import { useState, useEffect, useCallback } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, ShieldCheck, Tag, Trash2, Plus, Loader2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { branchProtectionApi, type BranchProtectionRule } from '../api/repositories';
import { issuesApi, type Label } from '../api/issues';

function Toggle({ checked, onChange, label }: { checked: boolean; onChange: (v: boolean) => void; label: string }) {
  return (
    <label className="flex items-center justify-between gap-3 cursor-pointer">
      <span className="text-sm text-zinc-400">{label}</span>
      <button
        type="button"
        onClick={() => onChange(!checked)}
        className={`relative w-10 h-5 rounded-full transition-colors flex-shrink-0 ${checked ? 'bg-amber-400' : 'bg-zinc-600'}`}
      >
        <span className={`absolute top-0.5 left-0.5 w-4 h-4 rounded-full bg-white transition-transform ${checked ? 'translate-x-5' : ''}`} />
      </button>
    </label>
  );
}

export function RepositorySettings() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [rules, setRules] = useState<BranchProtectionRule[]>([]);
  const [labels, setLabels] = useState<Label[]>([]);
  const [loading, setLoading] = useState(true);

  // New rule form
  const [branch, setBranch] = useState('main');
  const [requireReviews, setRequireReviews] = useState(true);
  const [requiredApprovals, setRequiredApprovals] = useState(1);
  const [requireStatusChecks, setRequireStatusChecks] = useState(false);
  const [blockForcePush, setBlockForcePush] = useState(true);
  const [dismissStale, setDismissStale] = useState(false);
  const [saving, setSaving] = useState(false);

  // New label form
  const [labelName, setLabelName] = useState('');
  const [labelColor, setLabelColor] = useState('#6366f1');

  const load = useCallback(async () => {
    if (!repoId) return;
    setLoading(true);
    try {
      const [r, l] = await Promise.all([branchProtectionApi.list(repoId), issuesApi.listLabels(repoId)]);
      setRules(r);
      setLabels(l);
    } finally {
      setLoading(false);
    }
  }, [repoId]);

  useEffect(() => { load(); }, [load]);

  async function saveRule() {
    if (!repoId || !branch.trim()) return;
    setSaving(true);
    try {
      await branchProtectionApi.upsert(repoId, {
        branch: branch.trim(),
        requireReviews,
        requiredApprovals,
        dismissStaleReviews: dismissStale,
        requireStatusChecks,
        blockForcePush,
      });
      addNotification({ type: 'success', title: t('repo.protection.saved') });
      load();
    } catch {
      addNotification({ type: 'error', title: t('repo.protection.saveFailed') });
    } finally {
      setSaving(false);
    }
  }

  async function deleteRule(b: string) {
    if (!repoId) return;
    try {
      await branchProtectionApi.delete(repoId, b);
      setRules((prev) => prev.filter((r) => r.branch !== b));
      addNotification({ type: 'success', title: t('repo.protection.deleted') });
    } catch {
      addNotification({ type: 'error', title: t('repo.protection.saveFailed') });
    }
  }

  async function createLabel() {
    if (!repoId || !labelName.trim()) return;
    try {
      const l = await issuesApi.createLabel(repoId, { name: labelName.trim(), color: labelColor });
      setLabels((prev) => [...prev, l]);
      setLabelName('');
    } catch {
      addNotification({ type: 'error', title: t('issues.createFailed') });
    }
  }

  async function deleteLabel(id: string) {
    if (!repoId) return;
    try {
      await issuesApi.deleteLabel(repoId, id);
      setLabels((prev) => prev.filter((l) => l.id !== id));
    } catch {
      addNotification({ type: 'error', title: t('issues.labelDeleteFailed') });
    }
  }

  const inputClass = `px-3 py-2 text-sm rounded-lg border ${isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'}`;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center gap-4">
        <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}`)} icon={<ArrowLeft size={16} />}>
          {t('common.back')}
        </PillButton>
        <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {t('repo.detail.settings')}
        </h1>
      </div>

      {loading ? (
        <div className="flex justify-center py-16"><Loader2 className="animate-spin text-zinc-400" size={22} /></div>
      ) : (
        <>
          {/* Branch protection */}
          <GlassCard padding="md" className="space-y-4">
            <div className="flex items-center gap-2">
              <ShieldCheck size={16} className="text-amber-400" />
              <div>
                <p className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{t('repo.protection.title')}</p>
                <p className="text-xs text-zinc-500">{t('repo.protection.subtitle')}</p>
              </div>
            </div>

            {rules.length > 0 && (
              <div className="space-y-2">
                {rules.map((r) => (
                  <div key={r.id} className={`flex items-center justify-between gap-3 px-3 py-2 rounded-lg border ${isDark ? 'border-white/10 bg-white/5' : 'border-black/10 bg-black/5'}`}>
                    <div className="min-w-0">
                      <p className="text-sm font-mono text-amber-400">{r.branch}</p>
                      <p className="text-xs text-zinc-500">
                        {r.requireReviews && `${r.requiredApprovals} approval(s)`}
                        {r.requireStatusChecks && ' · checks'}
                        {r.blockForcePush && ' · no force-push'}
                      </p>
                    </div>
                    <button onClick={() => deleteRule(r.branch)} className="text-zinc-600 hover:text-red-400 transition-colors flex-shrink-0">
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className={`space-y-3 pt-3 border-t ${isDark ? 'border-white/10' : 'border-black/10'}`}>
              <div>
                <label className="text-xs font-medium block mb-1.5 text-zinc-400">{t('repo.protection.branch')}</label>
                <input type="text" value={branch} onChange={(e) => setBranch(e.target.value)} placeholder={t('repo.protection.branchPlaceholder')} className={`${inputClass} w-full`} />
              </div>
              <Toggle checked={requireReviews} onChange={setRequireReviews} label={t('repo.protection.requireReviews')} />
              {requireReviews && (
                <div className="flex items-center justify-between gap-3 pl-4">
                  <span className="text-sm text-zinc-400">{t('repo.protection.requiredApprovals')}</span>
                  <input type="number" min={1} max={10} value={requiredApprovals} onChange={(e) => setRequiredApprovals(Math.max(1, parseInt(e.target.value || '1', 10)))} className={`${inputClass} w-20`} />
                </div>
              )}
              <Toggle checked={dismissStale} onChange={setDismissStale} label={t('repo.protection.dismissStale')} />
              <Toggle checked={requireStatusChecks} onChange={setRequireStatusChecks} label={t('repo.protection.requireStatusChecks')} />
              <Toggle checked={blockForcePush} onChange={setBlockForcePush} label={t('repo.protection.blockForcePush')} />
              <PillButton variant="primary" size="sm" onClick={saveRule} disabled={saving || !branch.trim()} icon={saving ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}>
                {t('repo.protection.save')}
              </PillButton>
            </div>
          </GlassCard>

          {/* Labels */}
          <GlassCard padding="md" className="space-y-4">
            <div className="flex items-center gap-2">
              <Tag size={16} className="text-amber-400" />
              <p className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{t('issues.labels')}</p>
            </div>

            <div className="flex flex-wrap gap-2">
              {labels.map((l) => (
                <span key={l.id} className="inline-flex items-center gap-1 px-2 py-0.5 rounded-full text-[11px] font-medium border"
                  style={{ color: l.color, borderColor: l.color + '66', backgroundColor: l.color + '1a' }}>
                  {l.name}
                  <button onClick={() => deleteLabel(l.id)} className="hover:opacity-70"><Trash2 size={10} /></button>
                </span>
              ))}
              {labels.length === 0 && <p className="text-sm text-zinc-500">{t('issues.noLabels')}</p>}
            </div>

            <div className="flex items-end gap-2">
              <div className="flex-1">
                <label className="text-xs font-medium block mb-1.5 text-zinc-400">{t('issues.labelName')}</label>
                <input type="text" value={labelName} onChange={(e) => setLabelName(e.target.value)} className={`${inputClass} w-full`} />
              </div>
              <input type="color" value={labelColor} onChange={(e) => setLabelColor(e.target.value)} className="h-9 w-12 rounded-lg border border-white/10 bg-transparent cursor-pointer" />
              <PillButton variant="primary" size="sm" onClick={createLabel} disabled={!labelName.trim()} icon={<Plus size={13} />}>
                {t('issues.labelCreate')}
              </PillButton>
            </div>
          </GlassCard>
        </>
      )}
    </div>
  );
}

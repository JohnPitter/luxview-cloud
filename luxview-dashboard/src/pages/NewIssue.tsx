import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Loader2, CircleDot } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { issuesApi, type Label } from '../api/issues';

export function NewIssue() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [title, setTitle] = useState('');
  const [body, setBody] = useState('');
  const [labels, setLabels] = useState<Label[]>([]);
  const [selected, setSelected] = useState<Set<string>>(new Set());
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (!repoId) return;
    issuesApi.listLabels(repoId).then(setLabels).catch(() => {});
  }, [repoId]);

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${
    isDark ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600' : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
  }`;

  function toggleLabel(id: string) {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id); else next.add(id);
      return next;
    });
  }

  async function handleCreate() {
    if (!repoId || !title.trim()) return;
    setCreating(true);
    try {
      const issue = await issuesApi.create(repoId, { title: title.trim(), body: body.trim(), labelIds: [...selected] });
      addNotification({ type: 'success', title: t('issues.created') });
      navigate(`/dashboard/repositories/${repoId}/issues/${issue.number}`);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('issues.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-4 mb-8">
        <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}/issues`)} icon={<ArrowLeft size={16} />}>
          {t('common.back')}
        </PillButton>
        <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {t('issues.new')}
        </h1>
      </div>

      <div className="max-w-2xl mx-auto">
        <GlassCard padding="lg" className="space-y-4">
          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('issues.titleLabel')}
            </label>
            <input type="text" placeholder={t('issues.titlePlaceholder')} value={title} onChange={(e) => setTitle(e.target.value)} className={inputClass} autoFocus />
          </div>
          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('issues.bodyLabel')}
            </label>
            <textarea placeholder={t('issues.bodyPlaceholder')} value={body} onChange={(e) => setBody(e.target.value)} rows={8} className={`${inputClass} resize-none font-mono`} />
          </div>
          {labels.length > 0 && (
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('issues.labels')}
              </label>
              <div className="flex flex-wrap gap-2">
                {labels.map((l) => {
                  const on = selected.has(l.id);
                  return (
                    <button
                      key={l.id}
                      onClick={() => toggleLabel(l.id)}
                      className="inline-flex items-center px-2 py-0.5 rounded-full text-[11px] font-medium border transition-opacity"
                      style={{ color: l.color, borderColor: l.color + '66', backgroundColor: l.color + (on ? '33' : '0d'), opacity: on ? 1 : 0.6 }}
                    >
                      {l.name}
                    </button>
                  );
                })}
              </div>
            </div>
          )}
          <div className="pt-2">
            <PillButton
              variant="primary" size="md"
              icon={creating ? <Loader2 size={14} className="animate-spin" /> : <CircleDot size={14} />}
              onClick={handleCreate}
              disabled={creating || !title.trim()}
            >
              {creating ? t('issues.creating') : t('issues.create')}
            </PillButton>
          </div>
        </GlassCard>
      </div>
    </div>
  );
}

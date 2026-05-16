import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Loader2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { repositoriesApi, pullRequestsApi } from '../api/repositories';

export function NewPullRequest() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [branches, setBranches] = useState<string[]>([]);
  const [title, setTitle] = useState('');
  const [description, setDescription] = useState('');
  const [headBranch, setHeadBranch] = useState('');
  const [baseBranch, setBaseBranch] = useState('');
  const [creating, setCreating] = useState(false);

  useEffect(() => {
    if (!repoId) return;
    repositoriesApi.listBranches(repoId).then((b) => {
      setBranches(b);
      if (b.length > 0) setBaseBranch(b[0]);
      if (b.length > 1) setHeadBranch(b[1]);
    });
  }, [repoId]);

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${
    isDark
      ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
      : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
  }`;

  const selectClass = `w-full px-3 py-2 text-sm rounded-lg border font-mono ${
    isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'
  }`;

  async function handleCreate() {
    if (!repoId || !title.trim() || !headBranch || !baseBranch) return;
    setCreating(true);
    try {
      const pr = await pullRequestsApi.create(repoId, { title, description, headBranch, baseBranch });
      addNotification({ type: 'success', title: t('pr.created') });
      navigate(`/dashboard/repositories/${repoId}/pulls/${pr.number}`);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('pr.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  if (!repoId) return null;

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-4 mb-8">
        <PillButton variant="ghost" size="sm" onClick={() => navigate(-1)} icon={<ArrowLeft size={16} />}>
          {t('common.back')}
        </PillButton>
        <div>
          <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('pr.newTitle')}
          </h1>
          <p className="text-sm text-zinc-500 mt-0.5">{t('pr.newSubtitle')}</p>
        </div>
      </div>

      <div className="max-w-lg mx-auto">
        <GlassCard padding="lg" className="space-y-4">
          {/* Branch selectors */}
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('pr.baseBranch')}
              </label>
              <select value={baseBranch} onChange={(e) => setBaseBranch(e.target.value)} className={selectClass}>
                {branches.map((b) => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('pr.headBranch')}
              </label>
              <select value={headBranch} onChange={(e) => setHeadBranch(e.target.value)} className={selectClass}>
                {branches.map((b) => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
          </div>

          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('pr.titleLabel')}
            </label>
            <input
              type="text"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder={t('pr.titlePlaceholder')}
              className={inputClass}
              autoFocus
            />
          </div>

          <div>
            <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
              {t('pr.description')}
            </label>
            <textarea
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder={t('pr.descriptionPlaceholder')}
              rows={4}
              className={`${inputClass} resize-none`}
            />
          </div>

          <div className="pt-2">
            <PillButton
              variant="primary"
              size="md"
              icon={creating ? <Loader2 size={14} className="animate-spin" /> : undefined}
              onClick={handleCreate}
              disabled={creating || !title.trim() || !headBranch || !baseBranch || headBranch === baseBranch}
            >
              {t('pr.create')}
            </PillButton>
          </div>
        </GlassCard>
      </div>
    </div>
  );
}

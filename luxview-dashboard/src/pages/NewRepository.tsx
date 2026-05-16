import { useState } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Loader2, Github, Plus } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { repositoriesApi } from '../api/repositories';

type Mode = 'create' | 'import';

export function NewRepository() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [mode, setMode] = useState<Mode>('create');

  // Create form
  const [name, setName] = useState('');
  const [defaultBranch, setDefaultBranch] = useState('main');
  const [visibility, setVisibility] = useState<'private' | 'public'>('private');
  const [creating, setCreating] = useState(false);

  // Import form
  const [ghOwner, setGhOwner] = useState('');
  const [ghRepo, setGhRepo] = useState('');
  const [ghBranch, setGhBranch] = useState('main');
  const [ghVisibility, setGhVisibility] = useState<'private' | 'public'>('private');
  const [importing, setImporting] = useState(false);

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${
    isDark
      ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
      : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
  }`;

  const selectClass = `px-3 py-2 text-sm rounded-lg border ${
    isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'
  }`;

  async function handleCreate() {
    if (!name.trim()) return;
    setCreating(true);
    try {
      const repo = await repositoriesApi.create({ name: name.trim(), defaultBranch, visibility });
      addNotification({ type: 'success', title: t('repo.new.created') });
      navigate(`/dashboard/new?source=luxview&repoId=${repo.id}`);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('repo.new.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  async function handleImport() {
    if (!ghOwner.trim() || !ghRepo.trim()) return;
    setImporting(true);
    try {
      const repo = await repositoriesApi.importFromGitHub({
        owner: ghOwner.trim(),
        repo: ghRepo.trim(),
        defaultBranch: ghBranch || undefined,
        visibility: ghVisibility,
      });
      addNotification({ type: 'success', title: t('repo.import.imported') });
      navigate(`/dashboard/new?source=luxview&repoId=${repo.id}`);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('repo.import.failed') });
    } finally {
      setImporting(false);
    }
  }

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-4 mb-8">
        <PillButton variant="ghost" size="sm" onClick={() => navigate(-1)} icon={<ArrowLeft size={16} />}>
          {t('common.back')}
        </PillButton>
        <div>
          <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('repo.new.title')}
          </h1>
          <p className="text-sm text-zinc-500 mt-0.5">{t('repo.new.subtitle')}</p>
        </div>
      </div>

      {/* Mode tabs */}
      <div className="max-w-lg mx-auto mb-4 flex gap-1">
        <button
          onClick={() => setMode('create')}
          className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
            mode === 'create'
              ? 'bg-amber-400/10 text-amber-400'
              : isDark ? 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50' : 'text-zinc-400 hover:text-zinc-700 hover:bg-zinc-100'
          }`}
        >
          <Plus size={13} /> {t('repo.new.tabCreate')}
        </button>
        <button
          onClick={() => setMode('import')}
          className={`flex items-center gap-1.5 px-3 py-1.5 text-xs font-medium rounded-lg transition-colors ${
            mode === 'import'
              ? 'bg-amber-400/10 text-amber-400'
              : isDark ? 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50' : 'text-zinc-400 hover:text-zinc-700 hover:bg-zinc-100'
          }`}
        >
          <Github size={13} /> {t('repo.new.tabImport')}
        </button>
      </div>

      <div className="max-w-lg mx-auto">
        {mode === 'create' ? (
          <GlassCard padding="lg" className="space-y-4">
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('repo.new.name')}
              </label>
              <input
                type="text"
                placeholder={t('repo.new.namePlaceholder')}
                value={name}
                onChange={(e) => setName(e.target.value)}
                className={inputClass}
                autoFocus
              />
            </div>
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('repo.new.defaultBranch')}
              </label>
              <input
                type="text"
                value={defaultBranch}
                onChange={(e) => setDefaultBranch(e.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('repo.new.visibility')}
              </label>
              <select value={visibility} onChange={(e) => setVisibility(e.target.value as 'private' | 'public')} className={selectClass}>
                <option value="private">{t('repo.new.private')}</option>
                <option value="public">{t('repo.new.public')}</option>
              </select>
            </div>
            <div className="pt-2">
              <PillButton
                variant="primary" size="md"
                icon={creating ? <Loader2 size={14} className="animate-spin" /> : undefined}
                onClick={handleCreate}
                disabled={creating || !name.trim()}
              >
                {t('repo.new.create')}
              </PillButton>
            </div>
          </GlassCard>
        ) : (
          <GlassCard padding="lg" className="space-y-4">
            <p className="text-xs text-zinc-500">{t('repo.import.hint')}</p>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                  {t('repo.import.owner')}
                </label>
                <input
                  type="text"
                  placeholder="octocat"
                  value={ghOwner}
                  onChange={(e) => setGhOwner(e.target.value)}
                  className={inputClass}
                  autoFocus
                />
              </div>
              <div>
                <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                  {t('repo.import.repoName')}
                </label>
                <input
                  type="text"
                  placeholder="my-repo"
                  value={ghRepo}
                  onChange={(e) => setGhRepo(e.target.value)}
                  className={inputClass}
                />
              </div>
            </div>
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('repo.new.defaultBranch')}
              </label>
              <input
                type="text"
                value={ghBranch}
                onChange={(e) => setGhBranch(e.target.value)}
                className={inputClass}
              />
            </div>
            <div>
              <label className={`text-xs font-medium block mb-1.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('repo.new.visibility')}
              </label>
              <select value={ghVisibility} onChange={(e) => setGhVisibility(e.target.value as 'private' | 'public')} className={selectClass}>
                <option value="private">{t('repo.new.private')}</option>
                <option value="public">{t('repo.new.public')}</option>
              </select>
            </div>
            <div className="pt-2">
              <PillButton
                variant="primary" size="md"
                icon={importing ? <Loader2 size={14} className="animate-spin" /> : <Github size={14} />}
                onClick={handleImport}
                disabled={importing || !ghOwner.trim() || !ghRepo.trim()}
              >
                {importing ? t('repo.import.importing') : t('repo.import.import')}
              </PillButton>
            </div>
          </GlassCard>
        )}
      </div>
    </div>
  );
}

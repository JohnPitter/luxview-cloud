import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, GitBranch, Plus, Trash2, Loader2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { gitApi } from '../api/git';
import { repositoriesApi } from '../api/repositories';

export function RepositoryBranches() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [defaultBranch, setDefaultBranch] = useState('main');
  const [branches, setBranches] = useState<string[]>([]);
  const [loading, setLoading] = useState(true);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [newFrom, setNewFrom] = useState('');
  const [showForm, setShowForm] = useState(false);
  const [deletingBranch, setDeletingBranch] = useState<string | null>(null);

  async function load() {
    if (!repoId) return;
    const [db, b] = await Promise.all([
      repositoriesApi.list(100).then((r) => r.find((x) => x.id === repoId)?.defaultBranch ?? 'main'),
      repositoriesApi.listBranches(repoId),
    ]);
    setDefaultBranch(db);
    setBranches(b);
    setLoading(false);
  }

  useEffect(() => { load(); }, [repoId]);

  async function handleCreate() {
    if (!repoId || !newName.trim()) return;
    setCreating(true);
    try {
      await gitApi.createBranch(repoId, newName.trim(), newFrom || defaultBranch);
      addNotification({ type: 'success', title: t('branches.created') });
      setNewName('');
      setNewFrom('');
      setShowForm(false);
      await load();
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('branches.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(name: string) {
    if (!repoId) return;
    setDeletingBranch(name);
    try {
      await gitApi.deleteBranch(repoId, name);
      addNotification({ type: 'success', title: t('branches.deleted') });
      setBranches((prev) => prev.filter((b) => b !== name));
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('branches.deleteFailed') });
    } finally {
      setDeletingBranch(null);
    }
  }

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${isDark ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600' : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'}`;

  return (
    <div className="animate-fade-in space-y-4">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <PillButton variant="ghost" size="sm" onClick={() => navigate(`/dashboard/repositories/${repoId}`)} icon={<ArrowLeft size={16} />}>
            {t('common.back')}
          </PillButton>
          <h1 className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('branches.title')}
          </h1>
        </div>
        <PillButton variant="primary" size="sm" icon={<Plus size={14} />} onClick={() => setShowForm(!showForm)}>
          {t('branches.new')}
        </PillButton>
      </div>

      {showForm && (
        <GlassCard padding="md" className="space-y-3">
          <p className={`text-xs font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{t('branches.newTitle')}</p>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-zinc-500 block mb-1">{t('branches.name')}</label>
              <input type="text" placeholder="feature/my-feature" value={newName} onChange={(e) => setNewName(e.target.value)} className={inputClass} autoFocus />
            </div>
            <div>
              <label className="text-xs text-zinc-500 block mb-1">{t('branches.from')}</label>
              <select value={newFrom} onChange={(e) => setNewFrom(e.target.value)} className={`px-3 py-2 text-sm rounded-lg border ${isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'}`}>
                {branches.map((b) => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
          </div>
          <div className="flex gap-2">
            <PillButton variant="ghost" size="sm" onClick={() => setShowForm(false)}>{t('common.cancel')}</PillButton>
            <PillButton
              variant="primary"
              size="sm"
              onClick={handleCreate}
              disabled={creating || !newName.trim()}
              icon={creating ? <Loader2 size={13} className="animate-spin" /> : undefined}
            >
              {t('branches.create')}
            </PillButton>
          </div>
        </GlassCard>
      )}

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="animate-spin text-zinc-400" size={24} />
        </div>
      ) : (
        <div className="space-y-2">
          {branches.map((b) => (
            <GlassCard key={b} padding="md">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <GitBranch size={15} className="text-indigo-400" />
                  <span className={`text-sm font-mono ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{b}</span>
                  {b === defaultBranch && (
                    <span className="text-xs px-1.5 py-0.5 rounded bg-amber-400/10 text-amber-400 border border-amber-400/20">
                      {t('branches.default')}
                    </span>
                  )}
                </div>
                {b !== defaultBranch && (
                  <button
                    onClick={() => handleDelete(b)}
                    disabled={deletingBranch === b}
                    className="p-1.5 text-zinc-600 hover:text-red-400 transition-colors disabled:opacity-40"
                  >
                    {deletingBranch === b ? <Loader2 size={13} className="animate-spin" /> : <Trash2 size={13} />}
                  </button>
                )}
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
}

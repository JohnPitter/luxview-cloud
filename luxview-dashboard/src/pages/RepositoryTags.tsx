import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Tag, Plus, Trash2, Loader2 } from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { gitApi, type TagEntry } from '../api/git';
import { repositoriesApi } from '../api/repositories';

function formatDate(iso: string) {
  if (!iso) return '';
  try {
    return new Date(iso).toLocaleDateString(undefined, { year: 'numeric', month: 'short', day: 'numeric' });
  } catch {
    return iso;
  }
}

export function RepositoryTags() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [branches, setBranches] = useState<string[]>([]);
  const [tags, setTags] = useState<TagEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [creating, setCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [newRef, setNewRef] = useState('');
  const [newMessage, setNewMessage] = useState('');
  const [deletingTag, setDeletingTag] = useState<string | null>(null);

  async function load() {
    if (!repoId) return;
    const [b, { tags: tgs }] = await Promise.all([
      repositoriesApi.listBranches(repoId),
      gitApi.listTags(repoId),
    ]);
    setBranches(b);
    setTags(tgs ?? []);
    setLoading(false);
  }

  useEffect(() => { load(); }, [repoId]);

  async function handleCreate() {
    if (!repoId || !newName.trim()) return;
    setCreating(true);
    try {
      await gitApi.createTag(repoId, newName.trim(), newRef, newMessage);
      addNotification({ type: 'success', title: t('tags.created') });
      setNewName('');
      setNewRef('');
      setNewMessage('');
      setShowForm(false);
      await load();
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('tags.createFailed') });
    } finally {
      setCreating(false);
    }
  }

  async function handleDelete(name: string) {
    if (!repoId) return;
    setDeletingTag(name);
    try {
      await gitApi.deleteTag(repoId, name);
      addNotification({ type: 'success', title: t('tags.deleted') });
      setTags((prev) => prev.filter((tg) => tg.name !== name));
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('tags.deleteFailed') });
    } finally {
      setDeletingTag(null);
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
            {t('tags.title')}
          </h1>
        </div>
        <PillButton variant="primary" size="sm" icon={<Plus size={14} />} onClick={() => setShowForm(!showForm)}>
          {t('tags.new')}
        </PillButton>
      </div>

      {showForm && (
        <GlassCard padding="md" className="space-y-3">
          <p className={`text-xs font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{t('tags.newTitle')}</p>
          <div className="grid grid-cols-2 gap-3">
            <div>
              <label className="text-xs text-zinc-500 block mb-1">{t('tags.name')}</label>
              <input type="text" placeholder="v1.0.0" value={newName} onChange={(e) => setNewName(e.target.value)} className={inputClass} autoFocus />
            </div>
            <div>
              <label className="text-xs text-zinc-500 block mb-1">{t('tags.from')}</label>
              <select value={newRef} onChange={(e) => setNewRef(e.target.value)} className={`px-3 py-2 text-sm rounded-lg border ${isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'}`}>
                <option value="">{t('tags.fromDefault')}</option>
                {branches.map((b) => <option key={b} value={b}>{b}</option>)}
              </select>
            </div>
          </div>
          <div>
            <label className="text-xs text-zinc-500 block mb-1">{t('tags.message')}</label>
            <input type="text" placeholder={t('tags.messagePlaceholder')} value={newMessage} onChange={(e) => setNewMessage(e.target.value)} className={inputClass} />
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
              {t('tags.create')}
            </PillButton>
          </div>
        </GlassCard>
      )}

      {loading ? (
        <div className="flex justify-center py-16">
          <Loader2 className="animate-spin text-zinc-400" size={24} />
        </div>
      ) : tags.length === 0 ? (
        <GlassCard padding="lg" className="text-center py-12">
          <Tag size={32} className="mx-auto text-zinc-500 mb-3" />
          <p className="text-sm text-zinc-500">{t('tags.empty')}</p>
        </GlassCard>
      ) : (
        <div className="space-y-2">
          {tags.map((tg) => (
            <GlassCard key={tg.name} padding="md">
              <div className="flex items-center justify-between">
                <div className="flex items-start gap-3">
                  <Tag size={15} className="text-emerald-400 flex-shrink-0 mt-0.5" />
                  <div>
                    <p className={`text-sm font-mono font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>{tg.name}</p>
                    <div className="flex items-center gap-2 mt-0.5">
                      <span className="font-mono text-xs text-zinc-600">{tg.sha.slice(0, 8)}</span>
                      {tg.type === 'annotated' && (
                        <span className="text-xs px-1.5 py-0.5 rounded bg-purple-400/10 text-purple-400 border border-purple-400/20">
                          {t('tags.annotated')}
                        </span>
                      )}
                      {tg.date && <span className="text-xs text-zinc-500">{formatDate(tg.date)}</span>}
                    </div>
                    {tg.message && (
                      <p className="text-xs text-zinc-500 mt-1">{tg.message}</p>
                    )}
                  </div>
                </div>
                <button
                  onClick={() => handleDelete(tg.name)}
                  disabled={deletingTag === tg.name}
                  className="p-1.5 text-zinc-600 hover:text-red-400 transition-colors disabled:opacity-40"
                >
                  {deletingTag === tg.name ? <Loader2 size={13} className="animate-spin" /> : <Trash2 size={13} />}
                </button>
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
}

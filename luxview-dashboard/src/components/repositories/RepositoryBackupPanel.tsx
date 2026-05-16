import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { RefreshCw, Plus, Check, X, Loader2, Clock, Github } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { repositoriesApi, type RepositoryRemote } from '../../api/repositories';
import { formatRelativeTime } from '../../lib/format';

interface RepositoryBackupPanelProps {
  repositoryId: string;
}

export function RepositoryBackupPanel({ repositoryId }: RepositoryBackupPanelProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [remotes, setRemotes] = useState<RepositoryRemote[]>([]);
  const [loading, setLoading] = useState(true);
  const [syncing, setSyncing] = useState<string | null>(null);

  const [showAdd, setShowAdd] = useState(false);
  const [newProvider, setNewProvider] = useState('github');
  const [newUrl, setNewUrl] = useState('');
  const [adding, setAdding] = useState(false);

  const fetchRemotes = useCallback(async () => {
    try {
      const result = await repositoriesApi.listRemotes(repositoryId);
      setRemotes(result);
    } catch {
      // silent
    } finally {
      setLoading(false);
    }
  }, [repositoryId]);

  useEffect(() => {
    fetchRemotes();
  }, [fetchRemotes]);

  async function handleAdd() {
    if (!newUrl.trim()) return;
    setAdding(true);
    try {
      const remote = await repositoriesApi.addRemote(repositoryId, newProvider, newUrl.trim());
      setRemotes((prev) => [...prev, remote]);
      setNewUrl('');
      setShowAdd(false);
      addNotification({ type: 'success', title: t('repo.backup.added') });
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('repo.backup.addFailed') });
    } finally {
      setAdding(false);
    }
  }

  async function handleSync(remote: RepositoryRemote) {
    setSyncing(remote.id);
    try {
      await repositoriesApi.syncRemote(repositoryId, remote.id);
      addNotification({ type: 'success', title: t('repo.backup.syncStarted') });
      // Optimistically update sync status after a short delay
      setTimeout(() => fetchRemotes(), 3000);
    } catch (e: unknown) {
      const msg = (e as { response?: { data?: { error?: string } } })?.response?.data?.error;
      addNotification({ type: 'error', title: msg ?? t('repo.backup.syncFailed') });
    } finally {
      setSyncing(null);
    }
  }

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border ${
    isDark
      ? 'bg-white/5 border-white/10 text-white placeholder:text-zinc-600'
      : 'bg-black/5 border-black/10 text-zinc-900 placeholder:text-zinc-400'
  }`;

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className={`text-sm font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
          {t('repo.backup.title')}
        </h3>
        <div className="flex gap-2">
          <PillButton variant="ghost" size="sm" icon={<RefreshCw size={13} />} onClick={fetchRemotes}>
            {t('common.refresh')}
          </PillButton>
          <PillButton variant="primary" size="sm" icon={<Plus size={13} />} onClick={() => setShowAdd((v) => !v)}>
            {t('repo.backup.addRemote')}
          </PillButton>
        </div>
      </div>

      {showAdd && (
        <GlassCard padding="md" className="space-y-3">
          <p className="text-xs text-zinc-500">{t('repo.backup.addDescription')}</p>
          <div className="flex gap-2">
            <select
              value={newProvider}
              onChange={(e) => setNewProvider(e.target.value)}
              className={`px-3 py-2 text-sm rounded-lg border ${
                isDark ? 'bg-white/5 border-white/10 text-white' : 'bg-black/5 border-black/10 text-zinc-900'
              }`}
            >
              <option value="github">GitHub</option>
            </select>
            <input
              type="text"
              placeholder="https://github.com/owner/repo.git"
              value={newUrl}
              onChange={(e) => setNewUrl(e.target.value)}
              className={inputClass}
            />
            <PillButton
              variant="primary"
              size="sm"
              icon={adding ? <Loader2 size={13} className="animate-spin" /> : <Plus size={13} />}
              onClick={handleAdd}
              disabled={adding || !newUrl.trim()}
            >
              {t('common.add')}
            </PillButton>
          </div>
        </GlassCard>
      )}

      {loading ? (
        <div className="flex justify-center py-8">
          <Loader2 className="animate-spin text-zinc-400" size={20} />
        </div>
      ) : remotes.length === 0 ? (
        <div className="text-center py-8 text-zinc-500 text-sm">{t('repo.backup.empty')}</div>
      ) : (
        <div className="space-y-2">
          {remotes.map((remote) => (
            <GlassCard key={remote.id} padding="sm" className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="p-1.5 rounded-md bg-zinc-500/10">
                  <Github size={14} className="text-zinc-400" />
                </div>
                <div>
                  <p className={`text-sm font-medium ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                    {remote.remoteUrl}
                  </p>
                  <div className="flex items-center gap-2 mt-0.5">
                    {remote.lastSyncStatus === 'success' && (
                      <span className="flex items-center gap-1 text-xs text-emerald-400">
                        <Check size={11} /> {t('repo.backup.syncOk')}
                      </span>
                    )}
                    {remote.lastSyncStatus === 'failed' && (
                      <span className="flex items-center gap-1 text-xs text-red-400">
                        <X size={11} /> {t('repo.backup.syncError')}
                      </span>
                    )}
                    {!remote.lastSyncStatus && (
                      <span className="flex items-center gap-1 text-xs text-zinc-500">
                        <Clock size={11} /> {t('repo.backup.neverSynced')}
                      </span>
                    )}
                    {remote.lastSyncAt && (
                      <span className="text-xs text-zinc-500">{formatRelativeTime(remote.lastSyncAt)}</span>
                    )}
                  </div>
                  {remote.lastSyncError && remote.lastSyncStatus === 'failed' && (
                    <p className="text-xs text-red-400 mt-0.5 font-mono truncate max-w-xs">
                      {remote.lastSyncError}
                    </p>
                  )}
                </div>
              </div>
              <PillButton
                variant="ghost"
                size="sm"
                icon={syncing === remote.id ? <Loader2 size={13} className="animate-spin" /> : <RefreshCw size={13} />}
                onClick={() => handleSync(remote)}
                disabled={syncing !== null}
              >
                {t('repo.backup.sync')}
              </PillButton>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
}

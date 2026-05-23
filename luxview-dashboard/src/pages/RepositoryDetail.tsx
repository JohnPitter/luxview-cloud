import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Rocket, Copy, Check, Trash2, GitPullRequest, FileCode2, GitCommit, GitBranch, Tag, Globe, Lock } from 'lucide-react';
import { PillButton } from '../components/common/PillButton';
import { GlassCard } from '../components/common/GlassCard';
import { RepositoryBackupPanel } from '../components/repositories/RepositoryBackupPanel';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { repositoriesApi, type LuxViewRepository } from '../api/repositories';

export function RepositoryDetail() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);
  const [repo, setRepo] = useState<LuxViewRepository | null>(null);
  const [copied, setCopied] = useState<string | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [confirmDelete, setConfirmDelete] = useState(false);
  const [togglingVisibility, setTogglingVisibility] = useState(false);

  async function handleToggleVisibility() {
    if (!repo || !repoId) return;
    setTogglingVisibility(true);
    try {
      const newVisibility = repo.visibility === 'public' ? 'private' : 'public';
      const updated = await repositoriesApi.updateVisibility(repoId, newVisibility);
      setRepo((prev) => prev ? { ...prev, visibility: updated.visibility } : prev);
      addNotification({ type: 'success', title: t('repo.detail.visibilityUpdated') });
    } catch {
      addNotification({ type: 'error', title: t('repo.detail.visibilityFailed') });
    } finally {
      setTogglingVisibility(false);
    }
  }

  async function handleDelete() {
    if (!repoId) return;
    setDeleting(true);
    try {
      await repositoriesApi.delete(repoId);
      addNotification({ type: 'success', title: t('repo.detail.deleted') });
      navigate('/dashboard/repositories');
    } catch {
      addNotification({ type: 'error', title: t('repo.detail.deleteFailed') });
      setDeleting(false);
      setConfirmDelete(false);
    }
  }

  function copyToClipboard(text: string, key: string) {
    navigator.clipboard.writeText(text);
    setCopied(key);
    setTimeout(() => setCopied(null), 2000);
  }

  useEffect(() => {
    if (!repoId) return;
    repositoriesApi.list(100).then((repos) => {
      const found = repos.find((r) => r.id === repoId);
      if (found) setRepo(found);
    });
  }, [repoId]);

  if (!repoId) return null;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <PillButton
            variant="ghost"
            size="sm"
            onClick={() => navigate(-1)}
            icon={<ArrowLeft size={16} />}
          >
            {t('common.back')}
          </PillButton>
          <div>
            <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
              {repo?.name ?? t('repo.detail.title')}
            </h1>
            {repo && (
              <p className="text-sm text-zinc-500 font-mono mt-0.5">{repo.slug} · {repo.defaultBranch}</p>
            )}
          </div>
        </div>

        <div className="flex items-center gap-2 flex-wrap justify-end">
          <PillButton variant="ghost" size="sm" icon={<FileCode2 size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/code`)}>
            {t('repo.detail.code')}
          </PillButton>
          <PillButton variant="ghost" size="sm" icon={<GitCommit size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/commits`)}>
            {t('repo.detail.commits')}
          </PillButton>
          <PillButton variant="ghost" size="sm" icon={<GitBranch size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/branches`)}>
            {t('repo.detail.branches')}
          </PillButton>
          <PillButton variant="ghost" size="sm" icon={<Tag size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/tags`)}>
            {t('repo.detail.tags')}
          </PillButton>
          <PillButton variant="ghost" size="sm" icon={<GitPullRequest size={14} />} onClick={() => navigate(`/dashboard/repositories/${repoId}/pulls`)}>
            {t('repo.detail.pullRequests')}
          </PillButton>
          <PillButton variant="primary" size="sm" icon={<Rocket size={14} />} onClick={() => navigate(`/dashboard/new?source=luxview&repoId=${repoId}`)}>
            {t('repo.detail.createApp')}
          </PillButton>
          <PillButton variant="ghost" size="sm" icon={<Trash2 size={14} className="text-red-400" />} onClick={() => setConfirmDelete(true)}>
            <span className="text-red-400">{t('repo.detail.delete')}</span>
          </PillButton>
        </div>
      </div>

      {confirmDelete && (
        <GlassCard padding="md" className={`border ${isDark ? 'border-red-500/30 bg-red-500/5' : 'border-red-200 bg-red-50'}`}>
          <p className={`text-sm font-medium mb-1 ${isDark ? 'text-red-300' : 'text-red-700'}`}>
            {t('repo.detail.deleteConfirmTitle')}
          </p>
          <p className="text-xs text-zinc-500 mb-3">{t('repo.detail.deleteConfirmMsg')}</p>
          <div className="flex gap-2">
            <PillButton
              variant="ghost"
              size="sm"
              onClick={() => setConfirmDelete(false)}
              disabled={deleting}
            >
              {t('common.cancel')}
            </PillButton>
            <button
              onClick={handleDelete}
              disabled={deleting}
              className="px-3 py-1.5 text-xs font-medium rounded-lg bg-red-500 hover:bg-red-600 text-white transition-colors disabled:opacity-50"
            >
              {deleting ? t('common.loading') : t('repo.detail.deleteConfirm')}
            </button>
          </div>
        </GlassCard>
      )}

      {repo && (() => {
        const cloneUrl = `${window.location.origin}/git/${repo.ownerUsername}/${repo.slug}.git`;
        const commands = [
          { key: 'clone', label: t('repo.detail.clone'), cmd: `git clone ${cloneUrl}` },
          { key: 'remote', label: t('repo.detail.addRemote'), cmd: `git remote add origin ${cloneUrl}` },
          { key: 'push', label: t('repo.detail.push'), cmd: `git push -u origin ${repo.defaultBranch}` },
        ];
        return (
          <GlassCard padding="md" className="space-y-3">
            <p className={`text-xs font-semibold uppercase tracking-wider ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
              {t('repo.detail.cloneTitle')}
            </p>
            {commands.map(({ key, label, cmd }) => (
              <div key={key}>
                <p className="text-xs text-zinc-500 mb-1">{label}</p>
                <div className={`flex items-center gap-2 px-3 py-2 rounded-lg font-mono text-xs border ${
                  isDark ? 'bg-zinc-900/50 border-zinc-800 text-zinc-200' : 'bg-zinc-100 border-zinc-200 text-zinc-800'
                }`}>
                  <span className="flex-1 truncate">{cmd}</span>
                  <button
                    onClick={() => copyToClipboard(cmd, key)}
                    className="flex-shrink-0 text-zinc-500 hover:text-zinc-300 transition-colors"
                  >
                    {copied === key ? <Check size={13} className="text-emerald-400" /> : <Copy size={13} />}
                  </button>
                </div>
              </div>
            ))}
          </GlassCard>
        );
      })()}

      {repo && (
        <GlassCard padding="md">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              {repo.visibility === 'public'
                ? <Globe size={16} className="text-emerald-400" />
                : <Lock size={16} className="text-zinc-500" />
              }
              <div>
                <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                  {t(`repo.new.${repo.visibility}`)}
                </p>
                <p className="text-xs text-zinc-500">
                  {repo.visibility === 'public' ? t('repo.detail.visibilityPublicDesc') : t('repo.detail.visibilityPrivateDesc')}
                </p>
              </div>
            </div>
            <PillButton
              variant="ghost"
              size="sm"
              disabled={togglingVisibility}
              onClick={handleToggleVisibility}
            >
              {togglingVisibility ? t('common.loading') : t('repo.detail.visibilityToggle')}
            </PillButton>
          </div>
        </GlassCard>
      )}

      <RepositoryBackupPanel repositoryId={repoId} />
    </div>
  );
}

import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Rocket, Copy, Check } from 'lucide-react';
import { PillButton } from '../components/common/PillButton';
import { GlassCard } from '../components/common/GlassCard';
import { RepositoryBackupPanel } from '../components/repositories/RepositoryBackupPanel';
import { useThemeStore } from '../stores/theme.store';
import { repositoriesApi, type LuxViewRepository } from '../api/repositories';

export function RepositoryDetail() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const [repo, setRepo] = useState<LuxViewRepository | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

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

        <PillButton
          variant="primary"
          size="sm"
          icon={<Rocket size={14} />}
          onClick={() => navigate(`/dashboard/new?source=luxview&repoId=${repoId}`)}
        >
          {t('repo.detail.createApp')}
        </PillButton>
      </div>

      {repo && (() => {
        const cloneUrl = `${window.location.origin}/git/${repo.slug}.git`;
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

      <RepositoryBackupPanel repositoryId={repoId} />
    </div>
  );
}

import { useState, useEffect } from 'react';
import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Rocket } from 'lucide-react';
import { PillButton } from '../components/common/PillButton';
import { RepositoryBackupPanel } from '../components/repositories/RepositoryBackupPanel';
import { useThemeStore } from '../stores/theme.store';
import { repositoriesApi, type LuxViewRepository } from '../api/repositories';

export function RepositoryDetail() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const [repo, setRepo] = useState<LuxViewRepository | null>(null);

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

      <RepositoryBackupPanel repositoryId={repoId} />
    </div>
  );
}

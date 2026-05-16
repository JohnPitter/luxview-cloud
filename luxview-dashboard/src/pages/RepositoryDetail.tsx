import { useNavigate, useParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft } from 'lucide-react';
import { PillButton } from '../components/common/PillButton';
import { RepositoryBackupPanel } from '../components/repositories/RepositoryBackupPanel';
import { useThemeStore } from '../stores/theme.store';

export function RepositoryDetail() {
  const { repoId } = useParams<{ repoId: string }>();
  const navigate = useNavigate();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  if (!repoId) return null;

  return (
    <div className="animate-fade-in space-y-6">
      <div className="flex items-center gap-4">
        <PillButton
          variant="ghost"
          size="sm"
          onClick={() => navigate(-1)}
          icon={<ArrowLeft size={16} />}
        >
          {t('common.back')}
        </PillButton>
        <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {t('repo.detail.title')}
        </h1>
      </div>

      <RepositoryBackupPanel repositoryId={repoId} />
    </div>
  );
}

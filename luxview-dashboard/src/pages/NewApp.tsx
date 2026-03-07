import { useState, useEffect, useCallback, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft } from 'lucide-react';
import { DeployWizard, type DeployConfig } from '../components/deploy/DeployWizard';
import { PillButton } from '../components/common/PillButton';
import { PageTour } from '../components/common/PageTour';
import { useAppsStore } from '../stores/apps.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useThemeStore } from '../stores/theme.store';
import { githubApi, type GithubRepo } from '../api/github';
import { newAppTourSteps } from '../tours/newApp';

export function NewApp() {
  const navigate = useNavigate();
  const { t } = useTranslation();
  const createApp = useAppsStore((s) => s.createApp);
  const apps = useAppsStore((s) => s.apps);
  const fetchApps = useAppsStore((s) => s.fetchApps);
  const addNotification = useNotificationsStore((s) => s.add);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [repos, setRepos] = useState<GithubRepo[]>([]);
  const [loadingRepos, setLoadingRepos] = useState(true);
  const [branches, setBranches] = useState<string[]>([]);
  const [deploying, setDeploying] = useState(false);

  useEffect(() => {
    fetchApps();
    githubApi
      .listRepos()
      .then(setRepos)
      .catch(() => {
        addNotification({ type: 'error', title: t('app.notifications.failedToLoadRepos') });
      })
      .finally(() => setLoadingRepos(false));
  }, [addNotification, fetchApps, t]);

  const deployedRepoUrls = useMemo(
    () => new Set(apps.map((a) => a.repoUrl.replace(/\.git$/, ''))),
    [apps],
  );

  const availableRepos = useMemo(
    () => repos.filter((r) => !deployedRepoUrls.has(r.htmlUrl.replace(/\.git$/, ''))),
    [repos, deployedRepoUrls],
  );

  const handleRepoSelect = useCallback(
    async (repo: GithubRepo) => {
      try {
        const branchList = await githubApi.listBranches(repo.fullName);
        setBranches(branchList.map((b) => b.name));
      } catch {
        setBranches([repo.defaultBranch]);
      }
    },
    [],
  );

  const handleDeploy = async (config: DeployConfig) => {
    setDeploying(true);
    try {
      const app = await createApp({
        name: config.repo.name,
        subdomain: config.subdomain,
        repoUrl: config.repo.htmlUrl,
        repoBranch: config.branch,
        envVars: config.envVars,
      });
      addNotification({
        type: 'success',
        title: t('app.notifications.deploymentStarted'),
        message: t('app.notifications.deploymentStartedMessage', { name: app.name }),
      });
      navigate(`/dashboard/apps/${app.id}`);
    } catch {
      addNotification({
        type: 'error',
        title: t('app.notifications.deploymentFailed'),
        message: t('app.notifications.deploymentFailedMessage'),
      });
    } finally {
      setDeploying(false);
    }
  };

  return (
    <div className="animate-fade-in">
      <PageTour tourId="newApp" steps={newAppTourSteps} autoStart />

      <div className="flex items-center gap-4 mb-8">
        <PillButton
          variant="ghost"
          size="sm"
          onClick={() => navigate('/dashboard')}
          icon={<ArrowLeft size={16} />}
        >
          {t('common.back')}
        </PillButton>
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            {t('app.title')}
          </h1>
          <p className="text-sm text-zinc-500 mt-0.5">{t('app.subtitle')}</p>
        </div>
      </div>

      <DeployWizard
        repos={availableRepos}
        loadingRepos={loadingRepos}
        branches={branches}
        onRepoSelect={handleRepoSelect}
        onDeploy={handleDeploy}
        deploying={deploying}
      />
    </div>
  );
}

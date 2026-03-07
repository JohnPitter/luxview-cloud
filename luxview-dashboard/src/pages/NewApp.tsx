import { useState, useEffect, useCallback } from 'react';
import { useNavigate } from 'react-router-dom';
import { ArrowLeft } from 'lucide-react';
import { DeployWizard, type DeployConfig } from '../components/deploy/DeployWizard';
import { PillButton } from '../components/common/PillButton';
import { useAppsStore } from '../stores/apps.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useThemeStore } from '../stores/theme.store';
import { githubApi, type GithubRepo, type GithubBranch } from '../api/github';

export function NewApp() {
  const navigate = useNavigate();
  const createApp = useAppsStore((s) => s.createApp);
  const deployApp = useAppsStore((s) => s.deployApp);
  const addNotification = useNotificationsStore((s) => s.add);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const [repos, setRepos] = useState<GithubRepo[]>([]);
  const [loadingRepos, setLoadingRepos] = useState(true);
  const [branches, setBranches] = useState<string[]>([]);
  const [deploying, setDeploying] = useState(false);

  useEffect(() => {
    githubApi
      .listRepos()
      .then(setRepos)
      .catch(() => {
        addNotification({ type: 'error', title: 'Failed to load repositories' });
      })
      .finally(() => setLoadingRepos(false));
  }, [addNotification]);

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
      // Trigger deploy immediately after creation
      await deployApp(app.id);
      addNotification({
        type: 'success',
        title: 'Deployment started',
        message: `${app.name} is being deployed...`,
      });
      navigate(`/dashboard/apps/${app.id}`);
    } catch {
      addNotification({
        type: 'error',
        title: 'Deployment failed',
        message: 'Could not create the app. Please try again.',
      });
    } finally {
      setDeploying(false);
    }
  };

  return (
    <div className="animate-fade-in">
      <div className="flex items-center gap-4 mb-8">
        <PillButton
          variant="ghost"
          size="sm"
          onClick={() => navigate('/dashboard')}
          icon={<ArrowLeft size={16} />}
        >
          Back
        </PillButton>
        <div>
          <h1
            className={`text-2xl font-bold tracking-tight ${
              isDark ? 'text-zinc-100' : 'text-zinc-900'
            }`}
          >
            Deploy New App
          </h1>
          <p className="text-sm text-zinc-500 mt-0.5">Select a repository and configure your deployment</p>
        </div>
      </div>

      <DeployWizard
        repos={repos}
        loadingRepos={loadingRepos}
        branches={branches}
        onRepoSelect={handleRepoSelect}
        onDeploy={handleDeploy}
        deploying={deploying}
      />
    </div>
  );
}

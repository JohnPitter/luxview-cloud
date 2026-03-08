import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
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
import { analyzeApi, type AnalysisResult } from '../api/analyze';
import { appsApi } from '../api/apps';
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

  // AI Analysis state
  const [analyzing, setAnalyzing] = useState(false);
  const [analysisResult, setAnalysisResult] = useState<AnalysisResult | null>(null);
  const [analysisError, setAnalysisError] = useState<string | null>(null);
  const createdAppIdRef = useRef<string | null>(null);
  const wizardEnvVarsRef = useRef<Record<string, string>>({});

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

  const deployAndNavigate = async (appId: string) => {
    setDeploying(true);
    try {
      await appsApi.deploy(appId);
      addNotification({
        type: 'success',
        title: t('app.notifications.deploymentStarted'),
        message: t('app.notifications.deploymentStartedMessage', { name: '' }),
      });
    } catch {
      addNotification({
        type: 'error',
        title: t('app.notifications.deploymentFailed'),
        message: t('app.notifications.deploymentFailedMessage'),
      });
    }
    navigate(`/dashboard/apps/${appId}`);
  };

  const runAnalysis = async (appId: string) => {
    setAnalyzing(true);
    setAnalysisError(null);
    setAnalysisResult(null);
    try {
      const result = await analyzeApi.analyze(appId);
      setAnalysisResult(result);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : '';
      const isAxios = typeof err === 'object' && err !== null && 'response' in err;
      const apiMsg = isAxios ? (err as { response?: { data?: { error?: string } } }).response?.data?.error : undefined;
      setAnalysisError(apiMsg || msg || t('analyze.aiNotConfigured'));
    } finally {
      setAnalyzing(false);
    }
  };

  const handleCreateAndAnalyze = async (config: DeployConfig) => {
    try {
      const app = await createApp({
        name: config.repo.name,
        subdomain: config.subdomain,
        repoUrl: config.repo.htmlUrl,
        repoBranch: config.branch,
        envVars: config.envVars,
        autoDeploy: false,
      });
      createdAppIdRef.current = app.id;
      wizardEnvVarsRef.current = config.envVars;
      await runAnalysis(app.id);
    } catch {
      addNotification({
        type: 'error',
        title: t('app.notifications.deploymentFailed'),
        message: t('app.notifications.deploymentFailedMessage'),
      });
    }
  };

  const handleRetryAnalysis = () => {
    const appId = createdAppIdRef.current;
    if (appId) {
      runAnalysis(appId);
    }
  };

  const handleDeploy = async (dockerfile: string, envVars: Record<string, string>, serviceModes?: Record<string, string>) => {
    const appId = createdAppIdRef.current;
    if (!appId) return;

    try {
      if (dockerfile) {
        await analyzeApi.saveDockerfile(appId, dockerfile);
      }

      // Auto-migrate services: provision + generate code changes + create PR
      if (serviceModes) {
        const autoServices = Object.entries(serviceModes).filter(([, mode]) => mode === 'auto');
        for (const [serviceType] of autoServices) {
          try {
            const result = await analyzeApi.autoMigrate(appId, serviceType);
            addNotification({
              type: 'success',
              title: t('analyze.serviceProvisioned', { service: serviceType }),
              message: result.prUrl
                ? t('analyze.prCreated')
                : result.message,
            });
            if (result.prUrl) {
              window.open(result.prUrl, '_blank');
            }
          } catch {
            addNotification({
              type: 'error',
              title: t('analyze.serviceProvisionFailed', { service: serviceType }),
            });
          }
        }
      }

      const mergedEnvVars = { ...wizardEnvVarsRef.current, ...envVars };
      const hasNewEnvVars = Object.keys(envVars).some((k) => envVars[k]);
      if (hasNewEnvVars) {
        await appsApi.updateEnvVars(appId, mergedEnvVars);
      }

      await deployAndNavigate(appId);
    } catch {
      addNotification({
        type: 'error',
        title: t('app.notifications.deploymentFailed'),
        message: t('app.notifications.deploymentFailedMessage'),
      });
    }
  };

  const handleDeployWithoutAnalysis = async () => {
    const appId = createdAppIdRef.current;
    if (!appId) return;
    await deployAndNavigate(appId);
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
        onCreateAndAnalyze={handleCreateAndAnalyze}
        onRetryAnalysis={handleRetryAnalysis}
        onDeploy={handleDeploy}
        onDeployWithoutAnalysis={handleDeployWithoutAnalysis}
        deploying={deploying}
        analysisResult={analysisResult}
        analyzing={analyzing}
        analysisError={analysisError}
      />
    </div>
  );
}

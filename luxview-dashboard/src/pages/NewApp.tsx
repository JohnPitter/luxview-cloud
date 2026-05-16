import { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import { useNavigate, useSearchParams } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import { ArrowLeft, Github, Server, GitBranch, Loader2, Plus, Rocket, Settings } from 'lucide-react';
import { DeployWizard, type DeployConfig } from '../components/deploy/DeployWizard';
import { PillButton } from '../components/common/PillButton';
import { PageTour } from '../components/common/PageTour';
import { GlassCard } from '../components/common/GlassCard';
import { useAppsStore } from '../stores/apps.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useThemeStore } from '../stores/theme.store';
import { githubApi, type GithubRepo } from '../api/github';
import { analyzeApi, aiSettingsApi, type AnalysisResult } from '../api/analyze';
import { appsApi } from '../api/apps';
import { repositoriesApi, type LuxViewRepository } from '../api/repositories';
import { newAppTourSteps } from '../tours/newApp';

type AppSource = 'github' | 'luxview';

export function NewApp() {
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { t } = useTranslation();
  const createApp = useAppsStore((s) => s.createApp);
  const apps = useAppsStore((s) => s.apps);
  const fetchApps = useAppsStore((s) => s.fetchApps);
  const addNotification = useNotificationsStore((s) => s.add);
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const initialSource = searchParams.get('source') as AppSource | null;
  const initialRepoId = searchParams.get('repoId');
  const [source, setSource] = useState<AppSource | null>(initialSource);

  // LuxView repo flow
  const [luxRepos, setLuxRepos] = useState<LuxViewRepository[]>([]);
  const [loadingLuxRepos, setLoadingLuxRepos] = useState(false);
  const [selectedLuxRepo, setSelectedLuxRepo] = useState<LuxViewRepository | null>(null);
  const [luxBranches, setLuxBranches] = useState<string[]>([]);
  const [luxBranch, setLuxBranch] = useState('');
  const [luxSubdomain, setLuxSubdomain] = useState('');
  const [deployingLux, setDeployingLux] = useState(false);

  const [repos, setRepos] = useState<GithubRepo[]>([]);
  const [loadingRepos, setLoadingRepos] = useState(false);
  const [branches, setBranches] = useState<string[]>([]);
  const [deploying, setDeploying] = useState(false);

  // AI Analysis state
  const [analyzing, setAnalyzing] = useState(false);
  const [analysisResult, setAnalysisResult] = useState<AnalysisResult | null>(null);
  const [analysisError, setAnalysisError] = useState<string | null>(null);
  const [provisioningDone, setProvisioningDone] = useState(false);
  const [aiEnabled, setAiEnabled] = useState<boolean | null>(null);
  const createdAppIdRef = useRef<string | null>(null);
  const deployedRef = useRef(false);
  const wizardEnvVarsRef = useRef<Record<string, string>>({});

  useEffect(() => {
    fetchApps();
  }, [fetchApps]);

  useEffect(() => {
    if (source !== 'github') return;
    setLoadingRepos(true);
    githubApi
      .listRepos()
      .then(setRepos)
      .catch(() => {
        addNotification({ type: 'error', title: t('app.notifications.failedToLoadRepos') });
      })
      .finally(() => setLoadingRepos(false));
  }, [source, addNotification, t]);

  useEffect(() => {
    if (source !== 'luxview') return;
    setLoadingLuxRepos(true);
    repositoriesApi
      .list()
      .then((repos) => {
        setLuxRepos(repos);
        if (initialRepoId) {
          const found = repos.find((r) => r.id === initialRepoId);
          if (found) handleSelectLuxRepo(found);
        }
      })
      .catch(() => {
        addNotification({ type: 'error', title: t('app.notifications.failedToLoadRepos') });
      })
      .finally(() => setLoadingLuxRepos(false));
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [source]);

  useEffect(() => {
    aiSettingsApi
      .get()
      .then((s) => setAiEnabled(s.aiEnabled))
      .catch(() => setAiEnabled(false));
  }, []);

  // Cleanup: delete app if user abandons the wizard without deploying
  const deleteApp = useAppsStore((s) => s.deleteApp);
  useEffect(() => {
    return () => {
      const appId = createdAppIdRef.current;
      if (appId && !deployedRef.current) {
        deleteApp(appId).catch(() => {});
      }
    };
  }, [deleteApp]);

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

  const deployAndNavigate = async (appId: string, source?: 'manual' | 'ai') => {
    deployedRef.current = true;
    setDeploying(true);
    try {
      await appsApi.deploy(appId, source);
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

  const handleApproveAnalysis = async (dockerfile: string, envVars: Record<string, string>, services: string[]) => {
    const appId = createdAppIdRef.current;
    if (!appId) return;

    setDeploying(true);
    try {
      // Apply analysis: save dockerfile + provision services
      await analyzeApi.applyAnalysis(appId, { dockerfile, envVars, services });

      // Update env vars via the app endpoint
      const mergedEnvVars = { ...wizardEnvVarsRef.current, ...envVars };
      const hasNewEnvVars = Object.keys(envVars).some((k) => envVars[k]);
      if (hasNewEnvVars) {
        await appsApi.updateEnvVars(appId, mergedEnvVars);
      }

      addNotification({
        type: 'success',
        title: t('analyze.analysisApplied'),
      });
      setProvisioningDone(true);
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

  const handleFinalDeploy = async () => {
    const appId = createdAppIdRef.current;
    if (!appId) return;
    await deployAndNavigate(appId, 'ai');
  };

  const handleDeployWithoutAnalysis = async () => {
    const appId = createdAppIdRef.current;
    if (!appId) return;
    await deployAndNavigate(appId);
  };

  const handleSelectLuxRepo = async (repo: LuxViewRepository) => {
    setSelectedLuxRepo(repo);
    setLuxBranch(repo.defaultBranch);
    setLuxSubdomain(repo.slug);
    try {
      const branches = await repositoriesApi.listBranches(repo.id);
      setLuxBranches(branches.length > 0 ? branches : [repo.defaultBranch]);
    } catch {
      setLuxBranches([repo.defaultBranch]);
    }
  };

  const handleDeployLuxView = async () => {
    if (!selectedLuxRepo) return;
    setDeployingLux(true);
    try {
      const app = await createApp({
        name: selectedLuxRepo.name,
        subdomain: luxSubdomain || selectedLuxRepo.slug,
        repositoryId: selectedLuxRepo.id,
        repoBranch: luxBranch || selectedLuxRepo.defaultBranch,
        autoDeploy: true,
      });
      await appsApi.deploy(app.id);
      addNotification({ type: 'success', title: t('app.notifications.deploymentStarted') });
      navigate(`/dashboard/apps/${app.id}`);
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.deploymentFailed') });
    } finally {
      setDeployingLux(false);
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

      {/* Source selector */}
      {!source && (
        <div className="max-w-xl mx-auto space-y-3">
          <p className={`text-sm mb-4 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
            {t('newApp.chooseSource')}
          </p>
          <GlassCard
            padding="md"
            hover
            className="cursor-pointer"
            onClick={() => setSource('luxview')}
          >
            <div className="flex items-center gap-4">
              <div className="p-2 rounded-lg bg-indigo-500/10">
                <Server size={20} className="text-indigo-400" />
              </div>
              <div>
                <p className={`text-sm font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                  {t('newApp.sourceLuxView')}
                </p>
                <p className="text-xs text-zinc-500 mt-0.5">{t('newApp.sourceLuxViewDesc')}</p>
              </div>
            </div>
          </GlassCard>
          <GlassCard
            padding="md"
            hover
            className="cursor-pointer"
            onClick={() => setSource('github')}
          >
            <div className="flex items-center gap-4">
              <div className="p-2 rounded-lg bg-zinc-500/10">
                <Github size={20} className="text-zinc-400" />
              </div>
              <div>
                <p className={`text-sm font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                  {t('newApp.sourceGitHub')}
                </p>
                <p className="text-xs text-zinc-500 mt-0.5">{t('newApp.sourceGitHubDesc')}</p>
              </div>
            </div>
          </GlassCard>
        </div>
      )}

      {/* LuxView repo flow */}
      {source === 'luxview' && (
        <div className="max-w-xl mx-auto space-y-4">
          {loadingLuxRepos ? (
            <div className="flex justify-center py-12">
              <Loader2 className="animate-spin text-zinc-400" size={24} />
            </div>
          ) : luxRepos.length === 0 ? (
            <div className="space-y-4">
              <p className="text-sm text-zinc-500">{t('newApp.noLuxRepos')}</p>
              <PillButton
                variant="primary"
                size="sm"
                icon={<Plus size={14} />}
                onClick={() => navigate('/dashboard/repositories/new')}
              >
                {t('newApp.createRepo')}
              </PillButton>
              <p className="text-xs text-zinc-500">{t('newApp.createRepoHint')}</p>
            </div>
          ) : (
            <div className="space-y-2">
              {luxRepos.map((repo) => (
                <GlassCard
                  key={repo.id}
                  padding="sm"
                  hover
                  className={`cursor-pointer border-l-2 transition-colors ${
                    selectedLuxRepo?.id === repo.id
                      ? 'border-indigo-500 bg-indigo-500/5'
                      : 'border-transparent'
                  }`}
                  onClick={() => handleSelectLuxRepo(repo)}
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-3">
                      <GitBranch size={14} className="text-zinc-500" />
                      <div>
                        <p className={`text-sm font-medium ${isDark ? 'text-white' : 'text-zinc-900'}`}>
                          {repo.name}
                        </p>
                        <p className="text-xs text-zinc-500 font-mono">{repo.slug}</p>
                      </div>
                    </div>
                    <div className="flex items-center gap-2">
                      <span className="text-xs text-zinc-500">{repo.defaultBranch}</span>
                      <button
                        onClick={(e) => { e.stopPropagation(); navigate(`/dashboard/repositories/${repo.id}`); }}
                        className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors"
                        title={t('repo.backup.title')}
                      >
                        <Settings size={13} />
                      </button>
                    </div>
                  </div>
                </GlassCard>
              ))}

              {selectedLuxRepo && (
                <div className="pt-2 space-y-3">
                  <div>
                    <label className="text-xs text-zinc-500 mb-1 block">{t('deploy.wizard.configure.branch')}</label>
                    <select
                      value={luxBranch}
                      onChange={(e) => setLuxBranch(e.target.value)}
                      className={`w-full px-3 py-2 text-sm rounded-lg border ${
                        isDark
                          ? 'bg-white/5 border-white/10 text-white'
                          : 'bg-black/5 border-black/10 text-zinc-900'
                      }`}
                    >
                      {luxBranches.map((b) => (
                        <option key={b} value={b}>{b}</option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="text-xs text-zinc-500 mb-1 block">{t('deploy.wizard.configure.subdomain')}</label>
                    <input
                      type="text"
                      value={luxSubdomain}
                      onChange={(e) => setLuxSubdomain(e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, '-'))}
                      className={`w-full px-3 py-2 text-sm rounded-lg border font-mono ${
                        isDark
                          ? 'bg-white/5 border-white/10 text-white'
                          : 'bg-black/5 border-black/10 text-zinc-900'
                      }`}
                    />
                  </div>
                  <PillButton
                    variant="primary"
                    size="md"
                    icon={deployingLux ? <Loader2 size={14} className="animate-spin" /> : <Rocket size={14} />}
                    onClick={handleDeployLuxView}
                    disabled={deployingLux || !luxSubdomain}
                  >
                    {t('app.actions.deploy')}
                  </PillButton>
                </div>
              )}
            </div>
          )}
        </div>
      )}

      {/* GitHub flow */}
      {source === 'github' && (
        <DeployWizard
          repos={availableRepos}
          loadingRepos={loadingRepos}
          branches={branches}
          onRepoSelect={handleRepoSelect}
          onCreateAndAnalyze={handleCreateAndAnalyze}
          onRetryAnalysis={handleRetryAnalysis}
          onDeploy={handleApproveAnalysis}
          onFinalDeploy={handleFinalDeploy}
          onDeployWithoutAnalysis={handleDeployWithoutAnalysis}
          deploying={deploying}
          analysisResult={analysisResult}
          analyzing={analyzing}
          analysisError={analysisError}
          provisioningDone={provisioningDone}
          aiEnabled={aiEnabled ?? false}
        />
      )}
    </div>
  );
}

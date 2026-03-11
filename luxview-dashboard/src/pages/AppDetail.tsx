import { useState, useEffect, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  ArrowLeft,
  ExternalLink,
  Play,
  Square,
  RotateCcw,
  Trash2,
  Plus,
  Eye,
  EyeOff,
  Save,
  Copy,
  Check,
  Rocket,
  Loader2,
  Stethoscope,
  X,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { StatusDot } from '../components/common/StatusDot';
import { PageTour } from '../components/common/PageTour';
import { AppStatusBadge } from '../components/apps/AppStatusBadge';
import { ConfirmDialog } from '../components/common/ConfirmDialog';
import { DeployHistory } from '../components/deploy/DeployHistory';
import { BuildLogViewer } from '../components/deploy/BuildLogViewer';
import { MetricsChart } from '../components/monitoring/MetricsChart';
import { UptimeBar } from '../components/monitoring/UptimeBar';
import { AlertConfig } from '../components/monitoring/AlertConfig';
import { RuntimeLogs } from '../components/monitoring/RuntimeLogs';
import { ServiceCard } from '../components/services/ServiceCard';
import { AddServiceDialog } from '../components/services/AddServiceDialog';
import { appDetailTourSteps } from '../tours/appDetail';
import { useAppsStore } from '../stores/apps.store';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useMetricsLive } from '../hooks/useMetricsLive';
import { formatBytes, formatPercent, formatRelativeTime } from '../lib/format';
import { deploymentsApi, type Deployment } from '../api/deployments';
import { servicesApi, type AppService, type ServiceType } from '../api/services';
import { alertsApi, type Alert, type CreateAlertPayload } from '../api/alerts';
import { appsApi } from '../api/apps';
import { analyzeApi, type AnalysisResult } from '../api/analyze';
import { DeployAnalysis } from '../components/deploy/DeployAnalysis';

type Tab = 'overview' | 'deployments' | 'logs' | 'env' | 'services' | 'metrics' | 'alerts' | 'settings';

export function AppDetail() {
  const { t } = useTranslation();
  const { appId } = useParams<{ appId: string }>();
  const navigate = useNavigate();
  const { selectedApp: app, fetchApp, deployApp, stopApp, restartApp, deleteApp } = useAppsStore();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const tabs: Array<{ id: Tab; label: string }> = [
    { id: 'overview', label: t('app.tabs.overview') },
    { id: 'deployments', label: t('app.tabs.deployments') },
    { id: 'logs', label: t('app.tabs.logs') },
    { id: 'env', label: t('app.tabs.environment') },
    { id: 'services', label: t('app.tabs.services') },
    { id: 'metrics', label: t('app.tabs.metrics') },
    { id: 'alerts', label: t('app.tabs.alerts') },
    { id: 'settings', label: t('app.tabs.settings') },
  ];

  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [services, setServices] = useState<AppService[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [showServiceDialog, setShowServiceDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);
  const [selectedBuildLog, setSelectedBuildLog] = useState<string>('');
  const [selectedDeployId, setSelectedDeployId] = useState<string>('');
  const [actionPending, setActionPending] = useState(false);
  const [logType, setLogType] = useState<'runtime' | 'build'>('runtime');
  const [notFound, setNotFound] = useState(false);

  // Env vars state
  const [envVars, setEnvVars] = useState<Array<{ key: string; value: string }>>([]);
  const [showValues, setShowValues] = useState(false);
  const [savingEnv, setSavingEnv] = useState(false);

  // Settings state
  const [settingsName, setSettingsName] = useState('');
  const [settingsBranch, setSettingsBranch] = useState('');
  const [settingsAutoDeploy, setSettingsAutoDeploy] = useState(true);

  // Failure analysis state
  const [analyzingFailure, setAnalyzingFailure] = useState(false);
  const [failureAnalysisResult, setFailureAnalysisResult] = useState<AnalysisResult | null>(null);
  const [showAnalysisModal, setShowAnalysisModal] = useState(false);

  const { metrics } = useMetricsLive(appId || '');

  useEffect(() => {
    if (appId) {
      fetchApp(appId).catch(() => setNotFound(true));
    }
  }, [appId, fetchApp]);

  // Track whether the deploy has entered a transitional state (building/deploying)
  const sawTransitionalRef = useRef(false);

  // Track previous status to detect transitions
  const prevStatusRef = useRef(app?.status);

  // Poll app status + deployments + build logs every 3s during transitional states
  useEffect(() => {
    if (!appId || !app) return;
    const transitional = ['building', 'deploying'];
    const shouldPoll = actionPending || transitional.includes(app.status);
    if (!shouldPoll) return;

    const poll = async () => {
      fetchApp(appId);
      try {
        const latest = await deploymentsApi.list(appId, 5, 0);
        // Merge latest statuses into full list instead of replacing it
        setDeployments((prev: Deployment[]) => {
          if (prev.length === 0) return latest;
          const map = new Map(latest.map((d: Deployment) => [d.id, d]));
          const merged = prev.map((d: Deployment) => map.get(d.id) || d);
          // Add any new deployments not in prev (e.g. new deploy just started)
          for (const d of latest) {
            if (!prev.some((p: Deployment) => p.id === d.id)) {
              merged.unshift(d);
            }
          }
          return merged;
        });
        const deps = latest;

        // Auto-select the active deployment (building/deploying) and poll its logs
        const activeDeploy = deps.find((d) => transitional.includes(d.status));
        if (activeDeploy) {
          setSelectedDeployId(activeDeploy.id);
          const res = await deploymentsApi.getLogs(activeDeploy.id);
          setSelectedBuildLog(res.buildLog || '');
        } else if (deps.length > 0 && (!selectedDeployId || selectedDeployId !== deps[0].id)) {
          // Build finished — load final logs from latest deployment
          setSelectedDeployId(deps[0].id);
          const res = await deploymentsApi.getLogs(deps[0].id);
          setSelectedBuildLog(res.buildLog || '');
        }
      } catch { /* ignore */ }
    };
    poll(); // immediate first poll
    const interval = setInterval(poll, 3000);

    // Track transitional states and clear actionPending when deploy cycle completes
    if (actionPending) {
      if (transitional.includes(app.status)) {
        sawTransitionalRef.current = true;
      } else if (sawTransitionalRef.current) {
        // Exited transitional → deploy finished (running, error, stopped, etc.)
        setActionPending(false);
        sawTransitionalRef.current = false;
      }
    }

    return () => clearInterval(interval);
  }, [appId, app?.status, actionPending, fetchApp]);

  // Background poll: check for status changes every 15s (catches webhook-triggered deploys)
  useEffect(() => {
    if (!appId || !app) return;
    const transitional = ['building', 'deploying'];
    // Skip if already fast-polling
    if (actionPending || transitional.includes(app.status)) return;

    const bgPoll = () => fetchApp(appId);
    const interval = setInterval(bgPoll, 15000);
    return () => clearInterval(interval);
  }, [appId, app?.status, actionPending, fetchApp]);

  // Detect status transitions and notify user
  useEffect(() => {
    if (!app) return;
    const prev = prevStatusRef.current;
    prevStatusRef.current = app.status;
    if (!prev || prev === app.status) return;

    const transitional = ['building', 'deploying'];
    // Transitioned from building/deploying to a final state
    if (transitional.includes(prev) && !transitional.includes(app.status)) {
      if (app.status === 'running') {
        addNotification({ type: 'success', title: t('app.notifications.deploySuccess') });
      } else if (app.status === 'error') {
        addNotification({ type: 'error', title: t('app.notifications.deployFailed') });
      }
    }
  }, [app?.status]);

  // Clear old build logs when a new deploy is triggered
  useEffect(() => {
    if (app && ['building', 'deploying'].includes(app.status)) {
      // Auto-switch to build logs view so user sees progress
      if (activeTab === 'logs') {
        setLogType('build');
      }
    }
  }, [app?.status]);

  useEffect(() => {
    if (app) {
      setEnvVars(
        Object.entries(app.envVars || {}).map(([key, value]) => ({ key, value })),
      );
      setSettingsName(app.name);
      setSettingsBranch(app.repoBranch);
      setSettingsAutoDeploy(app.autoDeploy);
    }
  }, [app]);

  useEffect(() => {
    if (appId && (activeTab === 'deployments' || activeTab === 'logs')) {
      deploymentsApi.list(appId).then((deps) => {
        setDeployments(deps);
        // Auto-load build log from latest deployment for Logs tab
        if (activeTab === 'logs' && deps.length > 0 && !selectedDeployId) {
          const target = deps[0];
          setSelectedDeployId(target.id);
          deploymentsApi.getLogs(target.id).then((res) => {
            setSelectedBuildLog(res.buildLog || '');
          }).catch(() => {});
        }
      }).catch(() => {});
    }
    if (appId && activeTab === 'services') {
      servicesApi.list(appId).then(setServices).catch(() => {});
    }
    if (appId && activeTab === 'alerts') {
      alertsApi.list(appId).then(setAlerts).catch(() => {});
    }
  }, [appId, activeTab]);

  const handleDelete = async () => {
    if (!appId) return;
    setDeleting(true);
    try {
      await deleteApp(appId);
      addNotification({ type: 'success', title: t('app.notifications.appDeleted') });
      navigate('/dashboard');
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.appDeleteFailed') });
    } finally {
      setDeleting(false);
      setShowDeleteDialog(false);
    }
  };

  const handleSaveEnv = async () => {
    if (!appId) return;
    setSavingEnv(true);
    try {
      const envRecord: Record<string, string> = {};
      envVars.forEach((e) => {
        if (e.key.trim()) envRecord[e.key.trim()] = e.value;
      });
      await appsApi.updateEnvVars(appId, envRecord);
      addNotification({ type: 'success', title: t('app.notifications.envSaved') });
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.envSaveFailed') });
    } finally {
      setSavingEnv(false);
    }
  };

  const handleSaveSettings = async () => {
    if (!appId) return;
    try {
      await appsApi.update(appId, {
        name: settingsName,
        repoBranch: settingsBranch,
        autoDeploy: settingsAutoDeploy,
      });
      addNotification({ type: 'success', title: t('app.notifications.settingsSaved') });
      fetchApp(appId);
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.settingsSaveFailed') });
    }
  };

  const handleAnalyzeFailure = async () => {
    if (!appId) return;
    setAnalyzingFailure(true);
    setShowAnalysisModal(true);
    setFailureAnalysisResult(null);
    try {
      const result = await analyzeApi.analyzeFailure(appId);
      setFailureAnalysisResult(result);
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.analysisFailed') });
      setShowAnalysisModal(false);
    } finally {
      setAnalyzingFailure(false);
    }
  };

  const handleApproveFailureAnalysis = async (dockerfile: string, envVars: Record<string, string>) => {
    if (!appId) return;
    try {
      if (dockerfile) {
        await analyzeApi.saveDockerfile(appId, dockerfile);
      }
      const hasEnvVars = Object.keys(envVars).some((k) => envVars[k]);
      if (hasEnvVars) {
        const merged = { ...(app?.envVars || {}), ...envVars };
        await appsApi.updateEnvVars(appId, merged);
      }
      sawTransitionalRef.current = false;
      setActionPending(true);
      await deployApp(appId);
      setShowAnalysisModal(false);
      setFailureAnalysisResult(null);
    } catch {
      addNotification({ type: 'error', title: t('app.notifications.deploymentFailed') });
    }
  };

  const handleDismissFailureAnalysis = () => {
    setShowAnalysisModal(false);
    setFailureAnalysisResult(null);
  };

  const latestMetric = metrics.length > 0 ? metrics[metrics.length - 1] : null;
  const currentCpu = latestMetric?.cpuPercent ?? 0;
  const currentMemory = latestMetric?.memoryBytes ?? 0;

  const metricsData = metrics.map((m) => ({
    time: new Date(m.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    cpu: m.cpuPercent,
    memory: m.memoryBytes / (1024 * 1024),
    networkRx: m.networkRx / 1024,
    networkTx: m.networkTx / 1024,
  }));

  // Mock uptime data
  const uptimeDays = Array.from({ length: 30 }, (_, i) => ({
    date: new Date(Date.now() - (29 - i) * 86400000).toLocaleDateString(),
    status: (Math.random() > 0.05 ? 'up' : 'partial') as 'up' | 'partial',
  }));

  const inputClass = `
    w-full px-4 py-2.5 rounded-xl text-sm border transition-all duration-200
    focus:outline-none focus:ring-2 focus:ring-amber-400/30
    ${isDark ? 'bg-zinc-900/50 border-zinc-800 text-zinc-100 placeholder-zinc-600' : 'bg-white border-zinc-200 text-zinc-900 placeholder-zinc-400'}
  `;

  if (notFound) {
    return (
      <div className="flex flex-col items-center justify-center py-32 animate-fade-in">
        {/* Animated ghost icon */}
        <div className="relative mb-8">
          <div
            className={`w-24 h-24 rounded-3xl flex items-center justify-center ${
              isDark ? 'bg-zinc-800/60' : 'bg-zinc-100'
            }`}
            style={{ animation: 'float 3s ease-in-out infinite' }}
          >
            <svg
              width="48"
              height="48"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
              className="text-zinc-400"
            >
              <path d="M12 2a7 7 0 0 0-7 7v8l2-2 2 2 2-2 2 2 2-2 2 2 2-2v-8a7 7 0 0 0-7-7z" />
              <circle cx="10" cy="10" r="1" fill="currentColor" />
              <circle cx="14" cy="10" r="1" fill="currentColor" />
            </svg>
          </div>
          {/* Pulse ring */}
          <div
            className={`absolute inset-0 rounded-3xl ${
              isDark ? 'bg-amber-400/10' : 'bg-amber-400/5'
            }`}
            style={{ animation: 'ping 2s cubic-bezier(0, 0, 0.2, 1) infinite' }}
          />
        </div>
        <h2
          className={`text-2xl font-bold tracking-tight mb-3 ${
            isDark ? 'text-zinc-100' : 'text-zinc-900'
          }`}
        >
          {t('app.notFound.title')}
        </h2>
        <p
          className={`text-sm max-w-md text-center mb-2 ${
            isDark ? 'text-zinc-400' : 'text-zinc-500'
          }`}
        >
          {t('app.notFound.description')}
        </p>
        <p
          className={`text-xs max-w-sm text-center mb-8 ${
            isDark ? 'text-zinc-500' : 'text-zinc-400'
          }`}
        >
          {t('app.notFound.support')}
        </p>
        <PillButton
          variant="secondary"
          size="md"
          onClick={() => navigate('/dashboard')}
          icon={<ArrowLeft size={16} />}
        >
          {t('app.notFound.backToDashboard')}
        </PillButton>
        <style>{`
          @keyframes float {
            0%, 100% { transform: translateY(0px); }
            50% { transform: translateY(-10px); }
          }
        `}</style>
      </div>
    );
  }

  if (!app) {
    return (
      <div className="flex items-center justify-center py-32">
        <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      <PageTour tourId="appDetail" steps={appDetailTourSteps} autoStart />
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <PillButton
            variant="ghost"
            size="sm"
            onClick={() => navigate('/dashboard')}
            icon={<ArrowLeft size={16} />}
          >
            {t('common.back')}
          </PillButton>
          <div className="flex items-center gap-3">
            <StatusDot status={app.status} size="lg" />
            <div>
              <h1
                className={`text-xl font-bold tracking-tight ${
                  isDark ? 'text-zinc-100' : 'text-zinc-900'
                }`}
              >
                {app.name}
              </h1>
              <a
                href={`https://${app.subdomain}.luxview.cloud`}
                target="_blank"
                rel="noopener noreferrer"
                className="flex items-center gap-1 text-xs text-zinc-500 hover:text-amber-400 transition-colors"
              >
                {app.subdomain}.luxview.cloud
                <ExternalLink size={10} />
              </a>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {app.status === 'stopped' && (
            <PillButton
              variant="secondary"
              size="sm"
              disabled={actionPending}
              onClick={() => { sawTransitionalRef.current = false; setActionPending(true); deployApp(appId!).catch(() => setActionPending(false)); }}
              icon={<Play size={14} />}
            >
              {t('app.actions.start')}
            </PillButton>
          )}
          {app.status === 'running' && (
            <>
              <PillButton
                variant="secondary"
                size="sm"
                disabled={actionPending || ['building', 'deploying'].includes(app.status)}
                onClick={async () => { setActionPending(true); await restartApp(appId!); setActionPending(false); }}
                icon={<RotateCcw size={14} />}
              >
                {t('app.actions.restart')}
              </PillButton>
              <PillButton
                variant="ghost"
                size="sm"
                disabled={actionPending || ['building', 'deploying'].includes(app.status)}
                onClick={async () => { setActionPending(true); await stopApp(appId!); setActionPending(false); }}
                icon={<Square size={14} />}
              >
                {t('app.actions.stop')}
              </PillButton>
            </>
          )}
          {app.status === 'error' && (
            <PillButton
              variant="ghost"
              size="sm"
              disabled={analyzingFailure}
              onClick={handleAnalyzeFailure}
              icon={analyzingFailure ? <Loader2 size={14} className="animate-spin" /> : <Stethoscope size={14} />}
            >
              {t('app.actions.analyzeFailure')}
            </PillButton>
          )}
          <PillButton
            variant="primary"
            size="sm"
            disabled={actionPending || ['building', 'deploying'].includes(app.status)}
            onClick={() => { sawTransitionalRef.current = false; setActionPending(true); deployApp(appId!).catch(() => setActionPending(false)); }}
            icon={['building', 'deploying'].includes(app.status) || actionPending
              ? <Loader2 size={14} className="animate-spin" />
              : <Rocket size={14} />}
          >
            {['building', 'deploying'].includes(app.status) ? t('app.actions.deploying') : t('app.actions.deploy')}
          </PillButton>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1 mb-6 overflow-x-auto pb-2">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`
              px-4 py-2 text-sm font-medium rounded-xl whitespace-nowrap
              transition-all duration-200
              ${
                activeTab === tab.id
                  ? 'bg-amber-400/10 text-amber-400'
                  : isDark
                    ? 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50'
                    : 'text-zinc-400 hover:text-zinc-700 hover:bg-zinc-100'
              }
            `}
          >
            {tab.label}
          </button>
        ))}
      </div>

      {/* Tab Content */}
      <div className="animate-fade-in">
        {/* ==================== OVERVIEW ==================== */}
        {activeTab === 'overview' && (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4" data-tour="app-overview">
            {/* Status Card */}
            <GlassCard className="lg:col-span-2">
              <h3 className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {t('app.info.title')}
              </h3>
              <div className="grid grid-cols-2 gap-4">
                {[
                  { label: t('app.info.status'), value: <AppStatusBadge status={app.status} size="md" /> },
                  { label: t('app.info.stack'), value: app.stack },
                  { label: t('app.info.branch'), value: app.repoBranch },
                  { label: t('app.info.autoDeploy'), value: app.autoDeploy ? t('app.info.autoDeployEnabled') : t('app.info.autoDeployDisabled') },
                  { label: t('app.info.created'), value: formatRelativeTime(app.createdAt) },
                  { label: t('app.info.lastUpdated'), value: formatRelativeTime(app.updatedAt) },
                ].map(({ label, value }) => (
                  <div key={label}>
                    <span className="text-[11px] text-zinc-500 uppercase tracking-wider font-medium">
                      {label}
                    </span>
                    <div className={`text-sm mt-1 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {value}
                    </div>
                  </div>
                ))}
              </div>
            </GlassCard>

            {/* Resource Usage */}
            <GlassCard>
              <h3 className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                {t('app.resources.title')}
              </h3>
              <div className="space-y-4">
                {/* CPU Gauge */}
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span className="text-zinc-500">{t('app.resources.cpu')}</span>
                    <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>
                      {formatPercent(currentCpu)}
                    </span>
                  </div>
                  <div className={`h-2 rounded-full ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                    <div
                      className="h-full rounded-full bg-amber-400 transition-all duration-500"
                      style={{ width: `${Math.min(currentCpu, 100)}%` }}
                    />
                  </div>
                </div>
                {/* RAM Gauge */}
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span className="text-zinc-500">{t('app.resources.memory')}</span>
                    <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>
                      {formatBytes(currentMemory)}
                    </span>
                  </div>
                  <div className={`h-2 rounded-full ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                    <div
                      className="h-full rounded-full bg-blue-400 transition-all duration-500"
                      style={{
                        width: `${Math.min(
                          (currentMemory / (parseInt(app.resourceLimits?.memory || '512') * 1024 * 1024)) * 100,
                          100,
                        )}%`,
                      }}
                    />
                  </div>
                </div>
                {/* Limits */}
                <div className="pt-2 border-t border-zinc-800/50 space-y-1">
                  <div className="flex justify-between text-[11px]">
                    <span className="text-zinc-500">{t('app.resources.cpuLimit')}</span>
                    <span className="text-zinc-400">{app.resourceLimits?.cpu || '0.5'} cores</span>
                  </div>
                  <div className="flex justify-between text-[11px]">
                    <span className="text-zinc-500">{t('app.resources.memoryLimit')}</span>
                    <span className="text-zinc-400">{(app.resourceLimits?.memory || '512m').replace(/[mg]/i, '')} MB</span>
                  </div>
                  <div className="flex justify-between text-[11px]">
                    <span className="text-zinc-500">{t('app.resources.diskLimit')}</span>
                    <span className="text-zinc-400">{(app.resourceLimits?.disk || '1g').replace(/[mg]/i, '')} GB</span>
                  </div>
                </div>
              </div>
            </GlassCard>

            {/* Uptime */}
            <GlassCard className="lg:col-span-3">
              <UptimeBar days={uptimeDays} uptimePercent={99.87} />
            </GlassCard>
          </div>
        )}

        {/* ==================== DEPLOYMENTS ==================== */}
        {activeTab === 'deployments' && (
          <div data-tour="app-deploys">
            <DeployHistory
              deployments={deployments}
              onRollback={async (deployId) => {
                try {
                  await deploymentsApi.rollback(appId!, deployId);
                  addNotification({ type: 'success', title: t('app.notifications.rollbackInitiated') });
                  const updated = await deploymentsApi.list(appId!);
                  setDeployments(updated);
                } catch {
                  addNotification({ type: 'error', title: t('app.notifications.rollbackFailed') });
                }
              }}
              onViewLog={(deployId) => {
                deploymentsApi.getLogs(deployId).then((res) => {
                  setSelectedBuildLog(res.buildLog || '');
                  setActiveTab('logs');
                }).catch(() => {
                  setActiveTab('logs');
                });
              }}
            />
          </div>
        )}

        {/* ==================== LOGS ==================== */}
        {activeTab === 'logs' && (
          <div className="space-y-4" data-tour="app-logs">
            {/* Log type toggle */}
            <div className="flex items-center gap-2">
              <button
                onClick={() => setLogType('runtime')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all ${
                  logType === 'runtime'
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                    : isDark ? 'text-zinc-400 hover:text-zinc-200 border border-zinc-800' : 'text-zinc-600 hover:text-zinc-900 border border-zinc-200'
                }`}
              >
                {t('app.logs.runtimeLogs')}
              </button>
              <button
                onClick={() => setLogType('build')}
                className={`px-3 py-1.5 text-xs font-medium rounded-lg transition-all ${
                  logType === 'build'
                    ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                    : isDark ? 'text-zinc-400 hover:text-zinc-200 border border-zinc-800' : 'text-zinc-600 hover:text-zinc-900 border border-zinc-200'
                }`}
              >
                {t('app.logs.buildLogs')}
              </button>
            </div>

            {/* Runtime logs */}
            {logType === 'runtime' && appId && (
              <RuntimeLogs appId={appId} containerId={app?.containerId} />
            )}

            {/* Build logs */}
            {logType === 'build' && (
              <>
                {deployments.length > 0 && (
                  <div className="flex items-center gap-3">
                    <label className="text-xs text-zinc-500">{t('app.logs.deployment')}:</label>
                    <select
                      value={selectedDeployId || deployments[0]?.id || ''}
                      onChange={(e: React.ChangeEvent<HTMLSelectElement>) => {
                        const deployId = e.target.value;
                        if (deployId) {
                          setSelectedDeployId(deployId);
                          deploymentsApi.getLogs(deployId).then((res: { buildLog: string }) => {
                            setSelectedBuildLog(res.buildLog || '');
                          }).catch(() => {});
                        }
                      }}
                      className={`
                        px-3 py-1.5 text-xs font-mono rounded-lg border transition-all
                        focus:outline-none focus:ring-2 focus:ring-amber-400/30
                        ${isDark ? 'bg-zinc-900/50 border-zinc-800 text-zinc-300' : 'bg-white border-zinc-200 text-zinc-700'}
                      `}
                    >
                      {deployments.map((d: Deployment) => (
                        <option key={d.id} value={d.id}>
                          {d.commitSha.slice(0, 7)} — {d.status} — {d.commitMessage?.slice(0, 40) || t('app.logs.noMessage')}
                        </option>
                      ))}
                    </select>
                    {/* Live indicator when actively building */}
                    {deployments.some((d) => ['building', 'deploying'].includes(d.status)) && (
                      <span className="flex items-center gap-1.5 text-[11px] text-amber-400 font-medium">
                        <span className="relative flex h-2 w-2">
                          <span className="animate-ping absolute inline-flex h-full w-full rounded-full bg-amber-400 opacity-75" />
                          <span className="relative inline-flex rounded-full h-2 w-2 bg-amber-400" />
                        </span>
                        {t('app.logs.streaming')}
                      </span>
                    )}
                  </div>
                )}
                <BuildLogViewer
                  log={selectedBuildLog}
                  streaming={deployments.some((d) => ['building', 'deploying'].includes(d.status))}
                  loading={!selectedBuildLog && deployments.some((d) => ['building', 'deploying'].includes(d.status))}
                />
                {!selectedBuildLog && deployments.length === 0 && (
                  <div className="text-center py-12 text-zinc-500 text-sm">
                    {t('app.logs.noBuildLogs')}
                  </div>
                )}
              </>
            )}
          </div>
        )}

        {/* ==================== ENVIRONMENT ==================== */}
        {activeTab === 'env' && (
          <GlassCard>
            <div className="flex items-center justify-between mb-4">
              <h3
                className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                {t('app.env.title')}
              </h3>
              <div className="flex items-center gap-2">
                <PillButton
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowValues(!showValues)}
                  icon={showValues ? <EyeOff size={14} /> : <Eye size={14} />}
                >
                  {showValues ? t('app.env.hideValues') : t('app.env.showValues')}
                </PillButton>
                <PillButton
                  variant="ghost"
                  size="sm"
                  onClick={() => setEnvVars([...envVars, { key: '', value: '' }])}
                  icon={<Plus size={14} />}
                >
                  {t('app.env.addVariable')}
                </PillButton>
              </div>
            </div>

            {envVars.length === 0 ? (
              <p className="text-sm text-zinc-500 text-center py-8">
                {t('app.env.noVariables')}
              </p>
            ) : (
              <div className="space-y-2 mb-4">
                {envVars.map((env, i) => (
                  <div key={i} className="flex items-center gap-2">
                    <input
                      type="text"
                      value={env.key}
                      onChange={(e) => {
                        const updated = [...envVars];
                        updated[i] = { ...updated[i], key: e.target.value };
                        setEnvVars(updated);
                      }}
                      placeholder={t('app.env.keyPlaceholder')}
                      className={`${inputClass} flex-1 font-mono text-xs !py-2`}
                    />
                    <input
                      type={showValues ? 'text' : 'password'}
                      value={env.value}
                      onChange={(e) => {
                        const updated = [...envVars];
                        updated[i] = { ...updated[i], value: e.target.value };
                        setEnvVars(updated);
                      }}
                      placeholder={t('app.env.valuePlaceholder')}
                      className={`${inputClass} flex-1 font-mono text-xs !py-2`}
                    />
                    <button
                      onClick={() => setEnvVars(envVars.filter((_, idx) => idx !== i))}
                      className="p-2 text-zinc-500 hover:text-red-400 transition-colors"
                    >
                      <Trash2 size={14} />
                    </button>
                  </div>
                ))}
              </div>
            )}

            <div className="flex justify-end">
              <PillButton
                variant="primary"
                size="sm"
                onClick={handleSaveEnv}
                disabled={savingEnv}
                icon={<Save size={14} />}
              >
                {savingEnv ? t('app.env.saving') : t('app.env.saveChanges')}
              </PillButton>
            </div>
          </GlassCard>
        )}

        {/* ==================== SERVICES ==================== */}
        {activeTab === 'services' && (
          <div data-tour="app-services">
            <div className="flex items-center justify-between mb-4">
              <h3
                className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                {t('app.services.title')}
              </h3>
              <PillButton
                variant="ghost"
                size="sm"
                onClick={() => setShowServiceDialog(true)}
                icon={<Plus size={14} />}
              >
                {t('app.services.addService')}
              </PillButton>
            </div>

            {services.length === 0 ? (
              <GlassCard>
                <p className="text-sm text-zinc-500 text-center py-8">
                  {t('app.services.noServices')}
                </p>
              </GlassCard>
            ) : (
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {services.map((svc) => (
                  <ServiceCard
                    key={svc.id}
                    service={svc}
                    onDelete={async (serviceId) => {
                      try {
                        await servicesApi.delete(serviceId);
                        setServices(services.filter((s) => s.id !== serviceId));
                        addNotification({ type: 'success', title: t('app.notifications.serviceRemoved') });
                      } catch {
                        addNotification({ type: 'error', title: t('app.notifications.serviceRemoveFailed') });
                      }
                    }}
                  />
                ))}
              </div>
            )}

            <AddServiceDialog
              open={showServiceDialog}
              onClose={() => setShowServiceDialog(false)}
              existingTypes={services.map((s) => s.serviceType)}
              onAdd={async (type) => {
                try {
                  const svc = await servicesApi.create(appId!, type);
                  setServices([...services, svc]);
                  setShowServiceDialog(false);
                  addNotification({ type: 'success', title: t('app.notifications.serviceAdded') });
                } catch {
                  addNotification({ type: 'error', title: t('app.notifications.serviceAddFailed') });
                }
              }}
            />
          </div>
        )}

        {/* ==================== METRICS ==================== */}
        {activeTab === 'metrics' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4" data-tour="app-metrics">
            <MetricsChart
              data={metricsData}
              dataKey="cpu"
              title={t('app.metrics.cpuUsage')}
              color="#fbbf24"
              unit="%"
            />
            <MetricsChart
              data={metricsData}
              dataKey="memory"
              title={t('app.metrics.memoryUsage')}
              color="#60a5fa"
              formatter={(v) => `${v.toFixed(0)}MB`}
            />
            <MetricsChart
              data={metricsData}
              dataKey="networkRx"
              title={t('app.metrics.networkIn')}
              color="#34d399"
              unit=" KB/s"
            />
            <MetricsChart
              data={metricsData}
              dataKey="networkTx"
              title={t('app.metrics.networkOut')}
              color="#a78bfa"
              unit=" KB/s"
            />
          </div>
        )}

        {/* ==================== ALERTS ==================== */}
        {activeTab === 'alerts' && (
          <AlertConfig
            alerts={alerts}
            onCreateAlert={async (payload) => {
              try {
                const alert = await alertsApi.create(appId!, payload);
                setAlerts([...alerts, alert]);
                addNotification({ type: 'success', title: t('app.notifications.alertCreated') });
              } catch {
                addNotification({ type: 'error', title: t('app.notifications.alertCreateFailed') });
              }
            }}
            onDeleteAlert={async (alertId) => {
              try {
                await alertsApi.delete(appId!, alertId);
                setAlerts(alerts.filter((a) => a.id !== alertId));
                addNotification({ type: 'success', title: t('app.notifications.alertDeleted') });
              } catch {
                addNotification({ type: 'error', title: t('app.notifications.alertDeleteFailed') });
              }
            }}
            onToggleAlert={async (alertId, enabled) => {
              try {
                await alertsApi.update(appId!, alertId, { enabled });
                setAlerts(alerts.map((a) => (a.id === alertId ? { ...a, enabled } : a)));
              } catch {
                addNotification({ type: 'error', title: t('app.notifications.alertUpdateFailed') });
              }
            }}
          />
        )}

        {/* ==================== SETTINGS ==================== */}
        {activeTab === 'settings' && (
          <div className="space-y-6 max-w-2xl mx-auto">
            <GlassCard>
              <h3
                className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                {t('app.settings.title')}
              </h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-xs text-zinc-500 mb-1.5">{t('app.settings.appName')}</label>
                  <input
                    type="text"
                    value={settingsName}
                    onChange={(e) => setSettingsName(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="block text-xs text-zinc-500 mb-1.5">{t('app.settings.branch')}</label>
                  <input
                    type="text"
                    value={settingsBranch}
                    onChange={(e) => setSettingsBranch(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div className="flex items-center justify-between">
                  <div>
                    <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {t('app.settings.autoDeploy')}
                    </p>
                    <p className="text-[11px] text-zinc-500">
                      {t('app.settings.autoDeployDescription')}
                    </p>
                  </div>
                  <button
                    onClick={() => setSettingsAutoDeploy(!settingsAutoDeploy)}
                    className={`
                      w-11 h-6 rounded-full transition-all duration-200 relative
                      ${settingsAutoDeploy ? 'bg-amber-400' : isDark ? 'bg-zinc-700' : 'bg-zinc-300'}
                    `}
                  >
                    <span
                      className={`
                        absolute top-0.5 w-5 h-5 rounded-full bg-white shadow-sm transition-all duration-200
                        ${settingsAutoDeploy ? 'left-[22px]' : 'left-0.5'}
                      `}
                    />
                  </button>
                </div>
              </div>
              <div className="flex justify-end mt-6">
                <PillButton variant="primary" size="sm" onClick={handleSaveSettings} icon={<Save size={14} />}>
                  {t('app.settings.saveSettings')}
                </PillButton>
              </div>
            </GlassCard>

            {/* Danger Zone */}
            <GlassCard className="!border-red-500/20">
              <h3 className="text-sm font-semibold text-red-400 mb-4">{t('app.settings.dangerZone')}</h3>
              <div className="flex items-center justify-between">
                <div>
                  <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                    {t('app.settings.deleteApp')}
                  </p>
                  <p className="text-[11px] text-zinc-500">
                    {t('app.settings.deleteAppDescription')}
                  </p>
                </div>
                <PillButton
                  variant="danger"
                  size="sm"
                  onClick={() => setShowDeleteDialog(true)}
                  icon={<Trash2 size={14} />}
                >
                  {t('app.settings.deleteAppButton')}
                </PillButton>
              </div>
            </GlassCard>
          </div>
        )}
      </div>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={showDeleteDialog}
        title={t('app.deleteDialog.title')}
        message={t('app.deleteDialog.message', { name: app.name })}
        confirmLabel={t('app.deleteDialog.confirm')}
        onConfirm={handleDelete}
        onCancel={() => setShowDeleteDialog(false)}
        loading={deleting}
      />

      {/* Failure Analysis Modal */}
      {showAnalysisModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center">
          <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={handleDismissFailureAnalysis} />
          <div className="relative z-10 w-full max-w-2xl max-h-[90vh] overflow-y-auto m-4">
            {!analyzingFailure && failureAnalysisResult && (
              <button
                onClick={handleDismissFailureAnalysis}
                className="absolute top-4 right-4 z-20 text-zinc-500 hover:text-zinc-300 transition-colors"
              >
                <X size={18} />
              </button>
            )}
            <DeployAnalysis
              result={failureAnalysisResult ?? { suggestions: [], dockerfile: '', port: 0, stack: '', envHints: [] }}
              loading={analyzingFailure}
              mode="failure"
              onApprove={handleApproveFailureAnalysis}
              onSkip={handleDismissFailureAnalysis}
            />
          </div>
        </div>
      )}
    </div>
  );
}

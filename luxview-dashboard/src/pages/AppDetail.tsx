import { useState, useEffect, useCallback } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
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
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { StatusDot } from '../components/common/StatusDot';
import { AppStatusBadge } from '../components/apps/AppStatusBadge';
import { ConfirmDialog } from '../components/common/ConfirmDialog';
import { DeployHistory } from '../components/deploy/DeployHistory';
import { BuildLogViewer } from '../components/deploy/BuildLogViewer';
import { LogViewer } from '../components/monitoring/LogViewer';
import { MetricsChart } from '../components/monitoring/MetricsChart';
import { UptimeBar } from '../components/monitoring/UptimeBar';
import { AlertConfig } from '../components/monitoring/AlertConfig';
import { ServiceCard } from '../components/services/ServiceCard';
import { AddServiceDialog } from '../components/services/AddServiceDialog';
import { useAppsStore } from '../stores/apps.store';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { useDeployLogs } from '../hooks/useDeployLogs';
import { useMetricsLive } from '../hooks/useMetricsLive';
import { formatBytes, formatPercent, formatRelativeTime } from '../lib/format';
import { deploymentsApi, type Deployment } from '../api/deployments';
import { servicesApi, type AppService, type ServiceType } from '../api/services';
import { alertsApi, type Alert, type CreateAlertPayload } from '../api/alerts';
import { appsApi } from '../api/apps';

type Tab = 'overview' | 'deployments' | 'logs' | 'env' | 'services' | 'metrics' | 'alerts' | 'settings';

const tabs: Array<{ id: Tab; label: string }> = [
  { id: 'overview', label: 'Overview' },
  { id: 'deployments', label: 'Deployments' },
  { id: 'logs', label: 'Logs' },
  { id: 'env', label: 'Environment' },
  { id: 'services', label: 'Services' },
  { id: 'metrics', label: 'Metrics' },
  { id: 'alerts', label: 'Alerts' },
  { id: 'settings', label: 'Settings' },
];

export function AppDetail() {
  const { appId } = useParams<{ appId: string }>();
  const navigate = useNavigate();
  const { selectedApp: app, fetchApp, deployApp, stopApp, restartApp, deleteApp } = useAppsStore();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [deployments, setDeployments] = useState<Deployment[]>([]);
  const [services, setServices] = useState<AppService[]>([]);
  const [alerts, setAlerts] = useState<Alert[]>([]);
  const [showDeleteDialog, setShowDeleteDialog] = useState(false);
  const [showServiceDialog, setShowServiceDialog] = useState(false);
  const [deleting, setDeleting] = useState(false);

  // Env vars state
  const [envVars, setEnvVars] = useState<Array<{ key: string; value: string }>>([]);
  const [showValues, setShowValues] = useState(false);
  const [savingEnv, setSavingEnv] = useState(false);

  // Settings state
  const [settingsName, setSettingsName] = useState('');
  const [settingsBranch, setSettingsBranch] = useState('');
  const [settingsAutoDeploy, setSettingsAutoDeploy] = useState(true);

  const { logs, clear: clearLogs } = useDeployLogs(appId || '');
  const { metrics } = useMetricsLive(appId || '');

  useEffect(() => {
    if (appId) {
      fetchApp(appId);
    }
  }, [appId, fetchApp]);

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
    if (appId && activeTab === 'deployments') {
      deploymentsApi.list(appId).then(setDeployments).catch(() => {});
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
      addNotification({ type: 'success', title: 'App deleted' });
      navigate('/dashboard');
    } catch {
      addNotification({ type: 'error', title: 'Failed to delete app' });
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
      addNotification({ type: 'success', title: 'Environment variables saved' });
    } catch {
      addNotification({ type: 'error', title: 'Failed to save environment variables' });
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
      addNotification({ type: 'success', title: 'Settings saved' });
      fetchApp(appId);
    } catch {
      addNotification({ type: 'error', title: 'Failed to save settings' });
    }
  };

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

  if (!app) {
    return (
      <div className="flex items-center justify-center py-32">
        <div className="w-8 h-8 border-2 border-amber-400 border-t-transparent rounded-full animate-spin" />
      </div>
    );
  }

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-6">
        <div className="flex items-center gap-4">
          <PillButton
            variant="ghost"
            size="sm"
            onClick={() => navigate('/dashboard')}
            icon={<ArrowLeft size={16} />}
          >
            Back
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
              onClick={() => deployApp(appId!)}
              icon={<Play size={14} />}
            >
              Start
            </PillButton>
          )}
          {app.status === 'running' && (
            <>
              <PillButton
                variant="secondary"
                size="sm"
                onClick={() => restartApp(appId!)}
                icon={<RotateCcw size={14} />}
              >
                Restart
              </PillButton>
              <PillButton
                variant="ghost"
                size="sm"
                onClick={() => stopApp(appId!)}
                icon={<Square size={14} />}
              >
                Stop
              </PillButton>
            </>
          )}
          <PillButton
            variant="primary"
            size="sm"
            onClick={() => deployApp(appId!)}
            icon={<Rocket size={14} />}
          >
            Deploy
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
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-4">
            {/* Status Card */}
            <GlassCard className="lg:col-span-2">
              <h3 className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                App Info
              </h3>
              <div className="grid grid-cols-2 gap-4">
                {[
                  { label: 'Status', value: <AppStatusBadge status={app.status} size="md" /> },
                  { label: 'Stack', value: app.stack },
                  { label: 'Branch', value: app.repoBranch },
                  { label: 'Auto-deploy', value: app.autoDeploy ? 'Enabled' : 'Disabled' },
                  { label: 'Created', value: formatRelativeTime(app.createdAt) },
                  { label: 'Last Updated', value: formatRelativeTime(app.updatedAt) },
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
                Resources
              </h3>
              <div className="space-y-4">
                {/* CPU Gauge */}
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span className="text-zinc-500">CPU</span>
                    <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>
                      {formatPercent(app.cpuPercent ?? 0)}
                    </span>
                  </div>
                  <div className={`h-2 rounded-full ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                    <div
                      className="h-full rounded-full bg-amber-400 transition-all duration-500"
                      style={{ width: `${Math.min(app.cpuPercent ?? 0, 100)}%` }}
                    />
                  </div>
                </div>
                {/* RAM Gauge */}
                <div>
                  <div className="flex justify-between text-xs mb-2">
                    <span className="text-zinc-500">Memory</span>
                    <span className={isDark ? 'text-zinc-300' : 'text-zinc-700'}>
                      {formatBytes(app.memoryBytes ?? 0)}
                    </span>
                  </div>
                  <div className={`h-2 rounded-full ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                    <div
                      className="h-full rounded-full bg-blue-400 transition-all duration-500"
                      style={{
                        width: `${Math.min(
                          ((app.memoryBytes ?? 0) / (parseInt(app.resourceLimits?.memory || '512') * 1024 * 1024)) * 100,
                          100,
                        )}%`,
                      }}
                    />
                  </div>
                </div>
                {/* Limits */}
                <div className="pt-2 border-t border-zinc-800/50 space-y-1">
                  <div className="flex justify-between text-[11px]">
                    <span className="text-zinc-500">CPU Limit</span>
                    <span className="text-zinc-400">{app.resourceLimits?.cpu || '0.5'} cores</span>
                  </div>
                  <div className="flex justify-between text-[11px]">
                    <span className="text-zinc-500">Memory Limit</span>
                    <span className="text-zinc-400">{app.resourceLimits?.memory || '512'}MB</span>
                  </div>
                  <div className="flex justify-between text-[11px]">
                    <span className="text-zinc-500">Disk Limit</span>
                    <span className="text-zinc-400">{app.resourceLimits?.disk || '1'}GB</span>
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
          <DeployHistory
            deployments={deployments}
            onRollback={async (deployId) => {
              try {
                await deploymentsApi.rollback(appId!, deployId);
                addNotification({ type: 'success', title: 'Rollback initiated' });
                const updated = await deploymentsApi.list(appId!);
                setDeployments(updated);
              } catch {
                addNotification({ type: 'error', title: 'Rollback failed' });
              }
            }}
            onViewLog={(deployId) => {
              const deploy = deployments.find((d) => d.id === deployId);
              if (deploy?.buildLog) {
                // could open a modal, for now just switch to logs tab
                setActiveTab('logs');
              }
            }}
          />
        )}

        {/* ==================== LOGS ==================== */}
        {activeTab === 'logs' && (
          <LogViewer
            logs={logs.map((l) => ({ message: l.message, level: l.level, timestamp: l.timestamp }))}
            onClear={clearLogs}
          />
        )}

        {/* ==================== ENVIRONMENT ==================== */}
        {activeTab === 'env' && (
          <GlassCard>
            <div className="flex items-center justify-between mb-4">
              <h3
                className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                Environment Variables
              </h3>
              <div className="flex items-center gap-2">
                <PillButton
                  variant="ghost"
                  size="sm"
                  onClick={() => setShowValues(!showValues)}
                  icon={showValues ? <EyeOff size={14} /> : <Eye size={14} />}
                >
                  {showValues ? 'Hide' : 'Show'} Values
                </PillButton>
                <PillButton
                  variant="ghost"
                  size="sm"
                  onClick={() => setEnvVars([...envVars, { key: '', value: '' }])}
                  icon={<Plus size={14} />}
                >
                  Add
                </PillButton>
              </div>
            </div>

            {envVars.length === 0 ? (
              <p className="text-sm text-zinc-500 text-center py-8">
                No environment variables configured
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
                      placeholder="KEY"
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
                      placeholder="value"
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
                {savingEnv ? 'Saving...' : 'Save Changes'}
              </PillButton>
            </div>
          </GlassCard>
        )}

        {/* ==================== SERVICES ==================== */}
        {activeTab === 'services' && (
          <div>
            <div className="flex items-center justify-between mb-4">
              <h3
                className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                Managed Services
              </h3>
              <PillButton
                variant="ghost"
                size="sm"
                onClick={() => setShowServiceDialog(true)}
                icon={<Plus size={14} />}
              >
                Add Service
              </PillButton>
            </div>

            {services.length === 0 ? (
              <GlassCard>
                <p className="text-sm text-zinc-500 text-center py-8">
                  No services provisioned. Add PostgreSQL, Redis, MongoDB, or RabbitMQ.
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
                        await servicesApi.delete(appId!, serviceId);
                        setServices(services.filter((s) => s.id !== serviceId));
                        addNotification({ type: 'success', title: 'Service removed' });
                      } catch {
                        addNotification({ type: 'error', title: 'Failed to remove service' });
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
                  addNotification({ type: 'success', title: 'Service added' });
                } catch {
                  addNotification({ type: 'error', title: 'Failed to add service' });
                }
              }}
            />
          </div>
        )}

        {/* ==================== METRICS ==================== */}
        {activeTab === 'metrics' && (
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
            <MetricsChart
              data={metricsData}
              dataKey="cpu"
              title="CPU Usage"
              color="#fbbf24"
              unit="%"
            />
            <MetricsChart
              data={metricsData}
              dataKey="memory"
              title="Memory Usage"
              color="#60a5fa"
              formatter={(v) => `${v.toFixed(0)}MB`}
            />
            <MetricsChart
              data={metricsData}
              dataKey="networkRx"
              title="Network In (KB/s)"
              color="#34d399"
              unit=" KB/s"
            />
            <MetricsChart
              data={metricsData}
              dataKey="networkTx"
              title="Network Out (KB/s)"
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
                addNotification({ type: 'success', title: 'Alert created' });
              } catch {
                addNotification({ type: 'error', title: 'Failed to create alert' });
              }
            }}
            onDeleteAlert={async (alertId) => {
              try {
                await alertsApi.delete(appId!, alertId);
                setAlerts(alerts.filter((a) => a.id !== alertId));
                addNotification({ type: 'success', title: 'Alert deleted' });
              } catch {
                addNotification({ type: 'error', title: 'Failed to delete alert' });
              }
            }}
            onToggleAlert={async (alertId, enabled) => {
              try {
                await alertsApi.update(appId!, alertId, { enabled });
                setAlerts(alerts.map((a) => (a.id === alertId ? { ...a, enabled } : a)));
              } catch {
                addNotification({ type: 'error', title: 'Failed to update alert' });
              }
            }}
          />
        )}

        {/* ==================== SETTINGS ==================== */}
        {activeTab === 'settings' && (
          <div className="space-y-6 max-w-2xl">
            <GlassCard>
              <h3
                className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
              >
                General Settings
              </h3>
              <div className="space-y-4">
                <div>
                  <label className="block text-xs text-zinc-500 mb-1.5">App Name</label>
                  <input
                    type="text"
                    value={settingsName}
                    onChange={(e) => setSettingsName(e.target.value)}
                    className={inputClass}
                  />
                </div>
                <div>
                  <label className="block text-xs text-zinc-500 mb-1.5">Branch</label>
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
                      Auto-deploy
                    </p>
                    <p className="text-[11px] text-zinc-500">
                      Automatically deploy when you push to the configured branch
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
                  Save Settings
                </PillButton>
              </div>
            </GlassCard>

            {/* Danger Zone */}
            <GlassCard className="!border-red-500/20">
              <h3 className="text-sm font-semibold text-red-400 mb-4">Danger Zone</h3>
              <div className="flex items-center justify-between">
                <div>
                  <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                    Delete this app
                  </p>
                  <p className="text-[11px] text-zinc-500">
                    This will permanently destroy the container, data, and subdomain.
                  </p>
                </div>
                <PillButton
                  variant="danger"
                  size="sm"
                  onClick={() => setShowDeleteDialog(true)}
                  icon={<Trash2 size={14} />}
                >
                  Delete App
                </PillButton>
              </div>
            </GlassCard>
          </div>
        )}
      </div>

      {/* Delete confirmation */}
      <ConfirmDialog
        open={showDeleteDialog}
        title="Delete App"
        message={`Are you sure you want to delete "${app.name}"? This action cannot be undone.`}
        confirmLabel="Delete Forever"
        onConfirm={handleDelete}
        onCancel={() => setShowDeleteDialog(false)}
        loading={deleting}
      />
    </div>
  );
}

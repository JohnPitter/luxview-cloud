import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Shield,
  Users,
  Server,
  Activity,
  Rocket,
  RefreshCw,
  Search,
  Crown,
  UserCheck,
  Trash2,
  SlidersHorizontal,
  X,
  Check,
  Cpu,
  MemoryStick,
  HardDrive,
  Monitor,
  CreditCard,
  Plus,
  Star,
  Pencil,
  ToggleLeft,
  ToggleRight,
  Bot,
  Zap,
  CheckCircle2,
  XCircle,
  FileText,
  List,
  ChevronDown,
  ChevronUp,
  Loader2,
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { StatusDot } from '../components/common/StatusDot';
import { AppStatusBadge } from '../components/apps/AppStatusBadge';
import { ConfirmDialog } from '../components/common/ConfirmDialog';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { adminApi, cleanupApi, auditApi, timezoneApi, authSettingsApi, type AdminStats, type AdminUser, type AdminApp, type VPSInfo, type CleanupSettings, type CleanupResult, type DiskUsage, type AuditLog, type AuditStats, type AuditLogFilters } from '../api/admin';
import { plansApi, type Plan, type CreatePlanPayload } from '../api/plans';
import { aiSettingsApi, type AISettings, type AITestResult } from '../api/analyze';
import { formatRelativeTime } from '../lib/format';

type Tab = 'overview' | 'users' | 'apps' | 'plans' | 'ai' | 'cleanup' | 'audit';

function getDefaultPlanForm(): CreatePlanPayload {
  return {
    name: '',
    description: '',
    price: 0,
    currency: 'USD',
    billingCycle: 'monthly',
    maxApps: 3,
    maxCpuPerApp: 0.5,
    maxMemoryPerApp: '512m',
    maxDiskPerApp: '1g',
    maxServicesPerApp: 2,
    autoDeployEnabled: true,
    customDomainEnabled: false,
    priorityBuilds: false,
    highlighted: false,
    sortOrder: 0,
    features: [],
  };
}

function planToForm(plan: Plan): CreatePlanPayload {
  return {
    name: plan.name,
    description: plan.description,
    price: plan.price,
    currency: plan.currency,
    billingCycle: plan.billingCycle,
    maxApps: plan.maxApps,
    maxCpuPerApp: plan.maxCpuPerApp,
    maxMemoryPerApp: plan.maxMemoryPerApp,
    maxDiskPerApp: plan.maxDiskPerApp,
    maxServicesPerApp: plan.maxServicesPerApp,
    autoDeployEnabled: plan.autoDeployEnabled,
    customDomainEnabled: plan.customDomainEnabled,
    priorityBuilds: plan.priorityBuilds,
    highlighted: plan.highlighted,
    sortOrder: plan.sortOrder,
    features: [...plan.features],
  };
}

const CPU_OPTIONS = ['0.25', '0.5', '1.0', '2.0', '4.0'];
const MEMORY_OPTIONS = ['256m', '512m', '1g', '2g', '4g', '8g'];
const DISK_OPTIONS = ['1g', '5g', '10g', '20g', '50g'];

export function Admin() {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const tabs: Array<{ id: Tab; label: string; icon: React.ReactNode }> = [
    { id: 'overview', label: t('admin.tabs.overview'), icon: <Activity size={14} /> },
    { id: 'users', label: t('admin.tabs.users'), icon: <Users size={14} /> },
    { id: 'apps', label: t('admin.tabs.applications'), icon: <Server size={14} /> },
    { id: 'plans', label: t('admin.tabs.plans'), icon: <CreditCard size={14} /> },
    { id: 'ai', label: t('admin.tabs.ai'), icon: <Bot size={14} /> },
    { id: 'cleanup', label: t('admin.tabs.cleanup'), icon: <HardDrive size={14} /> },
    { id: 'audit', label: t('admin.tabs.audit'), icon: <FileText size={14} /> },
  ];

  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [vpsInfo, setVpsInfo] = useState<VPSInfo | null>(null);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [apps, setApps] = useState<AdminApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [userSearch, setUserSearch] = useState('');
  const [appSearch, setAppSearch] = useState('');
  const [userPage, setUserPage] = useState(0);
  const [appPage, setAppPage] = useState(0);
  const PAGE_SIZE = 20;

  // Auth settings
  const [requireAuth, setRequireAuth] = useState(true);

  // Timezone
  const [platformTimezone, setPlatformTimezone] = useState('UTC');

  // Modals
  const [roleChangeUser, setRoleChangeUser] = useState<AdminUser | null>(null);
  const [limitsApp, setLimitsApp] = useState<AdminApp | null>(null);
  const [deleteApp, setDeleteApp] = useState<AdminApp | null>(null);
  const [editLimits, setEditLimits] = useState({ cpu: '1.0', memory: '512m', disk: '5g' });

  // Plans state
  const [plans, setPlans] = useState<Plan[]>([]);
  const [editPlan, setEditPlan] = useState<Plan | null>(null);
  const [showCreatePlan, setShowCreatePlan] = useState(false);
  const [deletePlanTarget, setDeletePlanTarget] = useState<Plan | null>(null);
  const [planFormData, setPlanFormData] = useState<CreatePlanPayload>(getDefaultPlanForm());
  const [planUserTarget, setPlanUserTarget] = useState<AdminUser | null>(null);

  // AI Settings state
  const [aiSettings, setAiSettings] = useState<AISettings | null>(null);
  const [aiLoading, setAiLoading] = useState(false);
  const [aiSaving, setAiSaving] = useState(false);
  const [aiTesting, setAiTesting] = useState(false);
  const [aiTestResult, setAiTestResult] = useState<AITestResult | null>(null);
  const [aiForm, setAiForm] = useState({
    apiKey: '',
    aiEnabled: false,
    aiModel: 'anthropic/claude-sonnet-4',
  });

  // Cleanup state
  const [cleanupSettings, setCleanupSettings] = useState<CleanupSettings | null>(null);
  const [cleanupLoading, setCleanupLoading] = useState(false);
  const [cleanupSaving, setCleanupSaving] = useState(false);
  const [cleanupRunning, setCleanupRunning] = useState(false);
  const [cleanupResult, setCleanupResult] = useState<CleanupResult | null>(null);
  const [diskUsage, setDiskUsage] = useState<DiskUsage | null>(null);
  const [cleanupForm, setCleanupForm] = useState({
    enabled: false,
    intervalHours: 24,
    thresholdPercent: 80,
  });

  // Audit state
  const [auditLogs, setAuditLogs] = useState<AuditLog[]>([]);
  const [auditTotal, setAuditTotal] = useState(0);
  const [auditStats, setAuditStats] = useState<AuditStats | null>(null);
  const [auditLoading, setAuditLoading] = useState(false);
  const [auditPage, setAuditPage] = useState(0);
  const [auditView, setAuditView] = useState<'table' | 'timeline'>('table');
  const [auditFilters, setAuditFilters] = useState<AuditLogFilters>({});
  const [auditExpanded, setAuditExpanded] = useState<Set<number>>(new Set());

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [statsData, vpsData, usersData, appsData, plansData, tzData, authData] = await Promise.all([
        adminApi.stats(),
        adminApi.vpsInfo(),
        adminApi.listUsers(100, 0),
        adminApi.listApps(100, 0),
        plansApi.listAll(),
        timezoneApi.get().catch(() => ({ timezone: 'UTC' })),
        authSettingsApi.get().catch(() => ({ requireAuth: true })),
      ]);
      setStats(statsData);
      setVpsInfo(vpsData);
      setUsers(usersData.users ?? []);
      setApps(appsData.apps ?? []);
      setPlans(plansData ?? []);
      setPlatformTimezone(tzData.timezone);
      setRequireAuth(authData.requireAuth);
    } catch {
      addNotification({ type: 'error', title: t('admin.failedToLoad') });
    } finally {
      setLoading(false);
    }
  }, [addNotification, t]);

  useEffect(() => {
    fetchData();
    // Auto-refresh admin data every 30s
    const interval = setInterval(async () => {
      try {
        const [statsData, vpsData, usersData, appsData, plansData] = await Promise.all([
          adminApi.stats(),
          adminApi.vpsInfo(),
          adminApi.listUsers(100, 0),
          adminApi.listApps(100, 0),
          plansApi.listAll(),
        ]);
        setStats(statsData);
        setVpsInfo(vpsData);
        setUsers(usersData.users ?? []);
        setApps(appsData.apps ?? []);
        setPlans(plansData ?? []);
      } catch { /* silent refresh */ }
    }, 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const fetchAISettings = useCallback(async () => {
    setAiLoading(true);
    try {
      const data = await aiSettingsApi.get();
      setAiSettings(data);
      setAiForm({
        apiKey: '',
        aiEnabled: data.aiEnabled,
        aiModel: data.aiModel || 'anthropic/claude-sonnet-4',
      });
    } catch {
      addNotification({ type: 'error', title: t('admin.failedToLoad') });
    } finally {
      setAiLoading(false);
    }
  }, [addNotification, t]);

  useEffect(() => {
    if (activeTab === 'ai') fetchAISettings();
  }, [activeTab, fetchAISettings]);

  const handleSaveAISettings = async () => {
    setAiSaving(true);
    setAiTestResult(null);
    try {
      const payload: Partial<AISettings> = {
        aiEnabled: aiForm.aiEnabled,
        aiModel: aiForm.aiModel,
      };
      if (aiForm.apiKey) payload.apiKey = aiForm.apiKey;
      await aiSettingsApi.update(payload);
      addNotification({ type: 'success', title: t('admin.ai.saved') });

      // Auto-test connection after save if credentials were provided
      if (aiForm.apiKey) {
        setAiTesting(true);
        try {
          const result = await aiSettingsApi.testConnection({
            model: aiForm.aiModel || undefined,
          });
          setAiTestResult(result);
        } catch {
          setAiTestResult({ success: false, error: t('admin.ai.testError') });
        } finally {
          setAiTesting(false);
        }
      }

      fetchAISettings();
    } catch {
      addNotification({ type: 'error', title: t('admin.failedToLoad') });
    } finally {
      setAiSaving(false);
    }
  };

  const handleTestAIConnection = async () => {
    setAiTesting(true);
    setAiTestResult(null);
    try {
      const result = await aiSettingsApi.testConnection({
        apiKey: aiForm.apiKey || undefined,
        model: aiForm.aiModel || undefined,
      });
      setAiTestResult(result);
    } catch {
      setAiTestResult({ success: false, error: t('admin.ai.testError') });
    } finally {
      setAiTesting(false);
    }
  };

  // Cleanup fetch and handlers
  const fetchCleanupSettings = useCallback(async () => {
    setCleanupLoading(true);
    try {
      const [settings, disk] = await Promise.all([cleanupApi.getSettings(), cleanupApi.diskUsage()]);
      setCleanupSettings(settings);
      setCleanupForm({
        enabled: settings.enabled,
        intervalHours: settings.intervalHours,
        thresholdPercent: settings.thresholdPercent,
      });
      setDiskUsage(disk);
    } catch {
      addNotification({ type: 'error', title: t('admin.failedToLoad') });
    } finally {
      setCleanupLoading(false);
    }
  }, [addNotification, t]);

  useEffect(() => {
    if (activeTab === 'cleanup') fetchCleanupSettings();
  }, [activeTab, fetchCleanupSettings]);

  const handleSaveCleanupSettings = async () => {
    setCleanupSaving(true);
    try {
      await cleanupApi.updateSettings(cleanupForm);
      addNotification({ type: 'success', title: t('admin.cleanup.saved') });
      fetchCleanupSettings();
    } catch {
      addNotification({ type: 'error', title: t('admin.cleanup.failedToSave') });
    } finally {
      setCleanupSaving(false);
    }
  };

  const handleTriggerCleanup = async () => {
    setCleanupRunning(true);
    setCleanupResult(null);
    try {
      const result = await cleanupApi.trigger();
      setCleanupResult(result);
      addNotification({ type: 'success', title: t('admin.cleanup.completed') });
      // Refresh disk usage after cleanup
      const disk = await cleanupApi.diskUsage();
      setDiskUsage(disk);
    } catch {
      addNotification({ type: 'error', title: t('admin.cleanup.failed') });
    } finally {
      setCleanupRunning(false);
    }
  };

  const fetchAuditLogs = useCallback(async () => {
    setAuditLoading(true);
    try {
      const [logsData, statsData] = await Promise.all([
        auditApi.list(auditFilters, 50, auditPage * 50),
        auditApi.stats(),
      ]);
      setAuditLogs(logsData.logs ?? []);
      setAuditTotal(logsData.total);
      setAuditStats(statsData);
    } catch {
      addNotification({ type: 'error', title: t('admin.failedToLoad') });
    } finally {
      setAuditLoading(false);
    }
  }, [auditFilters, auditPage, addNotification, t]);

  useEffect(() => {
    if (activeTab === 'audit') fetchAuditLogs();
  }, [activeTab, fetchAuditLogs]);

  const handleRoleChange = async (user: AdminUser, newRole: 'user' | 'admin') => {
    try {
      await adminApi.updateUserRole(user.id, newRole);
      setUsers((prev) => prev.map((u) => (u.id === user.id ? { ...u, role: newRole } : u)));
      addNotification({ type: 'success', title: t('admin.roleModal.roleChanged', { username: user.username, role: newRole }) });
    } catch {
      addNotification({ type: 'error', title: t('admin.roleModal.failedToUpdate') });
    }
    setRoleChangeUser(null);
  };

  const handleUpdateLimits = async () => {
    if (!limitsApp) return;
    try {
      await adminApi.updateAppLimits(limitsApp.id, editLimits);
      setApps((prev) =>
        prev.map((a) => (a.id === limitsApp.id ? { ...a, resourceLimits: editLimits } : a)),
      );
      addNotification({ type: 'success', title: t('admin.limitsModal.limitsUpdated', { name: limitsApp.name }) });
    } catch {
      addNotification({ type: 'error', title: t('admin.limitsModal.failedToUpdate') });
    }
    setLimitsApp(null);
  };

  const handleForceDelete = async () => {
    if (!deleteApp) return;
    try {
      await adminApi.forceDeleteApp(deleteApp.id);
      setApps((prev) => prev.filter((a) => a.id !== deleteApp.id));
      addNotification({ type: 'success', title: t('admin.deleteDialog.deleted', { name: deleteApp.name }) });
    } catch {
      addNotification({ type: 'error', title: t('admin.deleteDialog.failedToDelete') });
    }
    setDeleteApp(null);
  };

  const handleCreatePlan = async () => {
    try {
      const created = await plansApi.create(planFormData);
      setPlans((prev) => [...prev, created]);
      addNotification({ type: 'success', title: t('admin.plans.planCreated', { name: created.name }) });
      setShowCreatePlan(false);
    } catch {
      addNotification({ type: 'error', title: t('admin.plans.failedToCreate') });
    }
  };

  const handleUpdatePlan = async () => {
    if (!editPlan) return;
    try {
      const updated = await plansApi.update(editPlan.id, planFormData);
      setPlans((prev) => prev.map((p) => (p.id === editPlan.id ? updated : p)));
      addNotification({ type: 'success', title: t('admin.plans.planUpdated', { name: updated.name }) });
      setEditPlan(null);
    } catch {
      addNotification({ type: 'error', title: t('admin.plans.failedToUpdate') });
    }
  };

  const handleDeletePlan = async () => {
    if (!deletePlanTarget) return;
    try {
      await plansApi.delete(deletePlanTarget.id);
      setPlans((prev) => prev.filter((p) => p.id !== deletePlanTarget.id));
      addNotification({ type: 'success', title: t('admin.plans.planDeleted', { name: deletePlanTarget.name }) });
    } catch {
      addNotification({ type: 'error', title: t('admin.plans.failedToDelete') });
    }
    setDeletePlanTarget(null);
  };

  const handleSetDefault = async (planId: string) => {
    try {
      await plansApi.setDefault(planId);
      setPlans((prev) => prev.map((p) => ({ ...p, isDefault: p.id === planId })));
      addNotification({ type: 'success', title: t('admin.plans.defaultSet') });
    } catch {
      addNotification({ type: 'error', title: t('admin.plans.failedToUpdate') });
    }
  };

  const handleAssignPlan = async (userId: string, planId: string) => {
    const user = users.find((u) => u.id === userId);
    try {
      await plansApi.assignUserPlan(userId, planId);
      addNotification({ type: 'success', title: t('admin.plans.planAssigned', { username: user?.username ?? '' }) });
    } catch {
      addNotification({ type: 'error', title: t('admin.plans.failedToAssign') });
    }
    setPlanUserTarget(null);
  };

  const filteredUsers = users.filter(
    (u) =>
      !userSearch ||
      u.username.toLowerCase().includes(userSearch.toLowerCase()) ||
      u.email.toLowerCase().includes(userSearch.toLowerCase()),
  );
  const userTotalPages = Math.max(1, Math.ceil(filteredUsers.length / PAGE_SIZE));
  const paginatedUsers = filteredUsers.slice(userPage * PAGE_SIZE, (userPage + 1) * PAGE_SIZE);

  const filteredApps = apps.filter(
    (a) =>
      !appSearch ||
      a.name.toLowerCase().includes(appSearch.toLowerCase()) ||
      a.subdomain.toLowerCase().includes(appSearch.toLowerCase()),
  );
  const appTotalPages = Math.max(1, Math.ceil(filteredApps.length / PAGE_SIZE));
  const paginatedApps = filteredApps.slice(appPage * PAGE_SIZE, (appPage + 1) * PAGE_SIZE);

  const ownerMap = new Map(users.map((u) => [u.id, u.username]));

  const toggleAuditExpand = (id: number) => {
    setAuditExpanded((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  };

  const actionColor = (action: string) => {
    const colors: Record<string, string> = {
      create: isDark ? 'bg-emerald-900/30 text-emerald-400 border-emerald-800/50' : 'bg-emerald-50 text-emerald-700 border-emerald-200',
      update: isDark ? 'bg-blue-900/30 text-blue-400 border-blue-800/50' : 'bg-blue-50 text-blue-700 border-blue-200',
      delete: isDark ? 'bg-red-900/30 text-red-400 border-red-800/50' : 'bg-red-50 text-red-700 border-red-200',
      deploy: isDark ? 'bg-purple-900/30 text-purple-400 border-purple-800/50' : 'bg-purple-50 text-purple-700 border-purple-200',
      restart: isDark ? 'bg-purple-900/30 text-purple-400 border-purple-800/50' : 'bg-purple-50 text-purple-700 border-purple-200',
      stop: isDark ? 'bg-purple-900/30 text-purple-400 border-purple-800/50' : 'bg-purple-50 text-purple-700 border-purple-200',
      login: isDark ? 'bg-zinc-800/50 text-zinc-400 border-zinc-700/50' : 'bg-zinc-100 text-zinc-600 border-zinc-200',
    };
    return colors[action] || colors.update;
  };

  const resourceIcon = (type: string) => {
    const icons: Record<string, React.ReactNode> = {
      app: <Server size={14} />,
      user: <Users size={14} />,
      service: <HardDrive size={14} />,
      plan: <CreditCard size={14} />,
      setting: <SlidersHorizontal size={14} />,
      deployment: <Rocket size={14} />,
      alert: <Activity size={14} />,
      cleanup: <Trash2 size={14} />,
    };
    return icons[type] || <Activity size={14} />;
  };

  const groupLogsByDay = (logs: AuditLog[]) => {
    const groups: { label: string; logs: AuditLog[] }[] = [];
    const today = new Date().toDateString();
    const yesterday = new Date(Date.now() - 86400000).toDateString();

    for (const log of logs) {
      const day = new Date(log.createdAt).toDateString();
      const label = day === today ? t('admin.audit.today') : day === yesterday ? t('admin.audit.yesterday') : new Date(log.createdAt).toLocaleDateString();
      const existing = groups.find((g) => g.label === label);
      if (existing) existing.logs.push(log);
      else groups.push({ label, logs: [log] });
    }
    return groups;
  };

  return (
    <div className="animate-fade-in">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div className="flex items-center gap-3">
          <Shield size={24} className="text-amber-400" />
          <div>
            <h1
              className={`text-2xl font-bold tracking-tight ${
                isDark ? 'text-zinc-100' : 'text-zinc-900'
              }`}
            >
              {t('admin.title')}
            </h1>
            <p className="text-sm text-zinc-500">{t('admin.subtitle')}</p>
          </div>
        </div>
        <PillButton
          variant="secondary"
          size="sm"
          onClick={fetchData}
          icon={<RefreshCw size={14} />}
        >
          {t('common.refresh')}
        </PillButton>
      </div>

      {/* Tabs */}
      <div className="flex items-center gap-1 mb-6">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={`flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-xl transition-all ${
              activeTab === tab.id
                ? 'bg-amber-500/20 text-amber-400 border border-amber-500/30'
                : isDark
                  ? 'text-zinc-400 hover:text-zinc-200 border border-transparent hover:border-zinc-800'
                  : 'text-zinc-600 hover:text-zinc-900 border border-transparent hover:border-zinc-200'
            }`}
          >
            {tab.icon}
            {tab.label}
          </button>
        ))}
      </div>

      {loading ? (
        <div className="text-center py-16 text-sm text-zinc-500">{t('admin.loadingData')}</div>
      ) : (
        <>
          {/* ==================== OVERVIEW ==================== */}
          {activeTab === 'overview' && stats && (
            <div className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
                <StatCard
                  icon={<Users size={20} className="text-blue-400" />}
                  label={t('admin.stats.totalUsers')}
                  value={stats.totalUsers}
                  color="bg-blue-500/10"
                />
                <StatCard
                  icon={<Server size={20} className="text-emerald-400" />}
                  label={t('admin.stats.totalApps')}
                  value={stats.totalApps}
                  sub={t('admin.stats.runningApps', { count: stats.runningApps })}
                  color="bg-emerald-500/10"
                />
                <StatCard
                  icon={<Rocket size={20} className="text-amber-400" />}
                  label={t('admin.stats.deployments')}
                  value={stats.totalDeployments}
                  sub={t('admin.stats.allTime')}
                  color="bg-amber-500/10"
                />
                <StatCard
                  icon={<Activity size={20} className="text-violet-400" />}
                  label={t('admin.stats.running')}
                  value={stats.runningApps}
                  sub={t('admin.stats.ofApps', { total: stats.totalApps })}
                  color="bg-violet-500/10"
                />
              </div>

              {/* VPS Resources */}
              {vpsInfo && (
                <GlassCard>
                  <div className="flex items-center gap-2 mb-5">
                    <Monitor size={18} className="text-amber-400" />
                    <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {t('admin.vps.title')} — {vpsInfo.hostname}
                    </h3>
                    <span className="text-[10px] text-zinc-500 font-mono ml-auto">
                      {vpsInfo.os}/{vpsInfo.arch} • {vpsInfo.goVersion}
                    </span>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
                    {/* CPU */}
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <Cpu size={14} className="text-blue-400" />
                        <span className="text-xs text-zinc-400 uppercase tracking-wider">{t('admin.vps.cpu')}</span>
                      </div>
                      <div className="flex items-baseline gap-2">
                        <span className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                          {vpsInfo.cpuCores}
                        </span>
                        <span className="text-xs text-zinc-500">{t('common.units.coresTotal')}</span>
                      </div>
                      <ResourceBar
                        used={parseFloat(vpsInfo.allocatedCpu || '0') + vpsInfo.platformReservedCpu}
                        total={vpsInfo.cpuCores}
                        label={`${t('admin.vps.platformLabel')}: ${vpsInfo.platformReservedCpu} · ${t('admin.vps.appsLabel')}: ${vpsInfo.allocatedCpu || '0'} · ${t('admin.vps.freeLabel')}: ${vpsInfo.freeCpu || '0'} ${t('common.units.cores')}`}
                        color="blue"
                        isDark={isDark}
                      />
                    </div>

                    {/* Memory */}
                    <div className="space-y-2">
                      <div className="flex items-center gap-2">
                        <MemoryStick size={14} className="text-emerald-400" />
                        <span className="text-xs text-zinc-400 uppercase tracking-wider">{t('admin.vps.memory')}</span>
                      </div>
                      <div className="flex items-baseline gap-2">
                        <span className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                          {formatBytes(vpsInfo.totalMemory)}
                        </span>
                        <span className="text-xs text-zinc-500">{t('common.units.total')}</span>
                      </div>
                      <ResourceBar
                        used={vpsInfo.allocatedMemory + vpsInfo.platformReservedMem}
                        total={vpsInfo.totalMemory}
                        label={`${t('admin.vps.platformLabel')}: ${formatBytes(vpsInfo.platformReservedMem)} · ${t('admin.vps.appsLabel')}: ${formatBytes(vpsInfo.allocatedMemory)} · ${t('admin.vps.freeLabel')}: ${formatBytes(Math.max(0, vpsInfo.freeMemory))}`}
                        color="emerald"
                        isDark={isDark}
                      />
                    </div>

                    {/* Disk */}
                    {vpsInfo.disk && (
                      <div className="space-y-2">
                        <div className="flex items-center gap-2">
                          <HardDrive size={14} className="text-amber-400" />
                          <span className="text-xs text-zinc-400 uppercase tracking-wider">{t('admin.vps.disk')}</span>
                        </div>
                        <div className="flex items-baseline gap-2">
                          <span className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                            {formatBytes(vpsInfo.disk.total)}
                          </span>
                          <span className="text-xs text-zinc-500">{t('common.units.total')}</span>
                        </div>
                        <ResourceBar
                          used={vpsInfo.disk.used}
                          total={vpsInfo.disk.total}
                          label={`${formatBytes(vpsInfo.disk.used)} used · ${formatBytes(vpsInfo.disk.available)} free (${vpsInfo.disk.percent})`}
                          color="amber"
                          isDark={isDark}
                        />
                      </div>
                    )}
                  </div>

                  <div className={`mt-4 flex items-center gap-4 text-[11px] ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
                    <span>{t('admin.vps.appsDeployed', { count: vpsInfo.totalAppsCounted })}</span>
                    <span className="text-zinc-700">·</span>
                    <span>{t('admin.vps.platformReserved', { cpu: vpsInfo.platformReservedCpu, ram: formatBytes(vpsInfo.platformReservedMem) })}</span>
                    <span className="text-zinc-700">·</span>
                    <span>{t('admin.vps.defaultPerApp')}</span>
                  </div>
                </GlassCard>
              )}

              {/* Quick lists */}
              <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
                {/* Recent Users */}
                <GlassCard padding="none">
                  <div className="px-6 py-4 border-b border-zinc-800/50">
                    <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {t('admin.recentUsers')}
                    </h3>
                  </div>
                  <div className="divide-y divide-zinc-800/30">
                    {users.slice(0, 5).map((u) => (
                      <div key={u.id} className="flex items-center gap-3 px-6 py-3">
                        <img src={u.avatarUrl} alt="" className="w-8 h-8 rounded-full" />
                        <div className="flex-1 min-w-0">
                          <p className={`text-sm font-medium truncate ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                            {u.username}
                          </p>
                          <p className="text-[11px] text-zinc-500 truncate">{u.email}</p>
                        </div>
                        <span
                          className={`text-[10px] font-mono px-2 py-0.5 rounded-full ${
                            u.role === 'admin'
                              ? 'bg-amber-400/10 text-amber-400'
                              : 'bg-zinc-800/50 text-zinc-400'
                          }`}
                        >
                          {u.role}
                        </span>
                      </div>
                    ))}
                  </div>
                </GlassCard>

                {/* Error Apps */}
                <GlassCard padding="none">
                  <div className="px-6 py-4 border-b border-zinc-800/50">
                    <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {t('admin.appsWithIssues')}
                    </h3>
                  </div>
                  <div className="divide-y divide-zinc-800/30">
                    {apps.filter((a) => a.status === 'error' || a.status === 'stopped').length === 0 ? (
                      <div className="px-6 py-8 text-center text-sm text-zinc-500">
                        {t('admin.allAppsHealthy')}
                      </div>
                    ) : (
                      apps
                        .filter((a) => a.status === 'error' || a.status === 'stopped')
                        .slice(0, 5)
                        .map((a) => (
                          <div key={a.id} className="flex items-center gap-3 px-6 py-3">
                            <StatusDot status={a.status} size="sm" />
                            <div className="flex-1 min-w-0">
                              <p className={`text-sm font-medium truncate ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                                {a.name}
                              </p>
                              <p className="text-[11px] text-zinc-500">{a.subdomain}.luxview.cloud</p>
                            </div>
                            <AppStatusBadge status={a.status} />
                          </div>
                        ))
                    )}
                  </div>
                </GlassCard>
              </div>
            </div>
          )}

          {/* ==================== AUTH SETTINGS (in overview) ==================== */}
          {activeTab === 'overview' && (
            <GlassCard className="mt-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                    {t('admin.auth.title')}
                  </h3>
                  <p className="text-[11px] text-zinc-500 mt-0.5">
                    {t('admin.auth.description')}
                  </p>
                </div>
                <button
                  onClick={async () => {
                    const newVal = !requireAuth;
                    try {
                      await authSettingsApi.update(newVal);
                      setRequireAuth(newVal);
                      addNotification({ type: 'success', title: t('admin.auth.updated') });
                    } catch {
                      addNotification({ type: 'error', title: t('admin.auth.updateFailed') });
                    }
                  }}
                  className={`
                    w-11 h-6 rounded-full transition-all duration-200 relative
                    ${requireAuth ? 'bg-amber-400' : isDark ? 'bg-zinc-700' : 'bg-zinc-300'}
                  `}
                >
                  <span
                    className={`
                      absolute top-0.5 w-5 h-5 rounded-full bg-white shadow-sm transition-all duration-200
                      ${requireAuth ? 'left-[22px]' : 'left-0.5'}
                    `}
                  />
                </button>
              </div>
            </GlassCard>
          )}

          {/* ==================== TIMEZONE (in overview) ==================== */}
          {activeTab === 'overview' && (
            <GlassCard className="mt-4">
              <div className="flex items-center justify-between">
                <div>
                  <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                    {t('admin.timezone.title')}
                  </h3>
                  <p className="text-[11px] text-zinc-500 mt-0.5">
                    {t('admin.timezone.description')}
                  </p>
                </div>
                <div className="flex items-center gap-2">
                  <select
                    value={platformTimezone}
                    onChange={async (e) => {
                      const tz = e.target.value;
                      try {
                        await timezoneApi.update(tz);
                        setPlatformTimezone(tz);
                        addNotification({ type: 'success', title: t('admin.timezone.updated') });
                      } catch {
                        addNotification({ type: 'error', title: t('admin.timezone.updateFailed') });
                      }
                    }}
                    className={`text-xs rounded-lg border px-3 py-2 focus:outline-none focus:ring-2 focus:ring-primary/50 ${
                      isDark
                        ? 'bg-zinc-900 border-zinc-700 text-zinc-300'
                        : 'bg-white border-zinc-200 text-zinc-800'
                    }`}
                  >
                    <option value="UTC">UTC</option>
                    <option value="America/Sao_Paulo">America/Sao_Paulo (BRT)</option>
                    <option value="America/New_York">America/New_York (EST)</option>
                    <option value="America/Chicago">America/Chicago (CST)</option>
                    <option value="America/Denver">America/Denver (MST)</option>
                    <option value="America/Los_Angeles">America/Los_Angeles (PST)</option>
                    <option value="America/Argentina/Buenos_Aires">America/Buenos_Aires (ART)</option>
                    <option value="America/Bogota">America/Bogota (COT)</option>
                    <option value="America/Santiago">America/Santiago (CLT)</option>
                    <option value="America/Mexico_City">America/Mexico_City (CST)</option>
                    <option value="Europe/London">Europe/London (GMT)</option>
                    <option value="Europe/Paris">Europe/Paris (CET)</option>
                    <option value="Europe/Berlin">Europe/Berlin (CET)</option>
                    <option value="Europe/Lisbon">Europe/Lisbon (WET)</option>
                    <option value="Europe/Madrid">Europe/Madrid (CET)</option>
                    <option value="Asia/Tokyo">Asia/Tokyo (JST)</option>
                    <option value="Asia/Shanghai">Asia/Shanghai (CST)</option>
                    <option value="Asia/Kolkata">Asia/Kolkata (IST)</option>
                    <option value="Asia/Dubai">Asia/Dubai (GST)</option>
                    <option value="Australia/Sydney">Australia/Sydney (AEST)</option>
                  </select>
                </div>
              </div>
            </GlassCard>
          )}

          {/* ==================== USERS ==================== */}
          {activeTab === 'users' && (
            <div className="space-y-4">
              <div
                className={`flex items-center gap-2 px-4 py-2.5 rounded-xl ${
                  isDark ? 'bg-zinc-900/50 border border-zinc-800/50' : 'bg-white border border-zinc-200'
                }`}
              >
                <Search size={16} className="text-zinc-500" />
                <input
                  type="text"
                  placeholder={t('admin.users.searchPlaceholder')}
                  value={userSearch}
                  onChange={(e) => { setUserSearch(e.target.value); setUserPage(0); }}
                  className={`flex-1 bg-transparent text-sm outline-none ${
                    isDark ? 'text-zinc-200 placeholder:text-zinc-600' : 'text-zinc-800 placeholder:text-zinc-400'
                  }`}
                />
              </div>

              <GlassCard padding="none">
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className={isDark ? 'text-zinc-500' : 'text-zinc-400'}>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.user')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.email')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.role')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.plan')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.joined')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.actions')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {paginatedUsers.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="text-center py-12 text-sm text-zinc-500">
                            {t('admin.users.noUsersFound')}
                          </td>
                        </tr>
                      ) : (
                        paginatedUsers.map((u) => (
                          <tr
                            key={u.id}
                            className={`border-t transition-colors ${
                              isDark ? 'border-zinc-800/50 hover:bg-zinc-800/20' : 'border-zinc-100 hover:bg-zinc-50'
                            }`}
                          >
                            <td className="px-6 py-3">
                              <div className="flex items-center gap-3">
                                <img src={u.avatarUrl} alt="" className="w-8 h-8 rounded-full" />
                                <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                                  {u.username}
                                </span>
                              </div>
                            </td>
                            <td className="px-6 py-3 text-xs text-zinc-400">{u.email}</td>
                            <td className="px-6 py-3">
                              <span
                                className={`inline-flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-full ${
                                  u.role === 'admin'
                                    ? 'bg-amber-400/10 text-amber-400'
                                    : 'bg-zinc-800/50 text-zinc-400'
                                }`}
                              >
                                {u.role === 'admin' ? <Crown size={10} /> : <UserCheck size={10} />}
                                {u.role}
                              </span>
                            </td>
                            <td className="px-6 py-3">
                              <span className="text-[11px] font-mono px-2 py-0.5 rounded-full bg-zinc-800/50 text-zinc-400">
                                {plans.find((p) => p.id === (u as AdminUser & { planId?: string }).planId)?.name ?? t('admin.plans.noPlan')}
                              </span>
                            </td>
                            <td className="px-6 py-3 text-xs text-zinc-500">
                              {formatRelativeTime(u.createdAt)}
                            </td>
                            <td className="px-6 py-3">
                              <div className="flex items-center gap-1">
                                <PillButton
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => setRoleChangeUser(u)}
                                >
                                  {t('admin.users.changeRole')}
                                </PillButton>
                                <PillButton
                                  variant="ghost"
                                  size="sm"
                                  onClick={() => setPlanUserTarget(u)}
                                >
                                  {t('admin.users.changePlan')}
                                </PillButton>
                              </div>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </GlassCard>
              {userTotalPages > 1 && (
                <div className="flex items-center justify-center gap-3 mt-4">
                  <button
                    disabled={userPage <= 0}
                    onClick={() => setUserPage(userPage - 1)}
                    className={`px-3 py-1.5 text-xs rounded-lg border transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${isDark ? 'border-zinc-700 hover:bg-zinc-800' : 'border-zinc-200 hover:bg-zinc-100'}`}
                  >
                    {t('common.previous')}
                  </button>
                  <span className="text-xs text-zinc-500">{userPage + 1} / {userTotalPages}</span>
                  <button
                    disabled={userPage >= userTotalPages - 1}
                    onClick={() => setUserPage(userPage + 1)}
                    className={`px-3 py-1.5 text-xs rounded-lg border transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${isDark ? 'border-zinc-700 hover:bg-zinc-800' : 'border-zinc-200 hover:bg-zinc-100'}`}
                  >
                    {t('common.next')}
                  </button>
                </div>
              )}
            </div>
          )}

          {/* ==================== APPS ==================== */}
          {activeTab === 'apps' && (
            <div className="space-y-4">
              <div
                className={`flex items-center gap-2 px-4 py-2.5 rounded-xl ${
                  isDark ? 'bg-zinc-900/50 border border-zinc-800/50' : 'bg-white border border-zinc-200'
                }`}
              >
                <Search size={16} className="text-zinc-500" />
                <input
                  type="text"
                  placeholder={t('admin.apps.searchPlaceholder')}
                  value={appSearch}
                  onChange={(e) => { setAppSearch(e.target.value); setAppPage(0); }}
                  className={`flex-1 bg-transparent text-sm outline-none ${
                    isDark ? 'text-zinc-200 placeholder:text-zinc-600' : 'text-zinc-800 placeholder:text-zinc-400'
                  }`}
                />
              </div>

              <GlassCard padding="none">
                <div className="overflow-x-auto">
                  <table className="w-full">
                    <thead>
                      <tr className={isDark ? 'text-zinc-500' : 'text-zinc-400'}>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.app')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.owner')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.status')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.stack')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.limits')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.created')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.apps.tableHeaders.actions')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {paginatedApps.length === 0 ? (
                        <tr>
                          <td colSpan={7} className="text-center py-12 text-sm text-zinc-500">
                            {t('admin.apps.noAppsFound')}
                          </td>
                        </tr>
                      ) : (
                        paginatedApps.map((app) => (
                          <tr
                            key={app.id}
                            className={`border-t transition-colors ${
                              isDark ? 'border-zinc-800/50 hover:bg-zinc-800/20' : 'border-zinc-100 hover:bg-zinc-50'
                            }`}
                          >
                            <td className="px-6 py-3">
                              <div>
                                <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                                  {app.name}
                                </p>
                                <p className="text-[11px] text-zinc-500">{app.subdomain}.luxview.cloud</p>
                              </div>
                            </td>
                            <td className="px-6 py-3 text-xs text-zinc-400">
                              {ownerMap.get(app.userId) || t('admin.apps.unknown')}
                            </td>
                            <td className="px-6 py-3">
                              <AppStatusBadge status={app.status} />
                            </td>
                            <td className="px-6 py-3 text-xs text-zinc-400 uppercase">{app.stack || '\u2014'}</td>
                            <td className="px-6 py-3">
                              <div className="text-[11px] text-zinc-400 space-y-0.5">
                                <div>CPU: {app.resourceLimits?.cpu || '\u2014'}</div>
                                <div>RAM: {app.resourceLimits?.memory || '\u2014'}</div>
                              </div>
                            </td>
                            <td className="px-6 py-3 text-xs text-zinc-500">
                              {formatRelativeTime(app.createdAt)}
                            </td>
                            <td className="px-6 py-3">
                              <div className="flex items-center gap-1">
                                <button
                                  onClick={() => {
                                    setLimitsApp(app);
                                    setEditLimits(app.resourceLimits || { cpu: '1.0', memory: '512m', disk: '5g' });
                                  }}
                                  className="p-1.5 text-zinc-500 hover:text-amber-400 transition-colors rounded-lg hover:bg-amber-400/10"
                                  title={t('admin.apps.editLimits')}
                                >
                                  <SlidersHorizontal size={14} />
                                </button>
                                <button
                                  onClick={() => setDeleteApp(app)}
                                  className="p-1.5 text-zinc-500 hover:text-red-400 transition-colors rounded-lg hover:bg-red-400/10"
                                  title={t('admin.apps.forceDelete')}
                                >
                                  <Trash2 size={14} />
                                </button>
                              </div>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </GlassCard>
            </div>
          )}
          {/* ==================== PLANS ==================== */}
          {activeTab === 'plans' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <p className="text-sm text-zinc-500">
                  {t('admin.plans.subtitle', { count: plans.length })}
                </p>
                <PillButton
                  variant="primary"
                  size="sm"
                  onClick={() => {
                    setPlanFormData(getDefaultPlanForm());
                    setShowCreatePlan(true);
                  }}
                  icon={<Plus size={14} />}
                >
                  {t('admin.plans.addPlan')}
                </PillButton>
              </div>

              {plans.length === 0 ? (
                <GlassCard>
                  <div className="text-center py-12 text-sm text-zinc-500">
                    {t('admin.plans.subtitle', { count: 0 })}
                  </div>
                </GlassCard>
              ) : (
                <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                  {plans.map((plan) => (
                    <PlanCard
                      key={plan.id}
                      plan={plan}
                      onEdit={() => {
                        setEditPlan(plan);
                        setPlanFormData(planToForm(plan));
                      }}
                      onDelete={() => setDeletePlanTarget(plan)}
                      onSetDefault={() => handleSetDefault(plan.id)}
                      isDark={isDark}
                    />
                  ))}
                </div>
              )}
            </div>
          )}

          {/* ==================== AI SETTINGS ==================== */}
          {activeTab === 'ai' && (
            <div className="space-y-4">
              {aiLoading ? (
                <div className="flex items-center justify-center py-16">
                  <Loader2 size={24} className="animate-spin text-amber-400" />
                </div>
              ) : (
                <GlassCard>
                  <div className="flex items-center gap-2 mb-2">
                    <Bot size={18} className="text-amber-400" />
                    <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {t('admin.ai.title')}
                    </h3>
                  </div>
                  <p className="text-xs text-zinc-500 mb-6">{t('admin.ai.description')}</p>

                  <div className="space-y-4">
                    {/* Enable toggle */}
                    <ToggleField
                      label={t('admin.ai.enabled')}
                      value={aiForm.aiEnabled}
                      onChange={(v) => setAiForm((prev) => ({ ...prev, aiEnabled: v }))}
                      isDark={isDark}
                    />

                    {/* API Key */}
                    <div>
                      <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
                        {t('admin.ai.apiKey')}
                      </label>
                      <input
                        type="password"
                        value={aiForm.apiKey}
                        onChange={(e) => setAiForm((prev) => ({ ...prev, apiKey: e.target.value }))}
                        placeholder={aiSettings?.apiKey || t('admin.ai.apiKeyPlaceholder')}
                        className={`w-full px-3 py-2 text-sm rounded-lg border transition-colors outline-none ${
                          isDark
                            ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200 focus:border-amber-500/50 placeholder:text-zinc-600'
                            : 'bg-white border-zinc-200 text-zinc-800 focus:border-amber-500/50 placeholder:text-zinc-400'
                        }`}
                      />
                      <p className="text-[11px] text-zinc-500 mt-1">{t('admin.ai.apiKeyHint')}</p>
                    </div>

                    {/* Model select */}
                    <div>
                      <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
                        {t('admin.ai.model')}
                      </label>
                      <select
                        value={aiForm.aiModel}
                        onChange={(e) => setAiForm((prev) => ({ ...prev, aiModel: e.target.value }))}
                        className={`w-full px-3 py-2 text-sm rounded-lg border transition-colors outline-none ${
                          isDark
                            ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200 focus:border-amber-500/50'
                            : 'bg-white border-zinc-200 text-zinc-800 focus:border-amber-500/50'
                        }`}
                      >
                        <optgroup label="Anthropic">
                          <option value="anthropic/claude-sonnet-4">Claude Sonnet 4 — recommended</option>
                          <option value="anthropic/claude-haiku-4.5">Claude Haiku 4.5 — fast &amp; cheap</option>
                          <option value="anthropic/claude-opus-4">Claude Opus 4 — most capable</option>
                        </optgroup>
                        <optgroup label="Google">
                          <option value="google/gemini-2.5-pro">Gemini 2.5 Pro</option>
                          <option value="google/gemini-2.5-flash">Gemini 2.5 Flash — fast &amp; cheap</option>
                        </optgroup>
                        <optgroup label="OpenAI">
                          <option value="openai/gpt-4o">GPT-4o</option>
                          <option value="openai/gpt-4o-mini">GPT-4o Mini — fast &amp; cheap</option>
                        </optgroup>
                        <optgroup label="DeepSeek">
                          <option value="deepseek/deepseek-chat-v3.1">DeepSeek V3.1 — best value</option>
                          <option value="deepseek/deepseek-r1">DeepSeek R1 — reasoning</option>
                        </optgroup>
                        <optgroup label="Mistral">
                          <option value="mistralai/codestral-2508">Codestral — code specialist</option>
                          <option value="mistralai/devstral-medium">Devstral Medium — dev agent</option>
                        </optgroup>
                        <optgroup label="Qwen">
                          <option value="qwen/qwen3-coder">Qwen3 Coder 480B — code specialist</option>
                          <option value="qwen/qwen3.6-plus">Qwen3.6 Plus — free</option>
                        </optgroup>
                      </select>
                    </div>

                    {/* Test Connection result */}
                    {aiTestResult && (
                      <div
                        className={`flex items-center gap-2 px-3 py-2 rounded-lg text-sm ${
                          aiTestResult.success
                            ? isDark ? 'bg-emerald-900/30 text-emerald-400 border border-emerald-800/50' : 'bg-emerald-50 text-emerald-700 border border-emerald-200'
                            : isDark ? 'bg-red-900/30 text-red-400 border border-red-800/50' : 'bg-red-50 text-red-700 border border-red-200'
                        }`}
                      >
                        {aiTestResult.success ? (
                          <>
                            <CheckCircle2 size={16} />
                            <span>{t('admin.ai.testSuccess', { model: aiTestResult.model })}</span>
                          </>
                        ) : (
                          <>
                            <XCircle size={16} />
                            <span>{aiTestResult.error || t('admin.ai.testFailed')}</span>
                          </>
                        )}
                      </div>
                    )}

                    {/* Action buttons */}
                    <div className="flex items-center gap-2 pt-2">
                      <PillButton
                        variant="primary"
                        size="sm"
                        onClick={handleSaveAISettings}
                        disabled={aiSaving}
                        icon={aiSaving ? <RefreshCw size={14} className="animate-spin" /> : <Check size={14} />}
                      >
                        {aiSaving ? t('common.saving') : t('common.save')}
                      </PillButton>
                      <PillButton
                        variant="ghost"
                        size="sm"
                        onClick={handleTestAIConnection}
                        disabled={aiTesting || (!aiForm.apiKey && !aiSettings?.apiKey)}
                        icon={aiTesting ? <RefreshCw size={14} className="animate-spin" /> : <Zap size={14} />}
                      >
                        {aiTesting ? t('admin.ai.testing') : t('admin.ai.testConnection')}
                      </PillButton>
                    </div>
                  </div>
                </GlassCard>
              )}
            </div>
          )}

          {/* ==================== CLEANUP ==================== */}
          {activeTab === 'cleanup' && (
            <div className="space-y-4">
              {cleanupLoading ? (
                <div className="flex items-center justify-center py-16">
                  <Loader2 size={24} className="animate-spin text-amber-400" />
                </div>
              ) : (
                <>
                  {/* Disk Usage Overview */}
                  <GlassCard>
                    <div className="flex items-center gap-2 mb-4">
                      <HardDrive size={18} className="text-amber-400" />
                      <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                        {t('admin.cleanup.diskUsage')}
                      </h3>
                      <PillButton
                        variant="ghost"
                        size="sm"
                        onClick={fetchCleanupSettings}
                        icon={<RefreshCw size={12} />}
                        className="ml-auto"
                      >
                        {t('common.refresh')}
                      </PillButton>
                    </div>

                    {diskUsage && (
                      <div className="space-y-4">
                        {/* Disk bar */}
                        <div>
                          <div className="flex items-center justify-between text-xs text-zinc-500 mb-1.5">
                            <span>{t('admin.cleanup.rootDisk')}</span>
                            <span>{diskUsage.diskPercent}% {t('admin.cleanup.used')}</span>
                          </div>
                          <div className={`h-3 rounded-full overflow-hidden ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`}>
                            <div
                              className={`h-full rounded-full transition-all duration-500 ${
                                parseInt(diskUsage.diskPercent) > 90 ? 'bg-red-500' :
                                parseInt(diskUsage.diskPercent) > 75 ? 'bg-amber-500' : 'bg-emerald-500'
                              }`}
                              style={{ width: `${diskUsage.diskPercent}%` }}
                            />
                          </div>
                          <div className="flex items-center justify-between text-[11px] text-zinc-500 mt-1">
                            <span>{formatBytes(diskUsage.diskUsed)} / {formatBytes(diskUsage.diskTotal)}</span>
                            <span>{formatBytes(diskUsage.diskAvailable)} {t('admin.cleanup.available')}</span>
                          </div>
                        </div>

                        {/* Docker breakdown */}
                        {diskUsage.docker && diskUsage.docker.length > 0 && (
                          <div>
                            <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-2">{t('admin.cleanup.dockerBreakdown')}</p>
                            <div className="grid grid-cols-3 gap-3">
                              {diskUsage.docker.map((item) => (
                                <div
                                  key={item.type}
                                  className={`px-3 py-2 rounded-lg ${isDark ? 'bg-zinc-800/50' : 'bg-zinc-50'}`}
                                >
                                  <p className={`text-xs font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{item.type}</p>
                                  <p className={`text-sm font-semibold mt-0.5 ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>{item.size}</p>
                                  <p className="text-[10px] text-zinc-500">{item.reclaimable} {t('admin.cleanup.reclaimable')}</p>
                                </div>
                              ))}
                            </div>
                          </div>
                        )}

                        {/* Image & container count */}
                        <div className="flex items-center gap-4 text-xs text-zinc-500">
                          {diskUsage.imageCount !== undefined && (
                            <span>{diskUsage.imageCount} {t('admin.cleanup.images')}</span>
                          )}
                          {diskUsage.activeContainerCount !== undefined && (
                            <span>{diskUsage.activeContainerCount} {t('admin.cleanup.activeContainers')}</span>
                          )}
                        </div>
                      </div>
                    )}
                  </GlassCard>

                  {/* Manual Cleanup */}
                  <GlassCard>
                    <div className="flex items-center gap-2 mb-2">
                      <Trash2 size={18} className="text-red-400" />
                      <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                        {t('admin.cleanup.manualCleanup')}
                      </h3>
                    </div>
                    <p className="text-xs text-zinc-500 mb-4">{t('admin.cleanup.manualDescription')}</p>

                    <PillButton
                      variant="danger"
                      size="sm"
                      onClick={handleTriggerCleanup}
                      disabled={cleanupRunning}
                      icon={cleanupRunning ? <RefreshCw size={14} className="animate-spin" /> : <Trash2 size={14} />}
                    >
                      {cleanupRunning ? t('admin.cleanup.running') : t('admin.cleanup.runNow')}
                    </PillButton>

                    {cleanupResult && (
                      <div className={`mt-4 px-3 py-2.5 rounded-lg text-sm ${
                        isDark ? 'bg-emerald-900/20 border border-emerald-800/30' : 'bg-emerald-50 border border-emerald-200'
                      }`}>
                        <p className={`text-xs font-medium mb-1 ${isDark ? 'text-emerald-400' : 'text-emerald-700'}`}>
                          {t('admin.cleanup.resultTitle')}
                        </p>
                        <div className="grid grid-cols-2 gap-1 text-[11px] text-zinc-500">
                          <span>{t('admin.cleanup.imagesRemoved')}: {cleanupResult.imagesRemoved}</span>
                          <span>{t('admin.cleanup.containersRemoved')}: {cleanupResult.containersRemoved}</span>
                          <span>{t('admin.cleanup.imageSpace')}: {formatBytes(cleanupResult.imagesReclaimed)}</span>
                          <span>{t('admin.cleanup.buildCache')}: {formatBytes(cleanupResult.buildCacheReclaimed)}</span>
                        </div>
                        <p className={`text-xs font-semibold mt-1 ${isDark ? 'text-emerald-400' : 'text-emerald-700'}`}>
                          {t('admin.cleanup.totalReclaimed')}: {formatBytes(cleanupResult.totalReclaimed)}
                        </p>
                      </div>
                    )}
                  </GlassCard>

                  {/* Auto Cleanup Settings */}
                  <GlassCard>
                    <div className="flex items-center gap-2 mb-2">
                      <Activity size={18} className="text-amber-400" />
                      <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                        {t('admin.cleanup.autoCleanup')}
                      </h3>
                    </div>
                    <p className="text-xs text-zinc-500 mb-6">{t('admin.cleanup.autoDescription')}</p>

                    <div className="space-y-4">
                      <ToggleField
                        label={t('admin.cleanup.enableAuto')}
                        value={cleanupForm.enabled}
                        onChange={(v) => setCleanupForm((prev) => ({ ...prev, enabled: v }))}
                        isDark={isDark}
                      />

                      <div>
                        <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
                          {t('admin.cleanup.checkInterval')}
                        </label>
                        <select
                          value={cleanupForm.intervalHours}
                          onChange={(e) => setCleanupForm((prev) => ({ ...prev, intervalHours: parseInt(e.target.value) }))}
                          className={`w-full px-3 py-2 text-sm rounded-lg border transition-colors outline-none ${
                            isDark
                              ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200 focus:border-amber-500/50'
                              : 'bg-white border-zinc-200 text-zinc-800 focus:border-amber-500/50'
                          }`}
                        >
                          <option value={6}>6 {t('admin.cleanup.hours')}</option>
                          <option value={12}>12 {t('admin.cleanup.hours')}</option>
                          <option value={24}>24 {t('admin.cleanup.hours')} ({t('admin.cleanup.recommended')})</option>
                          <option value={48}>48 {t('admin.cleanup.hours')}</option>
                          <option value={72}>72 {t('admin.cleanup.hours')}</option>
                        </select>
                      </div>

                      <div>
                        <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
                          {t('admin.cleanup.diskThreshold')}
                        </label>
                        <div className="flex items-center gap-3">
                          <input
                            type="range"
                            min={30}
                            max={95}
                            step={5}
                            value={cleanupForm.thresholdPercent}
                            onChange={(e) => setCleanupForm((prev) => ({ ...prev, thresholdPercent: parseInt(e.target.value) }))}
                            className="flex-1 accent-amber-500"
                          />
                          <span className={`text-sm font-mono w-12 text-right ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                            {cleanupForm.thresholdPercent}%
                          </span>
                        </div>
                        <p className="text-[11px] text-zinc-500 mt-1">{t('admin.cleanup.thresholdHint')}</p>
                      </div>

                      <div className="pt-2">
                        <PillButton
                          variant="primary"
                          size="sm"
                          onClick={handleSaveCleanupSettings}
                          disabled={cleanupSaving}
                          icon={cleanupSaving ? <RefreshCw size={14} className="animate-spin" /> : <Check size={14} />}
                        >
                          {cleanupSaving ? t('common.saving') : t('common.save')}
                        </PillButton>
                      </div>
                    </div>
                  </GlassCard>
                </>
              )}
              {appTotalPages > 1 && (
                <div className="flex items-center justify-center gap-3 mt-4">
                  <button
                    disabled={appPage <= 0}
                    onClick={() => setAppPage(appPage - 1)}
                    className={`px-3 py-1.5 text-xs rounded-lg border transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${isDark ? 'border-zinc-700 hover:bg-zinc-800' : 'border-zinc-200 hover:bg-zinc-100'}`}
                  >
                    {t('common.previous')}
                  </button>
                  <span className="text-xs text-zinc-500">{appPage + 1} / {appTotalPages}</span>
                  <button
                    disabled={appPage >= appTotalPages - 1}
                    onClick={() => setAppPage(appPage + 1)}
                    className={`px-3 py-1.5 text-xs rounded-lg border transition-colors disabled:opacity-40 disabled:cursor-not-allowed ${isDark ? 'border-zinc-700 hover:bg-zinc-800' : 'border-zinc-200 hover:bg-zinc-100'}`}
                  >
                    {t('common.next')}
                  </button>
                </div>
              )}
            </div>
          )}

          {/* ==================== AUDIT LOGS ==================== */}
          {activeTab === 'audit' && (
            <div className="space-y-4">
              {auditLoading && auditLogs.length === 0 ? (
                <div className="flex items-center justify-center py-16">
                  <Loader2 size={24} className="animate-spin text-amber-400" />
                </div>
              ) : (
                <>
                  {/* Header with view toggle + stats badge */}
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <div className={`flex rounded-lg border ${isDark ? 'border-zinc-700' : 'border-zinc-200'}`}>
                        <button
                          onClick={() => setAuditView('table')}
                          className={`px-3 py-1.5 text-xs rounded-l-lg transition-colors ${
                            auditView === 'table'
                              ? isDark ? 'bg-zinc-700 text-zinc-200' : 'bg-zinc-200 text-zinc-800'
                              : isDark ? 'text-zinc-500 hover:text-zinc-300' : 'text-zinc-400 hover:text-zinc-600'
                          }`}
                        >
                          <List size={14} />
                        </button>
                        <button
                          onClick={() => setAuditView('timeline')}
                          className={`px-3 py-1.5 text-xs rounded-r-lg transition-colors ${
                            auditView === 'timeline'
                              ? isDark ? 'bg-zinc-700 text-zinc-200' : 'bg-zinc-200 text-zinc-800'
                              : isDark ? 'text-zinc-500 hover:text-zinc-300' : 'text-zinc-400 hover:text-zinc-600'
                          }`}
                        >
                          <Activity size={14} />
                        </button>
                      </div>
                      {auditStats && auditStats.total24h > 0 && (
                        <span className={`text-[11px] px-2 py-0.5 rounded-full font-medium ${
                          isDark ? 'bg-amber-900/30 text-amber-400' : 'bg-amber-50 text-amber-700'
                        }`}>
                          {auditStats.total24h} {t('admin.audit.last24h')}
                        </span>
                      )}
                    </div>
                    <PillButton variant="ghost" size="sm" onClick={fetchAuditLogs} icon={<RefreshCw size={12} />}>
                      {t('common.refresh')}
                    </PillButton>
                  </div>

                  {/* Filters */}
                  <GlassCard>
                    <div className="flex flex-wrap gap-3">
                      <select
                        value={auditFilters.action || ''}
                        onChange={(e) => { setAuditFilters((f) => ({ ...f, action: e.target.value || undefined })); setAuditPage(0); }}
                        className={`px-3 py-1.5 text-xs rounded-lg border transition-colors outline-none ${
                          isDark ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200' : 'bg-white border-zinc-200 text-zinc-800'
                        }`}
                      >
                        <option value="">{t('admin.audit.allActions')}</option>
                        {['create', 'update', 'delete', 'deploy', 'restart', 'stop', 'maintenance', 'login'].map((a) => (
                          <option key={a} value={a}>{t(`admin.audit.actions.${a}`)}</option>
                        ))}
                      </select>
                      <select
                        value={auditFilters.resourceType || ''}
                        onChange={(e) => { setAuditFilters((f) => ({ ...f, resourceType: e.target.value || undefined })); setAuditPage(0); }}
                        className={`px-3 py-1.5 text-xs rounded-lg border transition-colors outline-none ${
                          isDark ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200' : 'bg-white border-zinc-200 text-zinc-800'
                        }`}
                      >
                        <option value="">{t('admin.audit.allResources')}</option>
                        {['app', 'user', 'service', 'plan', 'setting', 'deployment', 'alert', 'cleanup'].map((r) => (
                          <option key={r} value={r}>{t(`admin.audit.resources.${r}`)}</option>
                        ))}
                      </select>
                      <input
                        type="text"
                        placeholder={t('admin.audit.search')}
                        value={auditFilters.search || ''}
                        onChange={(e) => { setAuditFilters((f) => ({ ...f, search: e.target.value || undefined })); setAuditPage(0); }}
                        className={`px-3 py-1.5 text-xs rounded-lg border transition-colors outline-none flex-1 min-w-[150px] ${
                          isDark ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200 placeholder:text-zinc-600' : 'bg-white border-zinc-200 text-zinc-800 placeholder:text-zinc-400'
                        }`}
                      />
                    </div>
                  </GlassCard>

                  {/* TABLE VIEW */}
                  {auditView === 'table' && (
                    <GlassCard>
                      {auditLogs.length === 0 ? (
                        <div className="text-center py-12 text-sm text-zinc-500">{t('admin.audit.noLogs')}</div>
                      ) : (
                        <div className="space-y-1">
                          {auditLogs.map((log) => (
                            <div key={log.id}>
                              <button
                                onClick={() => toggleAuditExpand(log.id)}
                                className={`w-full flex items-center gap-3 px-3 py-2.5 rounded-lg text-left transition-colors ${
                                  isDark ? 'hover:bg-zinc-800/50' : 'hover:bg-zinc-50'
                                }`}
                              >
                                <span className="text-[11px] text-zinc-500 w-[70px] shrink-0 font-mono">
                                  {new Date(log.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                </span>
                                <span className={`text-xs w-[100px] shrink-0 truncate ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                                  {log.actorUsername}
                                </span>
                                <span className={`text-[10px] px-2 py-0.5 rounded-full border font-medium shrink-0 ${actionColor(log.action)}`}>
                                  {t(`admin.audit.actions.${log.action}`)}
                                </span>
                                <span className="flex items-center gap-1 text-zinc-500 shrink-0">
                                  {resourceIcon(log.resourceType)}
                                </span>
                                <span className={`text-xs truncate flex-1 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                                  {log.resourceName || log.resourceId}
                                </span>
                                {(log.oldValues || log.newValues) && (
                                  auditExpanded.has(log.id) ? <ChevronUp size={14} className="text-zinc-500 shrink-0" /> : <ChevronDown size={14} className="text-zinc-500 shrink-0" />
                                )}
                              </button>
                              {auditExpanded.has(log.id) && (log.oldValues || log.newValues) && (
                                <div className={`mx-3 mb-2 px-3 py-2 rounded-lg text-[11px] space-y-1 ${
                                  isDark ? 'bg-zinc-800/50' : 'bg-zinc-50'
                                }`}>
                                  {log.oldValues && Object.entries(log.oldValues).map(([key, val]) => (
                                    <div key={`old-${key}`} className="flex gap-2">
                                      <span className="text-zinc-500 w-[80px] shrink-0">{key}</span>
                                      <span className="text-red-400 line-through">{String(val)}</span>
                                      {log.newValues?.[key] !== undefined && (
                                        <span className="text-emerald-400">&rarr; {String(log.newValues[key])}</span>
                                      )}
                                    </div>
                                  ))}
                                  {log.newValues && Object.entries(log.newValues)
                                    .filter(([key]) => !log.oldValues?.[key])
                                    .map(([key, val]) => (
                                      <div key={`new-${key}`} className="flex gap-2">
                                        <span className="text-zinc-500 w-[80px] shrink-0">{key}</span>
                                        <span className="text-emerald-400">{String(val)}</span>
                                      </div>
                                    ))}
                                  {log.ipAddress && (
                                    <div className="flex gap-2 pt-1 border-t border-zinc-700/30">
                                      <span className="text-zinc-500 w-[80px] shrink-0">IP</span>
                                      <span className="text-zinc-400 font-mono">{log.ipAddress}</span>
                                    </div>
                                  )}
                                </div>
                              )}
                            </div>
                          ))}
                        </div>
                      )}
                      {/* Pagination */}
                      {auditTotal > 50 && (
                        <div className="flex items-center justify-between pt-3 mt-3 border-t border-zinc-700/30">
                          <span className="text-[11px] text-zinc-500">{auditTotal} {t('admin.audit.totalEvents')}</span>
                          <div className="flex gap-1">
                            {Array.from({ length: Math.min(Math.ceil(auditTotal / 50), 10) }, (_, i) => (
                              <button
                                key={i}
                                onClick={() => setAuditPage(i)}
                                className={`w-7 h-7 text-[11px] rounded-md transition-colors ${
                                  auditPage === i
                                    ? 'bg-amber-500 text-white'
                                    : isDark ? 'text-zinc-400 hover:bg-zinc-800' : 'text-zinc-600 hover:bg-zinc-100'
                                }`}
                              >
                                {i + 1}
                              </button>
                            ))}
                          </div>
                        </div>
                      )}
                    </GlassCard>
                  )}

                  {/* TIMELINE VIEW */}
                  {auditView === 'timeline' && (
                    <div className="space-y-6">
                      {auditLogs.length === 0 ? (
                        <GlassCard>
                          <div className="text-center py-12 text-sm text-zinc-500">{t('admin.audit.noLogs')}</div>
                        </GlassCard>
                      ) : (
                        groupLogsByDay(auditLogs).map((group) => (
                          <div key={group.label}>
                            <h4 className={`text-xs font-semibold uppercase tracking-wider mb-3 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                              {group.label}
                            </h4>
                            <div className={`relative pl-6 border-l-2 ${isDark ? 'border-zinc-700/50' : 'border-zinc-200'} space-y-3`}>
                              {group.logs.map((log) => (
                                <div key={log.id} className="relative">
                                  <div className={`absolute -left-[29px] w-4 h-4 rounded-full border-2 flex items-center justify-center ${
                                    log.action === 'create' ? 'border-emerald-500 bg-emerald-500/20' :
                                    log.action === 'delete' ? 'border-red-500 bg-red-500/20' :
                                    log.action === 'deploy' || log.action === 'restart' || log.action === 'stop' ? 'border-purple-500 bg-purple-500/20' :
                                    log.action === 'login' ? 'border-zinc-500 bg-zinc-500/20' :
                                    'border-blue-500 bg-blue-500/20'
                                  }`} />
                                  <button
                                    onClick={() => toggleAuditExpand(log.id)}
                                    className={`w-full text-left px-3 py-2 rounded-lg transition-colors ${
                                      isDark ? 'hover:bg-zinc-800/30' : 'hover:bg-zinc-50'
                                    }`}
                                  >
                                    <div className="flex items-center gap-2 flex-wrap">
                                      <span className={`text-xs font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                                        {log.actorUsername}
                                      </span>
                                      <span className={`text-[10px] px-1.5 py-0.5 rounded border ${actionColor(log.action)}`}>
                                        {t(`admin.audit.actions.${log.action}`)}
                                      </span>
                                      <span className="flex items-center gap-1 text-zinc-500 text-xs">
                                        {resourceIcon(log.resourceType)}
                                        <span className={`font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                                          {log.resourceName || log.resourceId}
                                        </span>
                                      </span>
                                      <span className="text-[10px] text-zinc-500 ml-auto">
                                        {new Date(log.createdAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                                      </span>
                                    </div>
                                    {auditExpanded.has(log.id) && (log.oldValues || log.newValues) && (
                                      <div className={`mt-2 px-3 py-2 rounded text-[11px] space-y-1 ${
                                        isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'
                                      }`}>
                                        {log.oldValues && Object.entries(log.oldValues).map(([key, val]) => (
                                          <div key={`old-${key}`} className="flex gap-2">
                                            <span className="text-zinc-500 w-[80px] shrink-0">{key}</span>
                                            <span className="text-red-400 line-through">{String(val)}</span>
                                            {log.newValues?.[key] !== undefined && (
                                              <span className="text-emerald-400">&rarr; {String(log.newValues[key])}</span>
                                            )}
                                          </div>
                                        ))}
                                        {log.newValues && Object.entries(log.newValues)
                                          .filter(([key]) => !log.oldValues?.[key])
                                          .map(([key, val]) => (
                                            <div key={`new-${key}`} className="flex gap-2">
                                              <span className="text-zinc-500 w-[80px] shrink-0">{key}</span>
                                              <span className="text-emerald-400">{String(val)}</span>
                                            </div>
                                          ))}
                                        {log.ipAddress && (
                                          <div className="flex gap-2 pt-1 border-t border-zinc-700/30">
                                            <span className="text-zinc-500 w-[80px] shrink-0">IP</span>
                                            <span className="text-zinc-400 font-mono">{log.ipAddress}</span>
                                          </div>
                                        )}
                                      </div>
                                    )}
                                  </button>
                                </div>
                              ))}
                            </div>
                          </div>
                        ))
                      )}
                      {/* Pagination for timeline */}
                      {auditTotal > 50 && (
                        <div className="flex items-center justify-center gap-1">
                          {Array.from({ length: Math.min(Math.ceil(auditTotal / 50), 10) }, (_, i) => (
                            <button
                              key={i}
                              onClick={() => setAuditPage(i)}
                              className={`w-7 h-7 text-[11px] rounded-md transition-colors ${
                                auditPage === i
                                  ? 'bg-amber-500 text-white'
                                  : isDark ? 'text-zinc-400 hover:bg-zinc-800' : 'text-zinc-600 hover:bg-zinc-100'
                              }`}
                            >
                              {i + 1}
                            </button>
                          ))}
                        </div>
                      )}
                    </div>
                  )}
                </>
              )}
            </div>
          )}
        </>
      )}

      {/* ==================== PLAN FORM MODAL ==================== */}
      {(showCreatePlan || editPlan) && (
        <PlanFormModal
          formData={planFormData}
          setFormData={setPlanFormData}
          isEdit={!!editPlan}
          onSave={editPlan ? handleUpdatePlan : handleCreatePlan}
          onClose={() => {
            setShowCreatePlan(false);
            setEditPlan(null);
          }}
          isDark={isDark}
        />
      )}

      {/* ==================== DELETE PLAN CONFIRM ==================== */}
      <ConfirmDialog
        open={!!deletePlanTarget}
        title={t('admin.plans.deleteConfirmTitle')}
        message={t('admin.plans.deleteConfirmMessage', { name: deletePlanTarget?.name })}
        confirmLabel={t('admin.plans.deletePlan')}
        variant="danger"
        onConfirm={handleDeletePlan}
        onCancel={() => setDeletePlanTarget(null)}
      />

      {/* ==================== ASSIGN PLAN MODAL ==================== */}
      {planUserTarget && (
        <AssignPlanModal
          user={planUserTarget}
          plans={plans}
          onAssign={(planId) => handleAssignPlan(planUserTarget.id, planId)}
          onClose={() => setPlanUserTarget(null)}
          isDark={isDark}
        />
      )}

      {/* ==================== ROLE CHANGE MODAL ==================== */}
      {roleChangeUser && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div
            className={`w-full max-w-sm rounded-2xl p-6 shadow-xl ${
              isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'
            }`}
          >
            <div className="flex items-center justify-between mb-4">
              <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                {t('admin.roleModal.title', { username: roleChangeUser.username })}
              </h3>
              <button onClick={() => setRoleChangeUser(null)} className="text-zinc-500 hover:text-zinc-300">
                <X size={16} />
              </button>
            </div>
            <p className="text-xs text-zinc-500 mb-4">
              {t('admin.roleModal.currentRole')} <span className="font-mono text-amber-400">{roleChangeUser.role}</span>
            </p>
            <div className="flex gap-2">
              <PillButton
                variant={roleChangeUser.role === 'user' ? 'primary' : 'secondary'}
                size="sm"
                className="flex-1"
                onClick={() => handleRoleChange(roleChangeUser, 'user')}
              >
                <UserCheck size={14} className="mr-1" /> {t('admin.roleModal.user')}
              </PillButton>
              <PillButton
                variant={roleChangeUser.role === 'admin' ? 'primary' : 'secondary'}
                size="sm"
                className="flex-1"
                onClick={() => handleRoleChange(roleChangeUser, 'admin')}
              >
                <Crown size={14} className="mr-1" /> {t('admin.roleModal.admin')}
              </PillButton>
            </div>
          </div>
        </div>
      )}

      {/* ==================== LIMITS MODAL ==================== */}
      {limitsApp && (
        <LimitsModal
          app={limitsApp}
          allApps={apps}
          vpsInfo={vpsInfo}
          editLimits={editLimits}
          setEditLimits={setEditLimits}
          onSave={handleUpdateLimits}
          onClose={() => setLimitsApp(null)}
          isDark={isDark}
        />
      )}

      {/* ==================== DELETE CONFIRM ==================== */}
      <ConfirmDialog
        open={!!deleteApp}
        title={t('admin.deleteDialog.title')}
        message={t('admin.deleteDialog.message', { name: deleteApp?.name })}
        confirmLabel={t('admin.deleteDialog.confirm')}
        variant="danger"
        onConfirm={handleForceDelete}
        onCancel={() => setDeleteApp(null)}
      />
    </div>
  );
}

function parseMemStr(s: string): number {
  s = s.trim().toLowerCase();
  if (!s) return 0;
  const suffix = s[s.length - 1];
  const num = parseFloat(s.slice(0, -1));
  if (isNaN(num)) return 0;
  if (suffix === 'g') return num * 1024 * 1024 * 1024;
  if (suffix === 'm') return num * 1024 * 1024;
  if (suffix === 'k') return num * 1024;
  return parseInt(s, 10) || 0;
}

interface LimitsModalProps {
  app: AdminApp;
  allApps: AdminApp[];
  vpsInfo: VPSInfo | null;
  editLimits: { cpu: string; memory: string; disk: string };
  setEditLimits: React.Dispatch<React.SetStateAction<{ cpu: string; memory: string; disk: string }>>;
  onSave: () => void;
  onClose: () => void;
  isDark: boolean;
}

function LimitsModal({ app, allApps, vpsInfo, editLimits, setEditLimits, onSave, onClose, isDark }: LimitsModalProps) {
  const { t } = useTranslation();

  // Calculate budget: what other apps use (excluding current app)
  let otherCpu = 0;
  let otherMem = 0;
  for (const a of allApps) {
    if (a.id === app.id) continue;
    otherCpu += a.resourceLimits?.cpu ? parseFloat(a.resourceLimits.cpu) : 0.5;
    otherMem += a.resourceLimits?.memory ? parseMemStr(a.resourceLimits.memory) : 512 * 1024 * 1024;
  }

  const maxCpuForApp = vpsInfo ? vpsInfo.availableCpu - otherCpu : Infinity;
  const maxMemForApp = vpsInfo ? vpsInfo.availableMemory - otherMem : Infinity;

  const isCpuExceeded = (opt: string) => parseFloat(opt) > maxCpuForApp;
  const isMemExceeded = (opt: string) => parseMemStr(opt) > maxMemForApp;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div
        className={`w-full max-w-md rounded-2xl p-6 shadow-xl ${
          isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'
        }`}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('admin.limitsModal.title', { name: app.name })}
          </h3>
          <button onClick={onClose} className="text-zinc-500 hover:text-zinc-300">
            <X size={16} />
          </button>
        </div>

        {/* Budget info */}
        {vpsInfo && (
          <div className={`rounded-lg p-3 mb-4 text-[11px] space-y-1 ${
            isDark ? 'bg-zinc-800/50 text-zinc-400' : 'bg-zinc-100 text-zinc-500'
          }`}>
            <div className="flex justify-between">
              <span>{t('admin.limitsModal.availableForAppsCpu')}</span>
              <span className="font-mono">{vpsInfo.availableCpu} {t('common.units.cores')}</span>
            </div>
            <div className="flex justify-between">
              <span>{t('admin.limitsModal.usedByOtherApps')}</span>
              <span className="font-mono">{otherCpu.toFixed(1)} {t('common.units.cores')}</span>
            </div>
            <div className={`flex justify-between font-medium ${maxCpuForApp < 0.25 ? 'text-red-400' : 'text-emerald-400'}`}>
              <span>{t('admin.limitsModal.budgetForThisApp')}</span>
              <span className="font-mono">{Math.max(0, maxCpuForApp).toFixed(1)} {t('common.units.cores')}</span>
            </div>
            <div className="border-t border-zinc-700/30 my-1.5" />
            <div className="flex justify-between">
              <span>{t('admin.limitsModal.availableForAppsRam')}</span>
              <span className="font-mono">{formatBytes(vpsInfo.availableMemory)}</span>
            </div>
            <div className="flex justify-between">
              <span>{t('admin.limitsModal.usedByOtherApps')}</span>
              <span className="font-mono">{formatBytes(otherMem)}</span>
            </div>
            <div className={`flex justify-between font-medium ${maxMemForApp < 256 * 1024 * 1024 ? 'text-red-400' : 'text-emerald-400'}`}>
              <span>{t('admin.limitsModal.budgetForThisApp')}</span>
              <span className="font-mono">{formatBytes(Math.max(0, maxMemForApp))}</span>
            </div>
          </div>
        )}

        <div className="space-y-4">
          <div>
            <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
              {t('admin.limitsModal.cpuCores')}
            </label>
            <div className="flex gap-1.5 flex-wrap">
              {CPU_OPTIONS.map((opt) => {
                const exceeded = isCpuExceeded(opt);
                return (
                  <button
                    key={opt}
                    onClick={() => !exceeded && setEditLimits((p) => ({ ...p, cpu: opt }))}
                    disabled={exceeded}
                    className={`px-3 py-1.5 text-xs rounded-lg border transition-all ${
                      exceeded
                        ? 'opacity-30 cursor-not-allowed border-zinc-800 text-zinc-600 line-through'
                        : editLimits.cpu === opt
                          ? 'bg-amber-500/20 text-amber-400 border-amber-500/30'
                          : isDark
                            ? 'text-zinc-400 border-zinc-800 hover:border-zinc-600'
                            : 'text-zinc-600 border-zinc-200 hover:border-zinc-400'
                    }`}
                  >
                    {opt}
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
              {t('admin.limitsModal.memory')}
            </label>
            <div className="flex gap-1.5 flex-wrap">
              {MEMORY_OPTIONS.map((opt) => {
                const exceeded = isMemExceeded(opt);
                return (
                  <button
                    key={opt}
                    onClick={() => !exceeded && setEditLimits((p) => ({ ...p, memory: opt }))}
                    disabled={exceeded}
                    className={`px-3 py-1.5 text-xs rounded-lg border transition-all ${
                      exceeded
                        ? 'opacity-30 cursor-not-allowed border-zinc-800 text-zinc-600 line-through'
                        : editLimits.memory === opt
                          ? 'bg-amber-500/20 text-amber-400 border-amber-500/30'
                          : isDark
                            ? 'text-zinc-400 border-zinc-800 hover:border-zinc-600'
                            : 'text-zinc-600 border-zinc-200 hover:border-zinc-400'
                    }`}
                  >
                    {opt}
                  </button>
                );
              })}
            </div>
          </div>

          <div>
            <label className="block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5">
              {t('admin.limitsModal.disk')}
            </label>
            <div className="flex gap-1.5 flex-wrap">
              {DISK_OPTIONS.map((opt) => (
                <button
                  key={opt}
                  onClick={() => setEditLimits((p) => ({ ...p, disk: opt }))}
                  className={`px-3 py-1.5 text-xs rounded-lg border transition-all ${
                    editLimits.disk === opt
                      ? 'bg-amber-500/20 text-amber-400 border-amber-500/30'
                      : isDark
                        ? 'text-zinc-400 border-zinc-800 hover:border-zinc-600'
                        : 'text-zinc-600 border-zinc-200 hover:border-zinc-400'
                  }`}
                >
                  {opt}
                </button>
              ))}
            </div>
          </div>
        </div>

        <div className="flex gap-2 mt-6">
          <PillButton variant="secondary" size="sm" className="flex-1" onClick={onClose}>
            {t('common.cancel')}
          </PillButton>
          <PillButton variant="primary" size="sm" className="flex-1" onClick={onSave}>
            <Check size={14} className="mr-1" /> {t('admin.limitsModal.saveLimits')}
          </PillButton>
        </div>
      </div>
    </div>
  );
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(1024));
  const val = bytes / Math.pow(1024, i);
  return `${val < 10 ? val.toFixed(1) : Math.round(val)} ${units[i]}`;
}

interface ResourceBarProps {
  used: number;
  total: number;
  label: string;
  color: 'blue' | 'emerald' | 'amber';
  isDark: boolean;
}

function ResourceBar({ used, total, label, color, isDark }: ResourceBarProps) {
  const pct = total > 0 ? Math.min((used / total) * 100, 100) : 0;
  const colorMap = {
    blue: { bar: 'bg-blue-500', bg: isDark ? 'bg-zinc-800' : 'bg-zinc-200' },
    emerald: { bar: 'bg-emerald-500', bg: isDark ? 'bg-zinc-800' : 'bg-zinc-200' },
    amber: { bar: 'bg-amber-500', bg: isDark ? 'bg-zinc-800' : 'bg-zinc-200' },
  };
  const warn = pct > 80;
  return (
    <div className="space-y-1">
      <div className={`w-full h-2 rounded-full overflow-hidden ${colorMap[color].bg}`}>
        <div
          className={`h-full rounded-full transition-all duration-500 ${warn ? 'bg-red-500' : colorMap[color].bar}`}
          style={{ width: `${pct}%` }}
        />
      </div>
      <p className={`text-[10px] ${warn ? 'text-red-400' : 'text-zinc-500'}`}>{label}</p>
    </div>
  );
}

interface StatCardProps {
  icon: React.ReactNode;
  label: string;
  value: string | number;
  sub?: string;
  color: string;
}

function StatCard({ icon, label, value, sub, color }: StatCardProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  return (
    <GlassCard>
      <div className="flex items-center gap-3 mb-3">
        <div className={`flex items-center justify-center w-10 h-10 rounded-xl ${color}`}>
          {icon}
        </div>
        <span className="text-xs font-medium text-zinc-500 uppercase tracking-wider">{label}</span>
      </div>
      <p className={`text-2xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
        {value}
      </p>
      {sub && <p className="text-[11px] text-zinc-500 mt-1">{sub}</p>}
    </GlassCard>
  );
}

/* ==================== PLAN CARD ==================== */

interface PlanCardProps {
  plan: Plan;
  onEdit: () => void;
  onDelete: () => void;
  onSetDefault: () => void;
  isDark: boolean;
}

function PlanCard({ plan, onEdit, onDelete, onSetDefault, isDark }: PlanCardProps) {
  const { t } = useTranslation();
  return (
    <GlassCard>
      <div className="flex items-start justify-between mb-3">
        <div>
          <h3 className={`text-[15px] font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {plan.name}
          </h3>
          {plan.description && (
            <p className="text-[11px] text-zinc-500 mt-0.5">{plan.description}</p>
          )}
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={onEdit}
            className="p-1.5 text-zinc-500 hover:text-amber-400 transition-colors rounded-lg hover:bg-amber-400/10"
            title={t('admin.plans.editPlan')}
          >
            <Pencil size={14} />
          </button>
          <button
            onClick={onDelete}
            className="p-1.5 text-zinc-500 hover:text-red-400 transition-colors rounded-lg hover:bg-red-400/10"
            title={t('admin.plans.deletePlan')}
          >
            <Trash2 size={14} />
          </button>
        </div>
      </div>

      {/* Price */}
      <div className="flex items-baseline gap-1 mb-3">
        <span className={`text-xl font-bold tracking-tight ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
          {plan.price === 0 ? t('landing.pricing.free') : `${plan.currency === 'BRL' ? 'R$' : plan.currency === 'EUR' ? '\u20AC' : '$'}${plan.price}`}
        </span>
        {plan.price > 0 && (
          <span className="text-[11px] text-zinc-500">
            /{plan.billingCycle === 'monthly' ? t('landing.pricing.mo') : t('landing.pricing.yr')}
          </span>
        )}
      </div>

      {/* Badges */}
      <div className="flex flex-wrap gap-1.5 mb-3">
        {plan.isDefault && (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
            <Star size={10} />
            {t('admin.plans.default')}
          </span>
        )}
        {plan.highlighted && (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-full bg-amber-500/15 text-amber-400 border border-amber-500/30">
            {t('admin.plans.highlighted')}
          </span>
        )}
        {!plan.isActive && (
          <span className="inline-flex items-center gap-1 text-[10px] font-semibold px-2 py-0.5 rounded-full bg-red-500/15 text-red-400 border border-red-500/30">
            {t('admin.plans.inactive')}
          </span>
        )}
      </div>

      {/* Limits */}
      <div className="space-y-1 text-[11px] text-zinc-400 mb-4">
        <div>{t('admin.plans.maxApps')}: {plan.maxApps}</div>
        <div>{t('admin.plans.maxCpuPerApp')}: {plan.maxCpuPerApp}</div>
        <div>{t('admin.plans.maxMemoryPerApp')}: {plan.maxMemoryPerApp.toUpperCase()}</div>
        <div>{t('admin.plans.maxDiskPerApp')}: {plan.maxDiskPerApp.toUpperCase()}</div>
        <div>{t('admin.plans.maxServicesPerApp')}: {plan.maxServicesPerApp}</div>
      </div>

      {/* Features */}
      {plan.features.length > 0 && (
        <div className="space-y-1 mb-4">
          {plan.features.map((feat, i) => (
            <div key={i} className="flex items-center gap-1.5 text-[11px] text-zinc-300">
              <Check size={10} className="text-emerald-400 flex-shrink-0" />
              {feat}
            </div>
          ))}
        </div>
      )}

      {/* Set Default */}
      {!plan.isDefault && (
        <PillButton
          variant="ghost"
          size="sm"
          className="w-full"
          onClick={onSetDefault}
        >
          {t('admin.plans.setDefault')}
        </PillButton>
      )}
    </GlassCard>
  );
}

/* ==================== PLAN FORM MODAL ==================== */

interface PlanFormModalProps {
  formData: CreatePlanPayload;
  setFormData: React.Dispatch<React.SetStateAction<CreatePlanPayload>>;
  isEdit: boolean;
  onSave: () => void;
  onClose: () => void;
  isDark: boolean;
}

function PlanFormModal({ formData, setFormData, isEdit, onSave, onClose, isDark }: PlanFormModalProps) {
  const { t } = useTranslation();
  const [newFeature, setNewFeature] = useState('');

  const inputClass = `w-full px-3 py-2 text-sm rounded-lg border transition-colors outline-none ${
    isDark
      ? 'bg-zinc-800/50 border-zinc-700 text-zinc-200 focus:border-amber-500/50 placeholder:text-zinc-600'
      : 'bg-white border-zinc-200 text-zinc-800 focus:border-amber-500/50 placeholder:text-zinc-400'
  }`;

  const labelClass = 'block text-[11px] text-zinc-500 uppercase tracking-wider mb-1.5';

  const addFeature = () => {
    const trimmed = newFeature.trim();
    if (trimmed && !formData.features.includes(trimmed)) {
      setFormData((prev) => ({ ...prev, features: [...prev.features, trimmed] }));
      setNewFeature('');
    }
  };

  const removeFeature = (index: number) => {
    setFormData((prev) => ({
      ...prev,
      features: prev.features.filter((_, i) => i !== index),
    }));
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm overflow-y-auto py-8">
      <div
        className={`w-full max-w-lg rounded-2xl p-6 shadow-xl ${
          isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'
        }`}
      >
        <div className="flex items-center justify-between mb-5">
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {isEdit ? t('admin.plans.editPlan') : t('admin.plans.createPlan')}
          </h3>
          <button onClick={onClose} className="text-zinc-500 hover:text-zinc-300">
            <X size={16} />
          </button>
        </div>

        <div className="space-y-4 max-h-[60vh] overflow-y-auto pr-1">
          {/* Basic Info */}
          <div>
            <label className={labelClass}>{t('admin.plans.planName')}</label>
            <input
              type="text"
              value={formData.name}
              onChange={(e) => setFormData((prev) => ({ ...prev, name: e.target.value }))}
              className={inputClass}
              placeholder="e.g. Starter"
            />
          </div>

          <div>
            <label className={labelClass}>{t('admin.plans.description')}</label>
            <input
              type="text"
              value={formData.description}
              onChange={(e) => setFormData((prev) => ({ ...prev, description: e.target.value }))}
              className={inputClass}
              placeholder="e.g. Perfect for hobby projects"
            />
          </div>

          <div className="grid grid-cols-3 gap-3">
            <div>
              <label className={labelClass}>{t('admin.plans.price')}</label>
              <input
                type="number"
                min="0"
                step="0.01"
                value={formData.price}
                onChange={(e) => setFormData((prev) => ({ ...prev, price: parseFloat(e.target.value) || 0 }))}
                className={inputClass}
              />
            </div>
            <div>
              <label className={labelClass}>{t('admin.plans.currency')}</label>
              <select
                value={formData.currency}
                onChange={(e) => setFormData((prev) => ({ ...prev, currency: e.target.value }))}
                className={inputClass}
              >
                <option value="USD">USD</option>
                <option value="BRL">BRL</option>
                <option value="EUR">EUR</option>
              </select>
            </div>
            <div>
              <label className={labelClass}>{t('admin.plans.billingCycle')}</label>
              <select
                value={formData.billingCycle}
                onChange={(e) => setFormData((prev) => ({ ...prev, billingCycle: e.target.value }))}
                className={inputClass}
              >
                <option value="monthly">{t('admin.plans.monthly')}</option>
                <option value="yearly">{t('admin.plans.yearly')}</option>
              </select>
            </div>
          </div>

          {/* Limits */}
          <div className={`border-t pt-4 ${isDark ? 'border-zinc-800' : 'border-zinc-200'}`}>
            <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-3 font-medium">{t('admin.plans.limits')}</p>
            <div className="grid grid-cols-2 gap-3">
              <div>
                <label className={labelClass}>{t('admin.plans.maxApps')}</label>
                <input
                  type="number"
                  min="1"
                  value={formData.maxApps}
                  onChange={(e) => setFormData((prev) => ({ ...prev, maxApps: parseInt(e.target.value) || 1 }))}
                  className={inputClass}
                />
              </div>
              <div>
                <label className={labelClass}>{t('admin.plans.maxCpuPerApp')}</label>
                <input
                  type="number"
                  min="0.25"
                  step="0.25"
                  value={formData.maxCpuPerApp}
                  onChange={(e) => setFormData((prev) => ({ ...prev, maxCpuPerApp: parseFloat(e.target.value) || 0.25 }))}
                  className={inputClass}
                />
              </div>
              <div>
                <label className={labelClass}>{t('admin.plans.maxMemoryPerApp')}</label>
                <select
                  value={formData.maxMemoryPerApp}
                  onChange={(e) => setFormData((prev) => ({ ...prev, maxMemoryPerApp: e.target.value }))}
                  className={inputClass}
                >
                  {MEMORY_OPTIONS.map((opt) => (
                    <option key={opt} value={opt}>{opt.toUpperCase()}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className={labelClass}>{t('admin.plans.maxDiskPerApp')}</label>
                <select
                  value={formData.maxDiskPerApp}
                  onChange={(e) => setFormData((prev) => ({ ...prev, maxDiskPerApp: e.target.value }))}
                  className={inputClass}
                >
                  {DISK_OPTIONS.map((opt) => (
                    <option key={opt} value={opt}>{opt.toUpperCase()}</option>
                  ))}
                </select>
              </div>
              <div>
                <label className={labelClass}>{t('admin.plans.maxServicesPerApp')}</label>
                <input
                  type="number"
                  min="0"
                  value={formData.maxServicesPerApp}
                  onChange={(e) => setFormData((prev) => ({ ...prev, maxServicesPerApp: parseInt(e.target.value) || 0 }))}
                  className={inputClass}
                />
              </div>
              <div>
                <label className={labelClass}>{t('admin.plans.sortOrder')}</label>
                <input
                  type="number"
                  min="0"
                  value={formData.sortOrder}
                  onChange={(e) => setFormData((prev) => ({ ...prev, sortOrder: parseInt(e.target.value) || 0 }))}
                  className={inputClass}
                />
              </div>
            </div>
          </div>

          {/* Feature Flags */}
          <div className={`border-t pt-4 ${isDark ? 'border-zinc-800' : 'border-zinc-200'}`}>
            <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-3 font-medium">{t('admin.plans.featureFlags')}</p>
            <div className="space-y-2">
              <ToggleField
                label={t('admin.plans.autoDeployEnabled')}
                value={formData.autoDeployEnabled}
                onChange={(v) => setFormData((prev) => ({ ...prev, autoDeployEnabled: v }))}
                isDark={isDark}
              />
              <ToggleField
                label={t('admin.plans.customDomainEnabled')}
                value={formData.customDomainEnabled}
                onChange={(v) => setFormData((prev) => ({ ...prev, customDomainEnabled: v }))}
                isDark={isDark}
              />
              <ToggleField
                label={t('admin.plans.priorityBuilds')}
                value={formData.priorityBuilds}
                onChange={(v) => setFormData((prev) => ({ ...prev, priorityBuilds: v }))}
                isDark={isDark}
              />
              <ToggleField
                label={t('admin.plans.highlightPlan')}
                value={formData.highlighted}
                onChange={(v) => setFormData((prev) => ({ ...prev, highlighted: v }))}
                isDark={isDark}
              />
            </div>
          </div>

          {/* Display Features */}
          <div className={`border-t pt-4 ${isDark ? 'border-zinc-800' : 'border-zinc-200'}`}>
            <p className="text-[11px] text-zinc-500 uppercase tracking-wider mb-3 font-medium">{t('admin.plans.displayFeatures')}</p>
            <div className="space-y-2">
              {formData.features.map((feat, i) => (
                <div key={i} className="flex items-center gap-2">
                  <Check size={12} className="text-emerald-400 flex-shrink-0" />
                  <span className={`flex-1 text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{feat}</span>
                  <button
                    onClick={() => removeFeature(i)}
                    className="p-1 text-zinc-500 hover:text-red-400 transition-colors"
                  >
                    <X size={12} />
                  </button>
                </div>
              ))}
              <div className="flex items-center gap-2">
                <input
                  type="text"
                  value={newFeature}
                  onChange={(e) => setNewFeature(e.target.value)}
                  onKeyDown={(e) => e.key === 'Enter' && (e.preventDefault(), addFeature())}
                  placeholder={t('admin.plans.featurePlaceholder')}
                  className={inputClass}
                />
                <button
                  onClick={addFeature}
                  className="p-2 text-zinc-500 hover:text-amber-400 transition-colors rounded-lg hover:bg-amber-400/10 flex-shrink-0"
                >
                  <Plus size={14} />
                </button>
              </div>
            </div>
          </div>
        </div>

        <div className="flex gap-2 mt-6">
          <PillButton variant="secondary" size="sm" className="flex-1" onClick={onClose}>
            {t('common.cancel')}
          </PillButton>
          <PillButton variant="primary" size="sm" className="flex-1" onClick={onSave} disabled={!formData.name.trim()}>
            <Check size={14} className="mr-1" />
            {isEdit ? t('admin.plans.updatePlan') : t('admin.plans.createPlan')}
          </PillButton>
        </div>
      </div>
    </div>
  );
}

/* ==================== TOGGLE FIELD ==================== */

interface ToggleFieldProps {
  label: string;
  value: boolean;
  onChange: (v: boolean) => void;
  isDark: boolean;
}

function ToggleField({ label, value, onChange, isDark }: ToggleFieldProps) {
  return (
    <button
      type="button"
      onClick={() => onChange(!value)}
      className={`flex items-center justify-between w-full px-3 py-2 rounded-lg transition-colors ${
        isDark ? 'hover:bg-zinc-800/50' : 'hover:bg-zinc-50'
      }`}
    >
      <span className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{label}</span>
      {value ? (
        <ToggleRight size={20} className="text-amber-400" />
      ) : (
        <ToggleLeft size={20} className="text-zinc-600" />
      )}
    </button>
  );
}

/* ==================== ASSIGN PLAN MODAL ==================== */

interface AssignPlanModalProps {
  user: AdminUser;
  plans: Plan[];
  onAssign: (planId: string) => void;
  onClose: () => void;
  isDark: boolean;
}

function AssignPlanModal({ user, plans, onAssign, onClose, isDark }: AssignPlanModalProps) {
  const { t } = useTranslation();
  const activePlans = plans.filter((p) => p.isActive);

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
      <div
        className={`w-full max-w-sm rounded-2xl p-6 shadow-xl ${
          isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'
        }`}
      >
        <div className="flex items-center justify-between mb-4">
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            {t('admin.plans.assignPlan')}
          </h3>
          <button onClick={onClose} className="text-zinc-500 hover:text-zinc-300">
            <X size={16} />
          </button>
        </div>
        <p className="text-xs text-zinc-500 mb-4">
          {t('admin.plans.selectPlan', { username: user.username })}
        </p>
        <div className="space-y-2">
          {activePlans.map((plan) => (
            <button
              key={plan.id}
              onClick={() => onAssign(plan.id)}
              className={`w-full flex items-center justify-between px-4 py-3 rounded-xl border transition-all ${
                isDark
                  ? 'border-zinc-800 hover:border-amber-500/30 hover:bg-amber-500/5'
                  : 'border-zinc-200 hover:border-amber-500/30 hover:bg-amber-50'
              }`}
            >
              <div className="text-left">
                <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                  {plan.name}
                </p>
                <p className="text-[11px] text-zinc-500">
                  {plan.price === 0 ? t('landing.pricing.free') : `${plan.currency === 'BRL' ? 'R$' : plan.currency === 'EUR' ? '\u20AC' : '$'}${plan.price}/${plan.billingCycle === 'monthly' ? t('landing.pricing.mo') : t('landing.pricing.yr')}`}
                </p>
              </div>
              {plan.isDefault && (
                <span className="text-[10px] font-semibold px-2 py-0.5 rounded-full bg-emerald-500/15 text-emerald-400 border border-emerald-500/30">
                  {t('admin.plans.default')}
                </span>
              )}
            </button>
          ))}
        </div>
      </div>
    </div>
  );
}

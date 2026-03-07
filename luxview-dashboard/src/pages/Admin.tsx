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
} from 'lucide-react';
import { GlassCard } from '../components/common/GlassCard';
import { PillButton } from '../components/common/PillButton';
import { StatusDot } from '../components/common/StatusDot';
import { AppStatusBadge } from '../components/apps/AppStatusBadge';
import { ConfirmDialog } from '../components/common/ConfirmDialog';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { adminApi, type AdminStats, type AdminUser, type AdminApp, type VPSInfo } from '../api/admin';
import { formatRelativeTime } from '../lib/format';

type Tab = 'overview' | 'users' | 'apps';

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
  ];

  const [activeTab, setActiveTab] = useState<Tab>('overview');
  const [stats, setStats] = useState<AdminStats | null>(null);
  const [vpsInfo, setVpsInfo] = useState<VPSInfo | null>(null);
  const [users, setUsers] = useState<AdminUser[]>([]);
  const [apps, setApps] = useState<AdminApp[]>([]);
  const [loading, setLoading] = useState(true);
  const [userSearch, setUserSearch] = useState('');
  const [appSearch, setAppSearch] = useState('');

  // Modals
  const [roleChangeUser, setRoleChangeUser] = useState<AdminUser | null>(null);
  const [limitsApp, setLimitsApp] = useState<AdminApp | null>(null);
  const [deleteApp, setDeleteApp] = useState<AdminApp | null>(null);
  const [editLimits, setEditLimits] = useState({ cpu: '1.0', memory: '512m', disk: '5g' });

  const fetchData = useCallback(async () => {
    setLoading(true);
    try {
      const [statsData, vpsData, usersData, appsData] = await Promise.all([
        adminApi.stats(),
        adminApi.vpsInfo(),
        adminApi.listUsers(100, 0),
        adminApi.listApps(100, 0),
      ]);
      setStats(statsData);
      setVpsInfo(vpsData);
      setUsers(usersData.users ?? []);
      setApps(appsData.apps ?? []);
    } catch {
      addNotification({ type: 'error', title: t('admin.failedToLoad') });
    } finally {
      setLoading(false);
    }
  }, [addNotification, t]);

  useEffect(() => {
    fetchData();
  }, [fetchData]);

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

  const filteredUsers = users.filter(
    (u) =>
      !userSearch ||
      u.username.toLowerCase().includes(userSearch.toLowerCase()) ||
      u.email.toLowerCase().includes(userSearch.toLowerCase()),
  );

  const filteredApps = apps.filter(
    (a) =>
      !appSearch ||
      a.name.toLowerCase().includes(appSearch.toLowerCase()) ||
      a.subdomain.toLowerCase().includes(appSearch.toLowerCase()),
  );

  const ownerMap = new Map(users.map((u) => [u.id, u.username]));

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
                  onChange={(e) => setUserSearch(e.target.value)}
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
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.joined')}</th>
                        <th className="text-left text-[11px] font-medium uppercase tracking-wider px-6 py-3">{t('admin.users.tableHeaders.actions')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {filteredUsers.length === 0 ? (
                        <tr>
                          <td colSpan={5} className="text-center py-12 text-sm text-zinc-500">
                            {t('admin.users.noUsersFound')}
                          </td>
                        </tr>
                      ) : (
                        filteredUsers.map((u) => (
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
                            <td className="px-6 py-3 text-xs text-zinc-500">
                              {formatRelativeTime(u.createdAt)}
                            </td>
                            <td className="px-6 py-3">
                              <PillButton
                                variant="ghost"
                                size="sm"
                                onClick={() => setRoleChangeUser(u)}
                              >
                                {t('admin.users.changeRole')}
                              </PillButton>
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
                  onChange={(e) => setAppSearch(e.target.value)}
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
                      {filteredApps.length === 0 ? (
                        <tr>
                          <td colSpan={7} className="text-center py-12 text-sm text-zinc-500">
                            {t('admin.apps.noAppsFound')}
                          </td>
                        </tr>
                      ) : (
                        filteredApps.map((app) => (
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
        </>
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

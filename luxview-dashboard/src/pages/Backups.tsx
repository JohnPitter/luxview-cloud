import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Database,
  Download,
  Trash2,
  RotateCcw,
  Play,
  Save,
  Loader2,
  AlertTriangle,
  CheckCircle2,
  Clock,
  HardDrive,
  Settings2,
  RefreshCw,
} from 'lucide-react';
import { useThemeStore } from '../stores/theme.store';
import { useNotificationsStore } from '../stores/notifications.store';
import { backupsApi, type Backup, type BackupSettings } from '../api/backups';

const DB_OPTIONS = [
  { value: 'pg-platform', label: 'PostgreSQL (Platform)' },
  { value: 'pg-shared', label: 'PostgreSQL (Shared)' },
  { value: 'mongo-shared', label: 'MongoDB (Shared)' },
  { value: 'redis-shared', label: 'Redis (Shared)' },
];

const SCHEDULE_OPTIONS = [
  { value: 'daily', labelKey: 'backups.schedule.daily' },
  { value: 'weekly', labelKey: 'backups.schedule.weekly' },
  { value: 'monthly', labelKey: 'backups.schedule.monthly' },
];

const RETENTION_OPTIONS = [
  { value: 7, labelKey: 'backups.retention.7' },
  { value: 14, labelKey: 'backups.retention.14' },
  { value: 30, labelKey: 'backups.retention.30' },
  { value: 60, labelKey: 'backups.retention.60' },
];

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(1))} ${sizes[i]}`;
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`;
  const s = Math.floor(ms / 1000);
  if (s < 60) return `${s}s`;
  return `${Math.floor(s / 60)}m ${s % 60}s`;
}

function StatusBadge({ status }: { status: Backup['status'] }) {
  const config = {
    running: { icon: Loader2, color: 'text-blue-400 bg-blue-400/10', animate: 'animate-spin' },
    completed: { icon: CheckCircle2, color: 'text-emerald-400 bg-emerald-400/10', animate: '' },
    failed: { icon: AlertTriangle, color: 'text-red-400 bg-red-400/10', animate: '' },
    restoring: { icon: RotateCcw, color: 'text-amber-400 bg-amber-400/10', animate: 'animate-spin' },
  }[status];

  const Icon = config.icon;

  return (
    <span className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-xs font-medium ${config.color}`}>
      <Icon size={14} className={config.animate} />
      {status}
    </span>
  );
}

export function Backups() {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [settings, setSettings] = useState<BackupSettings | null>(null);
  const [backups, setBackups] = useState<Backup[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [page, setPage] = useState(0);
  const [restoreId, setRestoreId] = useState<string | null>(null);
  const [restoreConfirm, setRestoreConfirm] = useState('');
  const limit = 20;

  const loadData = useCallback(async () => {
    try {
      const [settingsData, backupsData] = await Promise.all([
        backupsApi.getSettings(),
        backupsApi.list(limit, page * limit),
      ]);
      setSettings({ ...settingsData, databases: settingsData.databases || [] });
      setBackups(backupsData.backups || []);
      setTotal(backupsData.total);
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.loadFailed') });
    } finally {
      setLoading(false);
    }
  }, [page, addNotification, t]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Auto-refresh while any backup is running
  useEffect(() => {
    const hasRunning = backups.some((b) => b.status === 'running' || b.status === 'restoring');
    if (!hasRunning) return;
    const interval = setInterval(loadData, 5000);
    return () => clearInterval(interval);
  }, [backups, loadData]);

  const handleSaveSettings = async () => {
    if (!settings) return;
    setSaving(true);
    try {
      await backupsApi.updateSettings(settings);
      addNotification({ type: 'success', title: t('backups.settingsSaved') });
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.saveFailed') });
    } finally {
      setSaving(false);
    }
  };

  const handleTrigger = async () => {
    try {
      await backupsApi.trigger(settings?.databases);
      addNotification({ type: 'info', title: t('backups.backupStarted') });
      setTimeout(loadData, 1000);
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.triggerFailed') });
    }
  };

  const handleDelete = async (id: string) => {
    try {
      await backupsApi.remove(id);
      addNotification({ type: 'success', title: t('backups.backupDeleted') });
      loadData();
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.deleteFailed') });
    }
  };

  const handleRestore = async () => {
    if (!restoreId || restoreConfirm !== 'RESTORE') return;
    try {
      await backupsApi.restore(restoreId, restoreConfirm);
      addNotification({ type: 'info', title: t('backups.restoreStarted') });
      setRestoreId(null);
      setRestoreConfirm('');
      setTimeout(loadData, 1000);
    } catch {
      addNotification({ type: 'error', title: t('backups.errors.restoreFailed') });
    }
  };

  const cardClass = `rounded-xl border ${isDark ? 'bg-zinc-900/50 border-zinc-800/50' : 'bg-white border-zinc-200'}`;
  const inputClass = `w-full rounded-lg px-3 py-2 text-sm border transition-colors ${isDark ? 'bg-zinc-800 border-zinc-700 text-white' : 'bg-white border-zinc-300 text-zinc-900'} focus:outline-none focus:ring-2 focus:ring-amber-400/50`;

  if (loading) {
    return (
      <div className="flex items-center justify-center h-64">
        <Loader2 size={24} className="animate-spin text-amber-400" />
      </div>
    );
  }

  const totalPages = Math.ceil(total / limit);
  const restoreBackup = backups.find((b) => b.id === restoreId);

  return (
    <div className="space-y-6 animate-in fade-in duration-300">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-xl bg-amber-400/10">
            <HardDrive size={24} className="text-amber-400" />
          </div>
          <div>
            <h1 className={`text-2xl font-bold tracking-tight ${isDark ? 'text-white' : 'text-zinc-900'}`}>
              {t('backups.title')}
            </h1>
            <p className={`text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
              {t('backups.subtitle')}
            </p>
          </div>
        </div>
        <button
          onClick={handleTrigger}
          className="flex items-center gap-2 px-4 py-2 rounded-lg bg-amber-400 text-black font-medium text-sm hover:bg-amber-300 transition-colors active:scale-[0.98]"
        >
          <Play size={16} />
          {t('backups.triggerNow')}
        </button>
      </div>

      {/* Settings Card */}
      <div className={`${cardClass} p-6`}>
        <div className="flex items-center gap-2 mb-4">
          <Settings2 size={18} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
          <h2 className={`text-base font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
            {t('backups.settings')}
          </h2>
        </div>

        {settings && (
          <div className="space-y-4">
            {/* Enabled toggle */}
            <div className="flex items-center justify-between">
              <label className={`text-sm font-medium ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                {t('backups.autoBackup')}
              </label>
              <button
                onClick={() => setSettings({ ...settings, enabled: !settings.enabled })}
                className={`relative w-11 h-6 rounded-full transition-colors ${
                  settings.enabled ? 'bg-amber-400' : isDark ? 'bg-zinc-700' : 'bg-zinc-300'
                }`}
              >
                <span
                  className={`absolute top-0.5 left-0.5 w-5 h-5 rounded-full bg-white transition-transform ${
                    settings.enabled ? 'translate-x-5' : ''
                  }`}
                />
              </button>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {/* Schedule */}
              <div>
                <label className={`block text-sm font-medium mb-1.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                  {t('backups.frequency')}
                </label>
                <select
                  value={settings.schedule}
                  onChange={(e) => setSettings({ ...settings, schedule: e.target.value as BackupSettings['schedule'] })}
                  className={inputClass}
                >
                  {SCHEDULE_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>{t(opt.labelKey)}</option>
                  ))}
                </select>
              </div>

              {/* Retention */}
              <div>
                <label className={`block text-sm font-medium mb-1.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                  {t('backups.retention')}
                </label>
                <select
                  value={settings.retentionDays}
                  onChange={(e) => setSettings({ ...settings, retentionDays: Number(e.target.value) })}
                  className={inputClass}
                >
                  {RETENTION_OPTIONS.map((opt) => (
                    <option key={opt.value} value={opt.value}>{t(opt.labelKey)}</option>
                  ))}
                </select>
              </div>
            </div>

            {/* Databases */}
            <div>
              <label className={`block text-sm font-medium mb-2 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                {t('backups.databases')}
              </label>
              <div className="flex flex-wrap gap-3">
                {DB_OPTIONS.map((db) => {
                  const checked = settings.databases.includes(db.value);
                  return (
                    <label
                      key={db.value}
                      className={`flex items-center gap-2 px-3 py-2 rounded-lg border cursor-pointer transition-all ${
                        checked
                          ? 'border-amber-400/50 bg-amber-400/10'
                          : isDark ? 'border-zinc-700 hover:border-zinc-600' : 'border-zinc-200 hover:border-zinc-300'
                      }`}
                    >
                      <input
                        type="checkbox"
                        checked={checked}
                        onChange={() => {
                          const dbs = checked
                            ? settings.databases.filter((d) => d !== db.value)
                            : [...settings.databases, db.value];
                          setSettings({ ...settings, databases: dbs });
                        }}
                        className="accent-amber-400"
                      />
                      <Database size={14} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
                      <span className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>{db.label}</span>
                    </label>
                  );
                })}
              </div>
            </div>

            <div className="flex justify-end">
              <button
                onClick={handleSaveSettings}
                disabled={saving}
                className="flex items-center gap-2 px-4 py-2 rounded-lg bg-amber-400 text-black font-medium text-sm hover:bg-amber-300 transition-colors disabled:opacity-50 active:scale-[0.98]"
              >
                {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
                {t('common.save')}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Backup History */}
      <div className={cardClass}>
        <div className="flex items-center justify-between p-6 pb-4">
          <h2 className={`text-base font-semibold ${isDark ? 'text-white' : 'text-zinc-900'}`}>
            {t('backups.history')}
          </h2>
          <button onClick={loadData} className={`p-2 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800' : 'hover:bg-zinc-100'}`}>
            <RefreshCw size={16} className={isDark ? 'text-zinc-400' : 'text-zinc-500'} />
          </button>
        </div>

        {backups.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 px-6">
            <HardDrive size={48} className={isDark ? 'text-zinc-700' : 'text-zinc-300'} />
            <p className={`mt-3 text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
              {t('backups.noBackups')}
            </p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full">
              <thead>
                <tr className={`text-xs uppercase tracking-wider ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
                  <th className="px-6 py-3 text-left">{t('backups.table.status')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.databases')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.size')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.duration')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.type')}</th>
                  <th className="px-6 py-3 text-left">{t('backups.table.date')}</th>
                  <th className="px-6 py-3 text-right">{t('backups.table.actions')}</th>
                </tr>
              </thead>
              <tbody>
                {backups.map((backup) => (
                  <tr
                    key={backup.id}
                    className={`border-t transition-colors ${isDark ? 'border-zinc-800/50 hover:bg-zinc-800/30' : 'border-zinc-100 hover:bg-zinc-50'}`}
                  >
                    <td className="px-6 py-3">
                      <StatusBadge status={backup.status} />
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex flex-wrap gap-1">
                        {backup.databases.map((db) => (
                          <span
                            key={db}
                            className={`px-2 py-0.5 rounded text-xs ${isDark ? 'bg-zinc-800 text-zinc-400' : 'bg-zinc-100 text-zinc-600'}`}
                          >
                            {db}
                          </span>
                        ))}
                      </div>
                    </td>
                    <td className={`px-6 py-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {formatBytes(backup.fileSize)}
                    </td>
                    <td className={`px-6 py-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {formatDuration(backup.durationMs)}
                    </td>
                    <td className="px-6 py-3">
                      <span className={`inline-flex items-center gap-1 text-xs ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                        {backup.trigger === 'scheduled' ? <Clock size={12} /> : <Play size={12} />}
                        {backup.trigger}
                      </span>
                    </td>
                    <td className={`px-6 py-3 text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                      {new Date(backup.startedAt).toLocaleString()}
                    </td>
                    <td className="px-6 py-3">
                      <div className="flex items-center justify-end gap-1">
                        {backup.status === 'completed' && (
                          <>
                            <a
                              href={backupsApi.downloadUrl(backup.id)}
                              className={`p-1.5 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800 text-zinc-400 hover:text-white' : 'hover:bg-zinc-100 text-zinc-500 hover:text-zinc-900'}`}
                              title={t('backups.download')}
                            >
                              <Download size={16} />
                            </a>
                            <button
                              onClick={() => setRestoreId(backup.id)}
                              className={`p-1.5 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800 text-zinc-400 hover:text-amber-400' : 'hover:bg-zinc-100 text-zinc-500 hover:text-amber-600'}`}
                              title={t('backups.restore')}
                            >
                              <RotateCcw size={16} />
                            </button>
                          </>
                        )}
                        <button
                          onClick={() => handleDelete(backup.id)}
                          className={`p-1.5 rounded-lg transition-colors ${isDark ? 'hover:bg-zinc-800 text-zinc-400 hover:text-red-400' : 'hover:bg-zinc-100 text-zinc-500 hover:text-red-600'}`}
                          title={t('common.delete')}
                        >
                          <Trash2 size={16} />
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Pagination */}
        {totalPages > 1 && (
          <div className={`flex items-center justify-between px-6 py-3 border-t ${isDark ? 'border-zinc-800/50' : 'border-zinc-100'}`}>
            <span className={`text-xs ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
              {t('backups.showing', { from: page * limit + 1, to: Math.min((page + 1) * limit, total), total })}
            </span>
            <div className="flex gap-1">
              <button
                onClick={() => setPage(Math.max(0, page - 1))}
                disabled={page === 0}
                className={`px-3 py-1 rounded text-xs transition-colors disabled:opacity-30 ${isDark ? 'hover:bg-zinc-800 text-zinc-400' : 'hover:bg-zinc-100 text-zinc-600'}`}
              >
                {t('backups.prev')}
              </button>
              <button
                onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
                disabled={page >= totalPages - 1}
                className={`px-3 py-1 rounded text-xs transition-colors disabled:opacity-30 ${isDark ? 'hover:bg-zinc-800 text-zinc-400' : 'hover:bg-zinc-100 text-zinc-600'}`}
              >
                {t('backups.next')}
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Restore Dialog */}
      {restoreId && restoreBackup && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className={`${cardClass} p-6 max-w-md w-full mx-4 space-y-4`}>
            <div className="flex items-center gap-2 text-amber-400">
              <AlertTriangle size={20} />
              <h3 className="text-lg font-semibold">{t('backups.restoreTitle')}</h3>
            </div>
            <p className={`text-sm ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
              {t('backups.restoreWarning')}
            </p>
            <div className="flex flex-wrap gap-1">
              {restoreBackup.databases.map((db) => (
                <span key={db} className="px-2 py-1 rounded text-xs bg-red-400/10 text-red-400">{db}</span>
              ))}
            </div>
            <div>
              <label className={`block text-sm mb-1.5 ${isDark ? 'text-zinc-400' : 'text-zinc-600'}`}>
                {t('backups.typeRestore')}
              </label>
              <input
                type="text"
                value={restoreConfirm}
                onChange={(e) => setRestoreConfirm(e.target.value)}
                placeholder="RESTORE"
                className={inputClass}
              />
            </div>
            <div className="flex justify-end gap-2">
              <button
                onClick={() => { setRestoreId(null); setRestoreConfirm(''); }}
                className={`px-4 py-2 rounded-lg text-sm ${isDark ? 'hover:bg-zinc-800 text-zinc-400' : 'hover:bg-zinc-100 text-zinc-600'}`}
              >
                {t('common.cancel')}
              </button>
              <button
                onClick={handleRestore}
                disabled={restoreConfirm !== 'RESTORE'}
                className="px-4 py-2 rounded-lg bg-red-500 text-white text-sm font-medium hover:bg-red-600 transition-colors disabled:opacity-30 active:scale-[0.98]"
              >
                {t('backups.confirmRestore')}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

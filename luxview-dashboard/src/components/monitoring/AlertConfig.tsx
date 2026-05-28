import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Bell, Plus, Trash2, ToggleLeft, ToggleRight } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import type { Alert, AlertChannel, CreateAlertPayload } from '../../api/alerts';

interface AlertConfigProps {
  alerts: Alert[];
  onCreateAlert: (payload: CreateAlertPayload) => void;
  onDeleteAlert: (alertId: string) => void;
  onToggleAlert: (alertId: string, enabled: boolean) => void;
}

const metricOptions = ['cpu_percent', 'memory_bytes', 'network_rx', 'network_tx'];
const metricLabelKeys: Record<string, string> = {
  cpu_percent: 'monitoring.alerts.metrics.cpuPercent',
  memory_bytes: 'monitoring.alerts.metrics.memoryBytes',
  network_rx: 'monitoring.alerts.metrics.networkRx',
  network_tx: 'monitoring.alerts.metrics.networkTx',
};
const conditionOptions = [
  { value: 'gt', label: '>' },
  { value: 'gte', label: '>=' },
  { value: 'lt', label: '<' },
  { value: 'lte', label: '<=' },
  { value: 'eq', label: '=' },
];
const channelOptions: AlertChannel[] = ['email', 'webhook', 'discord'];
const channelLabelKeys: Record<string, string> = {
  email: 'monitoring.alerts.channels.email',
  webhook: 'monitoring.alerts.channels.webhook',
  discord: 'monitoring.alerts.channels.discord',
};

const placeholderKeys: Record<AlertChannel, string> = {
  email: 'monitoring.alerts.placeholders.email',
  webhook: 'monitoring.alerts.placeholders.webhook',
  discord: 'monitoring.alerts.placeholders.discord',
};

export function AlertConfig({ alerts, onCreateAlert, onDeleteAlert, onToggleAlert }: AlertConfigProps) {
  const { t } = useTranslation();
  const [showForm, setShowForm] = useState(false);
  const [metric, setMetric] = useState('cpu_percent');
  const [condition, setCondition] = useState('gt');
  const [threshold, setThreshold] = useState(80);
  const [channel, setChannel] = useState<AlertChannel>('email');
  const [channelValue, setChannelValue] = useState('');
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const inputClass = `
    px-3 py-2 rounded-xl text-sm border transition-all duration-200
    focus:outline-none focus:ring-2 focus:ring-amber-400/30
    ${isDark ? 'bg-zinc-900/50 border-zinc-800 text-zinc-100' : 'bg-white border-zinc-200 text-zinc-900'}
  `;

  const needsTarget = channel === 'webhook' || channel === 'discord';
  const canCreate = !needsTarget || channelValue.trim() !== '';

  const handleCreate = () => {
    if (!canCreate) return;
    onCreateAlert({
      metric,
      condition,
      threshold,
      channel,
      channelConfig: { target: channelValue },
    });
    setShowForm(false);
    setChannelValue('');
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3
          className={`text-sm font-semibold ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
        >
          {t('monitoring.alerts.title')}
        </h3>
        <PillButton
          variant="ghost"
          size="sm"
          onClick={() => setShowForm(!showForm)}
          icon={<Plus size={14} />}
        >
          {t('monitoring.alerts.newAlert')}
        </PillButton>
      </div>

      {/* Create Form */}
      {showForm && (
        <GlassCard className="animate-slide-up">
          <div className="grid grid-cols-2 gap-3">
            <select value={metric} onChange={(e) => setMetric(e.target.value)} className={inputClass}>
              {metricOptions.map((m) => (
                <option key={m} value={m}>
                  {t(metricLabelKeys[m])}
                </option>
              ))}
            </select>
            <div className="flex items-center gap-2">
              <select value={condition} onChange={(e) => setCondition(e.target.value)} className={`${inputClass} w-20`}>
                {conditionOptions.map((c) => (
                  <option key={c.value} value={c.value}>{c.label}</option>
                ))}
              </select>
              <input
                type="number"
                value={threshold}
                onChange={(e) => setThreshold(Number(e.target.value))}
                className={`${inputClass} flex-1`}
              />
            </div>
            <select value={channel} onChange={(e) => setChannel(e.target.value as AlertChannel)} className={inputClass}>
              {channelOptions.map((c) => (
                <option key={c} value={c}>{t(channelLabelKeys[c])}</option>
              ))}
            </select>
            <div>
              <input
                type="text"
                value={channelValue}
                onChange={(e) => setChannelValue(e.target.value)}
                placeholder={t(placeholderKeys[channel])}
                className={`${inputClass} w-full`}
              />
              <p className={`text-[11px] mt-1 ${needsTarget && !channelValue.trim() ? 'text-red-400' : 'text-zinc-500'}`}>
                {channel === 'email'
                  ? t('monitoring.alerts.hints.email')
                  : channel === 'webhook'
                    ? t('monitoring.alerts.hints.webhookRequired')
                    : t('monitoring.alerts.hints.discordRequired')
                }
              </p>
            </div>
          </div>
          <div className="flex justify-end mt-3">
            <PillButton variant="primary" size="sm" onClick={handleCreate} disabled={!canCreate}>
              {t('monitoring.alerts.createAlert')}
            </PillButton>
          </div>
        </GlassCard>
      )}

      {/* Alerts list */}
      {alerts.length === 0 ? (
        <div className="text-center py-8 text-zinc-500 text-sm">{t('monitoring.alerts.noAlerts')}</div>
      ) : (
        <div className="space-y-2">
          {alerts.map((alert) => (
            <GlassCard key={alert.id} padding="sm">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <Bell size={16} className={alert.enabled ? 'text-amber-400' : 'text-zinc-600'} />
                  <div>
                    <p className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                      {t(metricLabelKeys[alert.metric] || alert.metric)} {conditionOptions.find((c) => c.value === alert.condition)?.label ?? alert.condition} {alert.threshold}
                    </p>
                    <p className="text-[11px] text-zinc-500">
                      {t('monitoring.alerts.via', { channel: t(channelLabelKeys[alert.channel] || alert.channel) })}
                      {alert.lastTriggered && ` | ${t('monitoring.alerts.lastTriggered', { date: new Date(alert.lastTriggered).toLocaleDateString() })}`}
                    </p>
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => onToggleAlert(alert.id, !alert.enabled)}
                    className="text-zinc-400 hover:text-amber-400 transition-colors"
                  >
                    {alert.enabled ? <ToggleRight size={20} /> : <ToggleLeft size={20} />}
                  </button>
                  <button
                    onClick={() => onDeleteAlert(alert.id)}
                    className="text-zinc-500 hover:text-red-400 transition-colors"
                  >
                    <Trash2 size={14} />
                  </button>
                </div>
              </div>
            </GlassCard>
          ))}
        </div>
      )}
    </div>
  );
}

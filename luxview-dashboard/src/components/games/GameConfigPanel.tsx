import { useState, useEffect, useCallback } from 'react';
import { Save, Loader2, Users, Wifi, WifiOff, RefreshCw } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { gameServersApi, type GameConfigResponse, type GameServerStatus } from '../../api/gameServers';

interface GameConfigPanelProps {
  appId: string;
}

const POLL_INTERVAL = 30_000;

export function GameConfigPanel({ appId }: GameConfigPanelProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [config, setConfig] = useState<GameConfigResponse | null>(null);
  const [status, setStatus] = useState<GameServerStatus | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [statusLoading, setStatusLoading] = useState(false);

  const loadConfig = useCallback(async () => {
    try {
      const cfg = await gameServersApi.getConfig(appId);
      setConfig(cfg);
      setFields(cfg.configFields ?? {});
    } catch {
      addNotification({ type: 'error', title: 'Falha ao carregar configuração do servidor' });
    } finally {
      setLoading(false);
    }
  }, [appId, addNotification]);

  const loadStatus = useCallback(async () => {
    setStatusLoading(true);
    try {
      const s = await gameServersApi.getStatus(appId);
      setStatus(s);
    } catch {
      setStatus(null);
    } finally {
      setStatusLoading(false);
    }
  }, [appId]);

  useEffect(() => {
    loadConfig();
    loadStatus();
    const interval = setInterval(loadStatus, POLL_INTERVAL);
    return () => clearInterval(interval);
  }, [loadConfig, loadStatus]);

  const handleSave = async () => {
    setSaving(true);
    try {
      await gameServersApi.updateConfig(appId, fields);
      addNotification({ type: 'success', title: 'Configuração salva e servidor reiniciado' });
    } catch {
      addNotification({ type: 'error', title: 'Falha ao salvar configuração' });
    } finally {
      setSaving(false);
    }
  };

  const inputClass = `
    w-full px-3 py-2 rounded-xl text-sm border transition-all duration-200
    focus:outline-none focus:ring-2 focus:ring-amber-400/30
    ${isDark ? 'bg-zinc-900/50 border-zinc-800 text-zinc-100 placeholder-zinc-600' : 'bg-white border-zinc-200 text-zinc-900 placeholder-zinc-400'}
  `;

  if (loading) {
    return (
      <div className="flex items-center justify-center py-24">
        <Loader2 className="animate-spin text-zinc-400" size={24} />
      </div>
    );
  }

  if (!config) return null;

  // Group fields by section
  const templateFields = config.template?.configFields ?? [];
  const sections = Array.from(new Set(templateFields.map((f) => f.section ?? 'Geral')));
  const fieldsBySection = Object.fromEntries(
    sections.map((sec) => [sec, templateFields.filter((f) => (f.section ?? 'Geral') === sec)]),
  );

  return (
    <div className="space-y-4">
      {/* Status + Connection Bar */}
      <GlassCard padding="sm">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div className="flex items-center gap-4">
            {/* Online status */}
            <div className="flex items-center gap-2">
              {status?.running ? (
                <Wifi size={15} className="text-emerald-400" />
              ) : (
                <WifiOff size={15} className="text-zinc-500" />
              )}
              <span className={`text-sm font-medium ${status?.running ? 'text-emerald-400' : 'text-zinc-500'}`}>
                {status?.running ? 'Online' : 'Offline'}
              </span>
            </div>

            {/* Player count */}
            {status?.running && (
              <div className="flex items-center gap-2">
                <Users size={14} className="text-zinc-400" />
                <span className={`text-sm ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                  {status.players}/{status.maxPlayers} jogadores
                </span>
              </div>
            )}

            {/* Connection info */}
            <div className="flex items-center gap-2">
              <span className="text-xs text-zinc-500">Conexão:</span>
              <code className={`text-xs font-mono px-2 py-0.5 rounded-md ${isDark ? 'bg-zinc-800 text-amber-400' : 'bg-zinc-100 text-amber-600'}`}>
                {config.serverIp ?? '...'}:{config.gamePort}
              </code>
            </div>

            {config.queryPort && (
              <div className="flex items-center gap-2">
                <span className="text-xs text-zinc-500">Query:</span>
                <code className={`text-xs font-mono px-2 py-0.5 rounded-md ${isDark ? 'bg-zinc-800 text-zinc-400' : 'bg-zinc-100 text-zinc-600'}`}>
                  :{config.queryPort}
                </code>
              </div>
            )}
          </div>

          <PillButton
            variant="ghost"
            size="sm"
            onClick={loadStatus}
            disabled={statusLoading}
            icon={<RefreshCw size={13} className={statusLoading ? 'animate-spin' : ''} />}
          >
            Atualizar
          </PillButton>
        </div>
      </GlassCard>

      {/* Config fields */}
      {sections.map((section) => (
        <GlassCard key={section}>
          <h3 className={`text-sm font-semibold mb-4 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
            {section}
          </h3>
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {fieldsBySection[section].map((fieldDef) => (
              <div key={fieldDef.key}>
                <label className="block text-xs text-zinc-500 mb-1.5">{fieldDef.label}</label>
                {fieldDef.type === 'select' && fieldDef.options ? (
                  <select
                    value={fields[fieldDef.key] ?? ''}
                    onChange={(e) => setFields({ ...fields, [fieldDef.key]: e.target.value })}
                    className={inputClass}
                  >
                    {fieldDef.options.map((opt) => (
                      <option key={opt.value} value={opt.value}>{opt.label}</option>
                    ))}
                  </select>
                ) : (
                  <input
                    type={fieldDef.type === 'password' ? 'password' : fieldDef.type === 'number' ? 'number' : 'text'}
                    value={fields[fieldDef.key] ?? ''}
                    onChange={(e) => setFields({ ...fields, [fieldDef.key]: e.target.value })}
                    placeholder={fieldDef.placeholder ?? ''}
                    className={inputClass}
                  />
                )}
              </div>
            ))}
          </div>
        </GlassCard>
      ))}

      {/* Save button */}
      <div className="flex justify-end">
        <PillButton
          variant="primary"
          size="sm"
          onClick={handleSave}
          disabled={saving}
          icon={saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
        >
          {saving ? 'Salvando...' : 'Salvar e Reiniciar'}
        </PillButton>
      </div>
    </div>
  );
}

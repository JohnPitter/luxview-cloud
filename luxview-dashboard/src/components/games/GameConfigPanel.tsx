import { useState, useEffect, useCallback, useRef } from 'react';
import { Save, Loader2, Users, Wifi, WifiOff, RefreshCw, RotateCw, X, Clock, Trophy } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { gameServersApi, type GameConfigResponse, type GameServerStatus, type PlayerInfo } from '../../api/gameServers';

interface GameConfigPanelProps {
  appId: string;
}

const POLL_INTERVAL_IDLE = 30_000;
const POLL_INTERVAL_RESTART = 5_000;
const RESTART_TIMEOUT_MS = 5 * 60_000; // give up the "Reiniciando" label after 5 min

export function GameConfigPanel({ appId }: GameConfigPanelProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);

  const [config, setConfig] = useState<GameConfigResponse | null>(null);
  const [status, setStatus] = useState<GameServerStatus | null>(null);
  const [fields, setFields] = useState<Record<string, string>>({});
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [statusLoading, setStatusLoading] = useState(false);
  const [activeSection, setActiveSection] = useState<string | null>(null);
  const [restarting, setRestarting] = useState(false);
  const [playersModal, setPlayersModal] = useState(false);
  const [players, setPlayers] = useState<PlayerInfo[]>([]);
  const [playersLoading, setPlayersLoading] = useState(false);
  const restartingSinceRef = useRef<number>(0);

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
      if (s.running && restartingSinceRef.current > 0) {
        setRestarting(false);
        restartingSinceRef.current = 0;
        addNotification({ type: 'success', title: 'Servidor online novamente' });
      }
    } catch {
      setStatus(null);
    } finally {
      setStatusLoading(false);
    }
  }, [appId, addNotification]);

  useEffect(() => {
    loadConfig();
    loadStatus();
  }, [loadConfig, loadStatus]);

  useEffect(() => {
    const intervalMs = restarting ? POLL_INTERVAL_RESTART : POLL_INTERVAL_IDLE;
    const interval = setInterval(() => {
      loadStatus();
      if (restarting && Date.now() - restartingSinceRef.current > RESTART_TIMEOUT_MS) {
        setRestarting(false);
        restartingSinceRef.current = 0;
      }
    }, intervalMs);
    return () => clearInterval(interval);
  }, [loadStatus, restarting]);

  const openPlayersModal = async () => {
    setPlayersModal(true);
    setPlayersLoading(true);
    try {
      const list = await gameServersApi.getPlayers(appId);
      setPlayers(list);
    } catch {
      setPlayers([]);
    } finally {
      setPlayersLoading(false);
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      await gameServersApi.updateConfig(appId, fields);
      addNotification({ type: 'success', title: 'Configuração salva — servidor reiniciando' });
      setRestarting(true);
      restartingSinceRef.current = Date.now();
      // Force an immediate status check (which will likely show offline) so the UI updates fast
      setTimeout(loadStatus, 500);
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

  const templateFields = config.template?.configFields ?? [];
  const sections = Array.from(new Set(templateFields.map((f) => f.section ?? 'Geral')));
  const fieldsBySection = Object.fromEntries(
    sections.map((sec) => [sec, templateFields.filter((f) => (f.section ?? 'Geral') === sec)]),
  );
  const currentSection = activeSection && sections.includes(activeSection) ? activeSection : sections[0];

  const statusBadge = () => {
    if (restarting) {
      return (
        <div className="flex items-center gap-2">
          <RotateCw size={15} className="text-amber-400 animate-spin" />
          <span className="text-sm font-medium text-amber-400">Reiniciando…</span>
        </div>
      );
    }
    if (status?.running) {
      return (
        <div className="flex items-center gap-2">
          <Wifi size={15} className="text-emerald-400" />
          <span className="text-sm font-medium text-emerald-400">Online</span>
        </div>
      );
    }
    return (
      <div className="flex items-center gap-2">
        <WifiOff size={15} className="text-zinc-500" />
        <span className="text-sm font-medium text-zinc-500">Offline</span>
      </div>
    );
  };

  return (
    <div className="space-y-4">
      {/* Status + Connection Bar */}
      <GlassCard padding="sm">
        <div className="flex items-center justify-between flex-wrap gap-3">
          <div className="flex items-center gap-4">
            {statusBadge()}

            {status?.running && !restarting && (
              <button
                onClick={openPlayersModal}
                className={`flex items-center gap-2 px-2 py-1 rounded-lg transition-all cursor-pointer ${
                  isDark
                    ? 'hover:bg-zinc-800 text-zinc-300 hover:text-amber-400'
                    : 'hover:bg-zinc-100 text-zinc-700 hover:text-amber-600'
                }`}
              >
                <Users size={14} className="text-zinc-400" />
                <span className="text-sm">
                  {status.players}/{status.maxPlayers} jogadores
                </span>
              </button>
            )}

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
        {restarting && (
          <div className={`mt-3 text-xs ${isDark ? 'text-amber-300/70' : 'text-amber-700/80'}`}>
            O servidor está reiniciando com as novas configurações. Pode levar 2–3 minutos até voltar online.
          </div>
        )}
      </GlassCard>

      {/* Section tabs */}
      <div className="flex flex-wrap gap-2">
        {sections.map((section) => {
          const isActive = section === currentSection;
          return (
            <button
              key={section}
              type="button"
              onClick={() => setActiveSection(section)}
              className={`
                px-3.5 py-1.5 rounded-full text-sm font-medium transition-all duration-200 border
                ${isActive
                  ? (isDark ? 'bg-amber-400/15 text-amber-300 border-amber-400/40' : 'bg-amber-100 text-amber-700 border-amber-300')
                  : (isDark ? 'bg-zinc-900/40 text-zinc-400 border-zinc-800 hover:text-zinc-200 hover:border-zinc-700' : 'bg-white text-zinc-600 border-zinc-200 hover:text-zinc-900 hover:border-zinc-300')
                }
              `}
            >
              {section}
            </button>
          );
        })}
      </div>

      {/* Active section fields */}
      <GlassCard>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {fieldsBySection[currentSection]?.map((fieldDef) => (
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

      {/* Save button */}
      <div className="flex justify-end">
        <PillButton
          variant="primary"
          size="sm"
          onClick={handleSave}
          disabled={saving || restarting}
          icon={saving ? <Loader2 size={14} className="animate-spin" /> : <Save size={14} />}
        >
          {saving ? 'Salvando...' : restarting ? 'Reiniciando…' : 'Salvar e Reiniciar'}
        </PillButton>
      </div>

      {/* Players Modal */}
      {playersModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 backdrop-blur-sm">
          <div className={`w-full max-w-md rounded-2xl p-6 shadow-xl ${
            isDark ? 'bg-zinc-900 border border-zinc-800' : 'bg-white border border-zinc-200'
          }`}>
            <div className="flex items-center justify-between mb-4">
              <div className="flex items-center gap-2">
                <Users size={16} className="text-amber-400" />
                <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                  Jogadores Online ({status?.players ?? 0}/{status?.maxPlayers ?? 0})
                </h3>
              </div>
              <button onClick={() => setPlayersModal(false)} className="text-zinc-500 hover:text-zinc-300">
                <X size={16} />
              </button>
            </div>

            {playersLoading ? (
              <div className="flex items-center justify-center py-8">
                <Loader2 size={20} className="animate-spin text-amber-400" />
              </div>
            ) : players.length === 0 ? (
              <div className={`text-center py-8 text-sm ${isDark ? 'text-zinc-500' : 'text-zinc-400'}`}>
                Nenhum jogador conectado
              </div>
            ) : (
              <div className="space-y-1.5 max-h-80 overflow-y-auto">
                {players.map((player, i) => (
                  <div
                    key={i}
                    className={`flex items-center justify-between px-3 py-2.5 rounded-lg ${
                      isDark ? 'bg-zinc-800/50' : 'bg-zinc-50'
                    }`}
                  >
                    <div className="flex items-center gap-3">
                      <div className={`w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold ${
                        isDark ? 'bg-amber-400/15 text-amber-400' : 'bg-amber-100 text-amber-700'
                      }`}>
                        {player.name.charAt(0).toUpperCase()}
                      </div>
                      <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
                        {player.name}
                      </span>
                    </div>
                    <div className="flex items-center gap-3">
                      {player.score > 0 && (
                        <div className="flex items-center gap-1">
                          <Trophy size={12} className="text-amber-400" />
                          <span className={`text-xs ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                            {player.score}
                          </span>
                        </div>
                      )}
                      <div className="flex items-center gap-1">
                        <Clock size={12} className="text-zinc-500" />
                        <span className={`text-xs font-mono ${isDark ? 'text-zinc-400' : 'text-zinc-500'}`}>
                          {formatDuration(player.duration)}
                        </span>
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function formatDuration(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.floor((seconds % 3600) / 60);
  if (h > 0) return `${h}h${m.toString().padStart(2, '0')}m`;
  return `${m}min`;
}

import { useCallback, useEffect, useState } from 'react';
import { Loader2, MapPin, RefreshCw, Users } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { gameServersApi, type PlayerInfo } from '../../api/gameServers';

interface GamePlayersPanelProps {
  appId: string;
}

export function GamePlayersPanel({ appId }: GamePlayersPanelProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);
  const [players, setPlayers] = useState<PlayerInfo[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async () => {
    try {
      setPlayers(await gameServersApi.getPlayers(appId));
    } catch {
      addNotification({ type: 'error', title: 'Falha ao listar jogadores online' });
      setPlayers([]);
    } finally {
      setLoading(false);
    }
  }, [appId, addNotification]);

  useEffect(() => {
    void load();
    const id = window.setInterval(() => { void load(); }, 15_000);
    return () => window.clearInterval(id);
  }, [load]);

  const muted = isDark ? 'text-zinc-500' : 'text-zinc-400';
  const rowBg = isDark ? 'bg-zinc-800/40' : 'bg-zinc-50';

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div>
          <h3 className={`text-sm font-semibold ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
            Jogadores online
          </h3>
          <p className={`mt-1 text-xs ${muted}`}>
            Personagem, vocação/classe e mapa conforme a wiki de cada jogo. Atualiza a cada 15s.
          </p>
        </div>
        <PillButton
          variant="ghost"
          size="sm"
          onClick={() => { setLoading(true); void load(); }}
          icon={<RefreshCw size={13} className={loading ? 'animate-spin' : ''} />}
        >
          Atualizar
        </PillButton>
      </div>

      <GlassCard>
        {loading && players.length === 0 ? (
          <div className="flex justify-center py-12">
            <Loader2 className="animate-spin text-amber-400" size={22} />
          </div>
        ) : players.length === 0 ? (
          <div className={`py-12 text-center text-sm ${muted}`}>
            <Users size={18} className="mx-auto mb-2 opacity-60" />
            Ninguém online neste momento
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className={`text-left text-xs uppercase tracking-wide ${muted}`}>
                  <th className="pb-2 pr-3 font-medium">Personagem</th>
                  <th className="pb-2 pr-3 font-medium">Classe</th>
                  <th className="pb-2 pr-3 font-medium">Lv</th>
                  <th className="pb-2 font-medium">Onde está</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-zinc-800/40">
                {players.map((player, i) => (
                  <tr key={`${player.name}-${i}`} className={rowBg}>
                    <td className="py-2.5 pr-3">
                      <div className={`font-medium ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                        {player.character || player.name}
                      </div>
                      {player.account && player.account !== player.character && (
                        <div className={`text-[11px] ${muted}`}>conta {player.account}</div>
                      )}
                    </td>
                    <td className={`py-2.5 pr-3 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                      {player.class || '—'}
                    </td>
                    <td className={`py-2.5 pr-3 font-mono ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                      {player.level && player.level > 0 ? player.level : '—'}
                    </td>
                    <td className={`py-2.5 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                      <span className="inline-flex items-center gap-1.5">
                        <MapPin size={12} className="shrink-0 text-amber-400" />
                        {player.location || '—'}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </GlassCard>
    </div>
  );
}

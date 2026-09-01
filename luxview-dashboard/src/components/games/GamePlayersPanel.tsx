import { useCallback, useEffect, useMemo, useState } from 'react';
import { ChevronLeft, ChevronRight, Loader2, MapPin, RefreshCw, Server, Users, Wifi, WifiOff } from 'lucide-react';
import { GlassCard } from '../common/GlassCard';
import { PillButton } from '../common/PillButton';
import { useThemeStore } from '../../stores/theme.store';
import { useNotificationsStore } from '../../stores/notifications.store';
import { gameServersApi, type PlayerInfo } from '../../api/gameServers';

interface GamePlayersPanelProps {
  appId: string;
}

const PAGE_SIZES = [10, 20, 25] as const;
const DEFAULT_PAGE_SIZE = 20;

export function clampPlayersPage(page: number, total: number, pageSize: number): number {
  if (total <= 0) return 0;
  const last = Math.max(0, Math.ceil(total / pageSize) - 1);
  return Math.min(Math.max(0, page), last);
}

export function GamePlayersPanel({ appId }: GamePlayersPanelProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const addNotification = useNotificationsStore((s) => s.add);
  const [players, setPlayers] = useState<PlayerInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [live, setLive] = useState(false);
  const [page, setPage] = useState(0);
  const [pageSize, setPageSize] = useState<number>(DEFAULT_PAGE_SIZE);
  const [reconnectKey, setReconnectKey] = useState(0);

  const applyRoster = useCallback((next: PlayerInfo[]) => {
    setPlayers(next);
    setLoading(false);
  }, []);

  useEffect(() => {
    let cancelled = false;
    let controller = new AbortController();

    const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

    const run = async () => {
      let fallbackDone = false;
      while (!cancelled) {
        controller = new AbortController();
        try {
          await consumePlayersStream(appId, controller.signal, (next) => {
            if (!cancelled) {
              setLive(true);
              applyRoster(next);
            }
          });
        } catch {
          /* abort or network — reconnect below */
        }
        if (cancelled) return;
        setLive(false);
        if (!fallbackDone) {
          fallbackDone = true;
          try {
            applyRoster(await gameServersApi.getPlayers(appId));
          } catch {
            addNotification({ type: 'error', title: 'Falha ao listar jogadores online' });
            applyRoster([]);
          }
        }
        await sleep(2000);
      }
    };
    void run();
    return () => {
      cancelled = true;
      controller.abort();
    };
  }, [appId, addNotification, applyRoster, reconnectKey]);

  useEffect(() => {
    setPage((current) => clampPlayersPage(current, players.length, pageSize));
  }, [pageSize, players.length]);

  const totalPages = Math.max(1, Math.ceil(players.length / pageSize));
  const safePage = clampPlayersPage(page, players.length, pageSize);
  const pageRows = useMemo(
    () => players.slice(safePage * pageSize, safePage * pageSize + pageSize),
    [players, safePage, pageSize],
  );
  const pageNumbers = visiblePageNumbers(safePage, totalPages);

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
            Personagem, servidor, vocação/classe e cidade. Atualiza ao vivo — entra, sai e muda de mapa na hora.
          </p>
        </div>
        <div className="flex items-center gap-2">
          <span className={`inline-flex items-center gap-1.5 text-[11px] ${live ? 'text-emerald-400' : muted}`}>
            {live ? <Wifi size={12} /> : <WifiOff size={12} />}
            {live ? 'Ao vivo' : 'Reconectando'}
          </span>
          <PillButton
            variant="ghost"
            size="sm"
            onClick={() => { setLoading(true); setReconnectKey((n) => n + 1); }}
            icon={<RefreshCw size={13} className={loading ? 'animate-spin' : ''} />}
          >
            Atualizar
          </PillButton>
        </div>
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
          <>
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className={`text-left text-xs uppercase tracking-wide ${muted}`}>
                    <th className="pb-2 pr-3 font-medium">Personagem</th>
                    <th className="pb-2 pr-3 font-medium">Servidor</th>
                    <th className="pb-2 pr-3 font-medium">Classe</th>
                    <th className="pb-2 pr-3 font-medium">Level</th>
                    <th className="pb-2 font-medium">Cidade</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-zinc-800/40">
                  {pageRows.map((player, i) => (
                    <tr key={`${player.name}-${safePage}-${i}`} className={rowBg}>
                      <td className="py-2.5 pr-3">
                        <div className={`font-medium ${isDark ? 'text-zinc-100' : 'text-zinc-900'}`}>
                          {player.character || player.name}
                        </div>
                        {player.account && player.account !== player.character && (
                          <div className={`text-[11px] ${muted}`}>conta {player.account}</div>
                        )}
                      </td>
                      <td className={`py-2.5 pr-3 ${isDark ? 'text-zinc-300' : 'text-zinc-700'}`}>
                        <span className="inline-flex items-center gap-1.5">
                          <Server size={12} className="shrink-0 text-amber-400" />
                          {player.server || '—'}
                        </span>
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
            <div className="mt-3 flex flex-wrap items-center justify-between gap-2 pt-2">
              <div className={`flex items-center gap-2 text-[11px] ${muted}`}>
                <span>
                  {players.length} jogador{players.length === 1 ? '' : 'es'}
                </span>
                <select
                  value={pageSize}
                  onChange={(e) => setPageSize(Number(e.target.value))}
                  className={`rounded-md border px-1.5 py-0.5 text-[11px] ${
                    isDark ? 'border-zinc-700 bg-zinc-900 text-zinc-200' : 'border-zinc-200 bg-white text-zinc-700'
                  }`}
                >
                  {PAGE_SIZES.map((size) => (
                    <option key={size} value={size}>{size} por página</option>
                  ))}
                </select>
              </div>
              <div className="flex items-center gap-1">
                <PillButton
                  variant="ghost"
                  size="sm"
                  icon={<ChevronLeft size={13} />}
                  onClick={() => setPage((p) => clampPlayersPage(p - 1, players.length, pageSize))}
                  disabled={safePage === 0}
                >
                  Anterior
                </PillButton>
                {pageNumbers.map((n) => (
                  <button
                    key={n}
                    type="button"
                    onClick={() => setPage(n)}
                    className={`min-w-[1.75rem] rounded-md px-1.5 py-1 text-[11px] ${
                      n === safePage
                        ? 'bg-amber-400/20 font-semibold text-amber-300'
                        : muted
                    }`}
                  >
                    {n + 1}
                  </button>
                ))}
                <PillButton
                  variant="ghost"
                  size="sm"
                  onClick={() => setPage((p) => clampPlayersPage(p + 1, players.length, pageSize))}
                  disabled={safePage >= totalPages - 1}
                >
                  Próxima <ChevronRight size={13} />
                </PillButton>
              </div>
            </div>
          </>
        )}
      </GlassCard>
    </div>
  );
}

function visiblePageNumbers(current: number, total: number): number[] {
  const windowSize = 5;
  const start = Math.max(0, Math.min(current - 2, total - windowSize));
  const end = Math.min(total, start + windowSize);
  return Array.from({ length: Math.max(0, end - start) }, (_, i) => start + i);
}

async function consumePlayersStream(
  appId: string,
  signal: AbortSignal,
  onRoster: (players: PlayerInfo[]) => void,
): Promise<void> {
  const token = localStorage.getItem('lv_token');
  const res = await fetch(gameServersApi.playersStreamUrl(appId), {
    headers: token ? { Authorization: `Bearer ${token}` } : {},
    credentials: 'include',
    signal,
  });
  if (!res.ok || !res.body) {
    throw new Error('stream');
  }
  const reader = res.body.getReader();
  const decoder = new TextDecoder();
  let buf = '';
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += decoder.decode(value, { stream: true });
    const chunks = buf.split('\n\n');
    buf = chunks.pop() ?? '';
    for (const chunk of chunks) {
      const roster = parseSseRoster(chunk);
      if (roster) onRoster(roster);
    }
  }
}

function parseSseRoster(chunk: string): PlayerInfo[] | null {
  for (const line of chunk.split('\n')) {
    if (!line.startsWith('data:')) continue;
    const raw = line.slice(5).trim();
    if (!raw) continue;
    try {
      const parsed = JSON.parse(raw) as unknown;
      if (Array.isArray(parsed)) return parsed as PlayerInfo[];
    } catch {
      return null;
    }
  }
  return null;
}

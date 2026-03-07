import { useState, useEffect, useRef, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Loader2, RotateCcw, Wifi, WifiOff, ArrowDown, Search, ChevronLeft, ChevronRight } from 'lucide-react';
import { appsApi } from '../../api/apps';
import { useThemeStore } from '../../stores/theme.store';

interface RuntimeLogsProps {
  appId: string;
  containerId?: string;
}

const LINES_PER_PAGE = 100;

export function RuntimeLogs({ appId, containerId }: RuntimeLogsProps) {
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const [lines, setLines] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const [page, setPage] = useState(0);
  const [autoScroll, setAutoScroll] = useState(true);
  const containerRef = useRef<HTMLDivElement>(null);
  const eventSourceRef = useRef<EventSource | null>(null);

  const connectSSE = useCallback(() => {
    if (!appId || !containerId) return;

    // Close previous connection
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
    }

    setLoading(true);
    const url = appsApi.logsStreamUrl(appId, 200);

    // SSE needs the auth token in the URL since EventSource doesn't support headers
    const token = localStorage.getItem('lv_token');
    const separator = url.includes('?') ? '&' : '?';
    const fullUrl = token ? `${url}${separator}token=${token}` : url;

    const es = new EventSource(fullUrl);
    eventSourceRef.current = es;

    es.onopen = () => {
      setConnected(true);
      setLoading(false);
    };

    es.onmessage = (event) => {
      setLoading(false);
      setConnected(true);
      const line = event.data;
      if (line && !line.startsWith('{"error"')) {
        setLines((prev) => [...prev, line]);
        // Reset to first page (newest) when new lines arrive and autoScroll is on
        if (autoScroll) {
          setPage(0);
        }
      }
    };

    es.onerror = () => {
      setConnected(false);
      setLoading(false);
      es.close();
      // Reconnect after 3 seconds
      setTimeout(() => {
        if (eventSourceRef.current === es) {
          connectSSE();
        }
      }, 3000);
    };
  }, [appId, containerId, autoScroll]);

  useEffect(() => {
    connectSSE();
    return () => {
      eventSourceRef.current?.close();
      eventSourceRef.current = null;
    };
  }, [connectSSE]);

  // Scroll to top when new lines arrive (newest first = top)
  useEffect(() => {
    if (autoScroll && page === 0 && containerRef.current) {
      containerRef.current.scrollTop = 0;
    }
  }, [lines.length, autoScroll, page]);

  const handleRefresh = useCallback(() => {
    setLines([]);
    setPage(0);
    connectSSE();
  }, [connectSSE]);

  // Filter and reverse (newest first)
  const allLines = [...lines].reverse();
  const filtered = search
    ? allLines.filter((l) => l.toLowerCase().includes(search.toLowerCase()))
    : allLines;

  const totalPages = Math.max(1, Math.ceil(filtered.length / LINES_PER_PAGE));
  const pageLines = filtered.slice(page * LINES_PER_PAGE, (page + 1) * LINES_PER_PAGE);

  if (!containerId) {
    return (
      <div
        className={`rounded-xl border p-8 text-center ${
          isDark ? 'bg-zinc-950 border-zinc-800' : 'bg-zinc-50 border-zinc-200'
        }`}
      >
        <span className="text-sm text-zinc-500">{t('monitoring.runtimeLogs.noContainer')}</span>
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {/* Controls */}
      <div className="flex items-center gap-2 flex-wrap">
        {/* Connection status */}
        <div className="flex items-center gap-1.5">
          {connected ? (
            <Wifi size={14} className="text-emerald-400" />
          ) : (
            <WifiOff size={14} className="text-zinc-500" />
          )}
          <span className={`text-[11px] ${connected ? 'text-emerald-400' : 'text-zinc-500'}`}>
            {connected ? t('monitoring.runtimeLogs.live') : t('monitoring.runtimeLogs.disconnected')}
          </span>
        </div>

        <span className={`text-[11px] ${isDark ? 'text-zinc-600' : 'text-zinc-400'}`}>
          {t('monitoring.runtimeLogs.lines', { count: filtered.length })}
        </span>

        <div className="flex-1" />

        {/* Search toggle */}
        <button
          onClick={() => setShowSearch(!showSearch)}
          className={`p-1.5 rounded-lg transition-colors ${
            showSearch
              ? 'text-amber-400 bg-amber-400/10'
              : 'text-zinc-500 hover:text-zinc-300'
          }`}
        >
          <Search size={14} />
        </button>

        {/* Auto-scroll toggle */}
        <button
          onClick={() => {
            setAutoScroll(!autoScroll);
            if (!autoScroll) setPage(0);
          }}
          className={`px-2.5 py-1 text-[11px] rounded-lg border transition-all ${
            autoScroll
              ? 'text-emerald-400 border-emerald-500/30 bg-emerald-500/10'
              : isDark
                ? 'text-zinc-500 border-zinc-800 hover:text-zinc-300'
                : 'text-zinc-400 border-zinc-200 hover:text-zinc-700'
          }`}
        >
          <ArrowDown size={10} className="inline mr-1" />
          {t('monitoring.runtimeLogs.auto')}
        </button>

        {/* Refresh */}
        <button
          onClick={handleRefresh}
          className={`px-2.5 py-1 text-[11px] rounded-lg border transition-colors ${
            isDark
              ? 'text-zinc-500 border-zinc-800 hover:text-amber-400'
              : 'text-zinc-400 border-zinc-200 hover:text-amber-500'
          }`}
        >
          <RotateCcw size={10} className="inline mr-1" />
          {t('monitoring.runtimeLogs.refresh')}
        </button>
      </div>

      {/* Search bar */}
      {showSearch && (
        <input
          type="text"
          value={search}
          onChange={(e) => {
            setSearch(e.target.value);
            setPage(0);
          }}
          placeholder={t('monitoring.runtimeLogs.filterPlaceholder')}
          className={`w-full px-3 py-2 text-xs font-mono rounded-lg border transition-all focus:outline-none focus:border-amber-400/50 ${
            isDark
              ? 'bg-zinc-900/50 border-zinc-800 text-zinc-300 placeholder:text-zinc-600'
              : 'bg-white border-zinc-200 text-zinc-700 placeholder:text-zinc-400'
          }`}
          autoFocus
        />
      )}

      {/* Log output */}
      <div
        className={`rounded-xl border overflow-hidden ${
          isDark ? 'bg-zinc-950 border-zinc-800' : 'bg-zinc-50 border-zinc-200'
        }`}
      >
        {/* Terminal header */}
        <div className={`flex items-center gap-2 px-4 py-2 border-b ${
          isDark ? 'bg-zinc-900/80 border-zinc-800' : 'bg-zinc-100 border-zinc-200'
        }`}>
          <div className="flex gap-1.5">
            <span className="w-2.5 h-2.5 rounded-full bg-red-500/80" />
            <span className="w-2.5 h-2.5 rounded-full bg-amber-500/80" />
            <span className="w-2.5 h-2.5 rounded-full bg-emerald-500/80" />
          </div>
          <span className="text-[11px] text-zinc-500 font-mono ml-1">{t('monitoring.runtimeLogs.title')}</span>
          {connected && (
            <span className="ml-auto flex items-center gap-1">
              <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span className="text-[10px] text-emerald-400/70">{t('monitoring.runtimeLogs.streaming')}</span>
            </span>
          )}
        </div>

        {/* Lines */}
        <div
          ref={containerRef}
          className="overflow-y-auto font-mono text-xs leading-6 p-4"
          style={{ maxHeight: '600px' }}
        >
          {loading ? (
            <div className="flex items-center gap-2 text-zinc-500 py-8 justify-center">
              <Loader2 size={14} className="animate-spin" />
              {t('monitoring.runtimeLogs.connectingToLogStream')}
            </div>
          ) : pageLines.length === 0 ? (
            <div className="text-zinc-500 text-center py-8">
              {search ? t('monitoring.runtimeLogs.noMatchingEntries') : t('monitoring.runtimeLogs.waitingForOutput')}
            </div>
          ) : (
            pageLines.map((line, i) => {
              const level = detectLevel(line);
              return (
                <div
                  key={`${page}-${i}`}
                  className={`flex gap-3 px-2 -mx-2 rounded hover:bg-zinc-800/30 ${
                    level === 'error' ? 'bg-red-500/5' : ''
                  }`}
                >
                  <span className="text-zinc-700 select-none flex-shrink-0 w-8 text-right">
                    {filtered.length - (page * LINES_PER_PAGE + i)}
                  </span>
                  <span className={levelColor(level, isDark)}>{line}</span>
                </div>
              );
            })
          )}
        </div>

        {/* Pagination */}
        {totalPages > 1 && (
          <div className={`flex items-center justify-between px-4 py-2 border-t ${
            isDark ? 'border-zinc-800' : 'border-zinc-200'
          }`}>
            <button
              onClick={() => setPage(Math.max(0, page - 1))}
              disabled={page === 0}
              className={`flex items-center gap-1 px-2 py-1 text-[11px] rounded transition-colors ${
                page === 0
                  ? 'text-zinc-700 cursor-not-allowed'
                  : 'text-zinc-400 hover:text-amber-400'
              }`}
            >
              <ChevronLeft size={12} /> {t('monitoring.runtimeLogs.newer')}
            </button>
            <span className="text-[11px] text-zinc-500">
              {t('monitoring.runtimeLogs.pageOf', { current: page + 1, total: totalPages })}
            </span>
            <button
              onClick={() => setPage(Math.min(totalPages - 1, page + 1))}
              disabled={page >= totalPages - 1}
              className={`flex items-center gap-1 px-2 py-1 text-[11px] rounded transition-colors ${
                page >= totalPages - 1
                  ? 'text-zinc-700 cursor-not-allowed'
                  : 'text-zinc-400 hover:text-amber-400'
              }`}
            >
              {t('monitoring.runtimeLogs.older')} <ChevronRight size={12} />
            </button>
          </div>
        )}
      </div>
    </div>
  );
}

function detectLevel(line: string): 'error' | 'warn' | 'info' | 'debug' | 'default' {
  const lower = line.toLowerCase();
  if (lower.includes('error') || lower.includes('err') || lower.includes('fatal') || lower.includes('panic')) return 'error';
  if (lower.includes('warn')) return 'warn';
  if (lower.includes('debug') || lower.includes('trace')) return 'debug';
  if (lower.includes('info')) return 'info';
  return 'default';
}

function levelColor(level: string, isDark: boolean): string {
  switch (level) {
    case 'error': return 'text-red-400';
    case 'warn': return 'text-amber-400';
    case 'debug': return 'text-zinc-500';
    case 'info': return isDark ? 'text-emerald-400/80' : 'text-emerald-600';
    default: return isDark ? 'text-zinc-300' : 'text-zinc-700';
  }
}

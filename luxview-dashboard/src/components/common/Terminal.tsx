import { useEffect, useRef, useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Search, Pause, Play, Trash2 } from 'lucide-react';
import { useThemeStore } from '../../stores/theme.store';

export interface TerminalLine {
  timestamp?: string;
  level?: 'info' | 'error' | 'warn' | 'debug';
  message: string;
}

interface TerminalProps {
  lines: TerminalLine[];
  title?: string;
  maxHeight?: string;
  onClear?: () => void;
}

const levelColors: Record<string, string> = {
  info: 'text-emerald-400',
  error: 'text-red-400',
  warn: 'text-amber-400',
  debug: 'text-zinc-500',
};

export function Terminal({ lines, title, maxHeight = '500px', onClear }: TerminalProps) {
  const { t } = useTranslation();
  const containerRef = useRef<HTMLDivElement>(null);
  const [autoScroll, setAutoScroll] = useState(true);
  const [search, setSearch] = useState('');
  const [showSearch, setShowSearch] = useState(false);
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const displayTitle = title || t('monitoring.terminal.title');

  const filteredLines = search
    ? lines.filter((l) => l.message.toLowerCase().includes(search.toLowerCase()))
    : lines;

  useEffect(() => {
    if (autoScroll && containerRef.current) {
      containerRef.current.scrollTop = containerRef.current.scrollHeight;
    }
  }, [lines, autoScroll]);

  const handleScroll = useCallback(() => {
    if (!containerRef.current) return;
    const { scrollTop, scrollHeight, clientHeight } = containerRef.current;
    const isAtBottom = scrollHeight - scrollTop - clientHeight < 50;
    setAutoScroll(isAtBottom);
  }, []);

  return (
    <div
      className={`rounded-xl overflow-hidden border ${
        isDark ? 'bg-zinc-950 border-zinc-800' : 'bg-zinc-900 border-zinc-700'
      }`}
    >
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-2 bg-zinc-900/80 border-b border-zinc-800">
        <div className="flex items-center gap-2">
          <div className="flex gap-1.5">
            <span className="w-3 h-3 rounded-full bg-red-500/80" />
            <span className="w-3 h-3 rounded-full bg-amber-500/80" />
            <span className="w-3 h-3 rounded-full bg-emerald-500/80" />
          </div>
          <span className="text-xs text-zinc-500 font-mono ml-2">{displayTitle}</span>
        </div>
        <div className="flex items-center gap-1">
          <button
            onClick={() => setShowSearch(!showSearch)}
            className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors"
            title={t('monitoring.terminal.search')}
          >
            <Search size={14} />
          </button>
          <button
            onClick={() => setAutoScroll(!autoScroll)}
            className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors"
            title={autoScroll ? t('monitoring.terminal.pauseAutoScroll') : t('monitoring.terminal.resumeAutoScroll')}
          >
            {autoScroll ? <Pause size={14} /> : <Play size={14} />}
          </button>
          {onClear && (
            <button
              onClick={onClear}
              className="p-1 text-zinc-500 hover:text-zinc-300 transition-colors"
              title={t('monitoring.terminal.clear')}
            >
              <Trash2 size={14} />
            </button>
          )}
        </div>
      </div>

      {/* Search */}
      {showSearch && (
        <div className="px-4 py-2 bg-zinc-900/60 border-b border-zinc-800">
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={t('monitoring.terminal.searchPlaceholder')}
            className="w-full bg-zinc-800/50 rounded-lg px-3 py-1.5 text-xs font-mono text-zinc-300 placeholder-zinc-600 border border-zinc-700 focus:outline-none focus:border-amber-400/50"
            autoFocus
          />
        </div>
      )}

      {/* Logs */}
      <div
        ref={containerRef}
        onScroll={handleScroll}
        className="overflow-y-auto font-mono text-xs leading-6 p-4"
        style={{ maxHeight }}
      >
        {filteredLines.length === 0 ? (
          <div className="text-zinc-600 text-center py-8">
            {search ? t('monitoring.terminal.noMatchingEntries') : t('monitoring.terminal.waitingForOutput')}
          </div>
        ) : (
          filteredLines.map((line, i) => (
            <div key={i} className="flex gap-3 hover:bg-zinc-800/30 px-2 -mx-2 rounded">
              <span className="text-zinc-700 select-none flex-shrink-0 w-8 text-right">
                {i + 1}
              </span>
              {line.timestamp && (
                <span className="text-zinc-600 flex-shrink-0">
                  {new Date(line.timestamp).toLocaleTimeString()}
                </span>
              )}
              <span className={line.level ? levelColors[line.level] : 'text-zinc-300'}>
                {line.message}
              </span>
            </div>
          ))
        )}
      </div>
    </div>
  );
}

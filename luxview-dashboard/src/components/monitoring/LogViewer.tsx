import { useState, useMemo } from 'react';
import { Terminal, type TerminalLine } from '../common/Terminal';
import { useThemeStore } from '../../stores/theme.store';

interface LogViewerProps {
  logs: TerminalLine[];
  onClear?: () => void;
}

type LogLevel = 'all' | 'info' | 'error' | 'warn' | 'debug';

export function LogViewer({ logs, onClear }: LogViewerProps) {
  const [level, setLevel] = useState<LogLevel>('all');
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  const filtered = useMemo(
    () => (level === 'all' ? logs : logs.filter((l) => l.level === level)),
    [logs, level],
  );

  const levels: LogLevel[] = ['all', 'info', 'warn', 'error', 'debug'];

  return (
    <div className="space-y-3">
      {/* Level filter */}
      <div className="flex items-center gap-1">
        {levels.map((l) => (
          <button
            key={l}
            onClick={() => setLevel(l)}
            className={`
              px-3 py-1 text-xs font-medium rounded-lg
              transition-all duration-200 capitalize
              ${
                level === l
                  ? 'bg-amber-400/10 text-amber-400'
                  : isDark
                    ? 'text-zinc-500 hover:text-zinc-300 hover:bg-zinc-800/50'
                    : 'text-zinc-400 hover:text-zinc-700 hover:bg-zinc-100'
              }
            `}
          >
            {l}
          </button>
        ))}
      </div>

      <Terminal lines={filtered} title="Application Logs" maxHeight="600px" onClear={onClear} />
    </div>
  );
}

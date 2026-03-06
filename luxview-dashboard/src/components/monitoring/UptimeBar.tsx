import { useThemeStore } from '../../stores/theme.store';

interface UptimeBarProps {
  days: Array<{ date: string; status: 'up' | 'down' | 'partial' | 'unknown' }>;
  uptimePercent: number;
}

const statusColors = {
  up: 'bg-emerald-500',
  down: 'bg-red-500',
  partial: 'bg-amber-500',
  unknown: 'bg-zinc-700',
};

export function UptimeBar({ days, uptimePercent }: UptimeBarProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  return (
    <div>
      <div className="flex items-center justify-between mb-3">
        <span className={`text-sm font-medium ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}>
          Uptime (30 days)
        </span>
        <span
          className={`text-sm font-mono font-bold ${
            uptimePercent >= 99.9
              ? 'text-emerald-400'
              : uptimePercent >= 99
                ? 'text-amber-400'
                : 'text-red-400'
          }`}
        >
          {uptimePercent.toFixed(2)}%
        </span>
      </div>
      <div className="flex gap-0.5">
        {days.map((day, i) => (
          <div
            key={i}
            className={`flex-1 h-8 rounded-sm ${statusColors[day.status]} transition-all duration-200 hover:opacity-80`}
            title={`${day.date}: ${day.status}`}
          />
        ))}
      </div>
      <div className="flex items-center justify-between mt-2">
        <span className="text-[11px] text-zinc-500">30 days ago</span>
        <span className="text-[11px] text-zinc-500">Today</span>
      </div>
    </div>
  );
}

import type { AppStatus } from '../../api/apps';

interface StatusDotProps {
  status: AppStatus;
  size?: 'sm' | 'md' | 'lg';
  showLabel?: boolean;
}

const statusConfig: Record<AppStatus, { color: string; pulse: boolean; label: string }> = {
  running: { color: 'bg-emerald-400', pulse: true, label: 'Running' },
  building: { color: 'bg-amber-400', pulse: true, label: 'Building' },
  deploying: { color: 'bg-blue-400', pulse: true, label: 'Deploying' },
  stopped: { color: 'bg-zinc-500', pulse: false, label: 'Stopped' },
  error: { color: 'bg-red-500', pulse: false, label: 'Error' },
  sleeping: { color: 'bg-violet-400', pulse: false, label: 'Sleeping' },
};

const sizeMap = {
  sm: 'h-2 w-2',
  md: 'h-2.5 w-2.5',
  lg: 'h-3 w-3',
};

export function StatusDot({ status, size = 'md', showLabel = false }: StatusDotProps) {
  const config = statusConfig[status];

  return (
    <span className="inline-flex items-center gap-2">
      <span className="relative flex">
        {config.pulse && (
          <span
            className={`absolute inline-flex h-full w-full rounded-full ${config.color} opacity-50 animate-ping`}
          />
        )}
        <span className={`relative inline-flex rounded-full ${config.color} ${sizeMap[size]}`} />
      </span>
      {showLabel && (
        <span className="text-xs font-medium text-zinc-400 capitalize">{config.label}</span>
      )}
    </span>
  );
}

import { useTranslation } from 'react-i18next';
import type { AppStatus } from '../../api/apps';

interface AppStatusBadgeProps {
  status: AppStatus;
  size?: 'sm' | 'md';
}

const statusStyles: Record<AppStatus, string> = {
  running: 'bg-emerald-500/10 text-emerald-400 border-emerald-500/20',
  building: 'bg-amber-500/10 text-amber-400 border-amber-500/20',
  deploying: 'bg-blue-500/10 text-blue-400 border-blue-500/20',
  stopped: 'bg-zinc-500/10 text-zinc-400 border-zinc-500/20',
  error: 'bg-red-500/10 text-red-400 border-red-500/20',
  sleeping: 'bg-violet-500/10 text-violet-400 border-violet-500/20',
};

const statusLabelKeys: Record<AppStatus, string> = {
  running: 'common.status.running',
  building: 'common.status.building',
  deploying: 'common.status.deploying',
  stopped: 'common.status.stopped',
  error: 'common.status.error',
  sleeping: 'common.status.sleeping',
};

export function AppStatusBadge({ status, size = 'sm' }: AppStatusBadgeProps) {
  const { t } = useTranslation();

  return (
    <span
      className={`
        inline-flex items-center font-medium border rounded-full capitalize
        ${statusStyles[status]}
        ${size === 'sm' ? 'text-[11px] px-2.5 py-0.5' : 'text-xs px-3 py-1'}
      `}
    >
      {t(statusLabelKeys[status])}
    </span>
  );
}

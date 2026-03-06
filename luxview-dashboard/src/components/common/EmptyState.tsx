import { type ReactNode } from 'react';
import { useThemeStore } from '../../stores/theme.store';

interface EmptyStateProps {
  icon: ReactNode;
  title: string;
  description: string;
  action?: ReactNode;
}

export function EmptyState({ icon, title, description, action }: EmptyStateProps) {
  const isDark = useThemeStore((s) => s.theme) === 'dark';

  return (
    <div className="flex flex-col items-center justify-center py-20 animate-fade-in">
      <div
        className={`flex items-center justify-center w-16 h-16 rounded-2xl mb-6 ${
          isDark ? 'bg-zinc-800/50' : 'bg-zinc-100'
        }`}
      >
        <span className="text-zinc-500">{icon}</span>
      </div>
      <h3
        className={`text-lg font-semibold mb-2 ${isDark ? 'text-zinc-200' : 'text-zinc-800'}`}
      >
        {title}
      </h3>
      <p className="text-sm text-zinc-500 max-w-sm text-center mb-6">{description}</p>
      {action}
    </div>
  );
}

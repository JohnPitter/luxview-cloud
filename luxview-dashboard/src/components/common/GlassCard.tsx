import { type ReactNode } from 'react';
import { useThemeStore } from '../../stores/theme.store';

interface GlassCardProps {
  children: ReactNode;
  className?: string;
  hover?: boolean;
  onClick?: () => void;
  padding?: 'none' | 'sm' | 'md' | 'lg';
}

const paddingMap = {
  none: '',
  sm: 'p-4',
  md: 'p-6',
  lg: 'p-8',
};

export function GlassCard({
  children,
  className = '',
  hover = false,
  onClick,
  padding = 'md',
}: GlassCardProps) {
  const theme = useThemeStore((s) => s.theme);
  const isDark = theme === 'dark';

  return (
    <div
      onClick={onClick}
      className={`
        rounded-2xl backdrop-blur-md shadow-lg
        ${isDark ? 'bg-zinc-900/50 border border-zinc-800/50' : 'bg-white/60 border border-zinc-200/60'}
        ${hover ? 'hover:shadow-xl hover:scale-[1.01] cursor-pointer' : ''}
        ${paddingMap[padding]}
        transition-all duration-200
        ${className}
      `}
    >
      {children}
    </div>
  );
}

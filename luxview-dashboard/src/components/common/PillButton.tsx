import { type ReactNode, type ButtonHTMLAttributes } from 'react';

interface PillButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md' | 'lg';
  children: ReactNode;
  icon?: ReactNode;
}

const variants = {
  primary:
    'bg-amber-400 text-zinc-950 hover:bg-amber-300 active:bg-amber-500 font-semibold shadow-md hover:shadow-lg',
  secondary:
    'bg-zinc-800 text-white hover:bg-zinc-700 active:bg-zinc-600 border border-zinc-700',
  ghost:
    'bg-transparent hover:bg-zinc-800/50 active:bg-zinc-800/70 text-zinc-300 hover:text-white',
  danger:
    'bg-red-500/10 text-red-400 hover:bg-red-500/20 active:bg-red-500/30 border border-red-500/20',
};

const sizes = {
  sm: 'px-4 py-1.5 text-xs gap-1.5',
  md: 'px-6 py-2 text-sm gap-2',
  lg: 'px-8 py-3 text-base gap-2.5',
};

export function PillButton({
  variant = 'primary',
  size = 'md',
  children,
  icon,
  className = '',
  disabled,
  ...props
}: PillButtonProps) {
  return (
    <button
      className={`
        inline-flex items-center justify-center rounded-full
        transition-all duration-200
        focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-amber-400/50 focus-visible:ring-offset-2 focus-visible:ring-offset-zinc-950
        active:scale-[0.97]
        disabled:opacity-50 disabled:pointer-events-none
        ${variants[variant]}
        ${sizes[size]}
        ${className}
      `}
      disabled={disabled}
      {...props}
    >
      {icon && <span className="flex-shrink-0">{icon}</span>}
      {children}
    </button>
  );
}

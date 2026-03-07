import { useNavigate, useLocation } from 'react-router-dom';
import { Signal, Layers, FileText, Sun, Moon } from 'lucide-react';
import { useThemeStore } from '../../stores/theme.store';

interface NavItem {
  icon: typeof Signal;
  path: string;
  label: string;
}

const navItems: NavItem[] = [
  { icon: Signal, path: '/dashboard/admin', label: 'Monitoring' },
  { icon: Layers, path: '/dashboard', label: 'Apps' },
  { icon: FileText, path: '/dashboard/logs', label: 'Logs' },
];

export function Toolbar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { theme, toggleTheme } = useThemeStore();
  const isDark = theme === 'dark';

  return (
    <nav
      className={`
        fixed top-6 left-1/2 -translate-x-1/2 z-50
        flex items-center gap-6 h-12 px-6
        rounded-3xl shadow-2xl backdrop-blur-md
        transition-all duration-200
        ${isDark ? 'bg-zinc-950/80 text-white border border-zinc-800/50' : 'bg-white/80 text-zinc-950 border border-zinc-200/60'}
      `}
    >
      {/* Logo */}
      <button
        onClick={() => navigate('/dashboard')}
        className={`
          flex items-center justify-center w-8 h-8 rounded-xl
          transition-all duration-200
          ring-1 ring-amber-400/40
          hover:ring-amber-400 hover:shadow-[0_0_15px_rgba(251,191,36,0.3)]
          active:scale-95
          ${isDark ? 'bg-zinc-900' : 'bg-zinc-100'}
        `}
        title="LuxView Cloud"
      >
        <img src="/logo.svg" alt="LuxView Cloud" className="w-5 h-5" />
      </button>

      {/* Divider */}
      <div className={`w-px h-5 ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`} />

      {/* Nav Items */}
      {navItems.map((item) => {
        const isActive = location.pathname === item.path;
        const Icon = item.icon;
        return (
          <button
            key={item.label}
            onClick={() => navigate(item.path)}
            className={`
              flex items-center justify-center w-8 h-8 rounded-lg
              transition-all duration-200
              hover:scale-110
              ${
                isActive
                  ? isDark
                    ? 'text-amber-400 bg-amber-400/10'
                    : 'text-amber-600 bg-amber-100'
                  : isDark
                    ? 'text-zinc-400 hover:text-white hover:bg-zinc-800/50'
                    : 'text-zinc-500 hover:text-zinc-900 hover:bg-zinc-100'
              }
            `}
            title={item.label}
          >
            <Icon size={18} strokeWidth={isActive ? 2.5 : 2} />
          </button>
        );
      })}

      {/* Divider */}
      <div className={`w-px h-5 ${isDark ? 'bg-zinc-800' : 'bg-zinc-200'}`} />

      {/* Theme Toggle */}
      <button
        onClick={toggleTheme}
        className={`
          flex items-center justify-center w-8 h-8 rounded-lg
          transition-all duration-200 hover:scale-110
          ${isDark ? 'text-zinc-400 hover:text-amber-400 hover:bg-zinc-800/50' : 'text-zinc-500 hover:text-amber-600 hover:bg-zinc-100'}
        `}
        title={`Switch to ${isDark ? 'light' : 'dark'} mode`}
      >
        {isDark ? <Sun size={18} /> : <Moon size={18} />}
      </button>
    </nav>
  );
}

import { useNavigate, useLocation } from 'react-router-dom';
import { useTranslation } from 'react-i18next';
import {
  LayoutDashboard,
  Plus,
  BarChart3,
  Settings,
  Shield,
} from 'lucide-react';
import { useThemeStore } from '../../stores/theme.store';
import { useAuthStore } from '../../stores/auth.store';

interface SidebarItem {
  icon: typeof LayoutDashboard;
  labelKey: string;
  path: string;
  adminOnly?: boolean;
}

const items: SidebarItem[] = [
  { icon: LayoutDashboard, labelKey: 'layout.sidebar.dashboard', path: '/dashboard' },
  { icon: Plus, labelKey: 'layout.sidebar.newApp', path: '/dashboard/new' },
  { icon: BarChart3, labelKey: 'layout.sidebar.analytics', path: '/dashboard/analytics' },
  { icon: Settings, labelKey: 'layout.sidebar.settings', path: '/dashboard/settings' },
  { icon: Shield, labelKey: 'layout.sidebar.admin', path: '/dashboard/admin', adminOnly: true },
];

export function Sidebar() {
  const navigate = useNavigate();
  const location = useLocation();
  const { t } = useTranslation();
  const isDark = useThemeStore((s) => s.theme) === 'dark';
  const user = useAuthStore((s) => s.user);

  return (
    <aside
      className={`
        fixed left-6 top-24 bottom-6 w-14 z-30
        flex flex-col items-center py-4 gap-2
        rounded-2xl backdrop-blur-md
        transition-all duration-200
        ${isDark ? 'bg-zinc-900/50 border border-zinc-800/50' : 'bg-white/50 border border-zinc-200/50'}
      `}
    >
      {items
        .filter((item) => !item.adminOnly || user?.role === 'admin')
        .map((item) => {
          const isActive = location.pathname === item.path;
          const Icon = item.icon;
          return (
            <button
              key={item.path}
              onClick={() => navigate(item.path)}
              className={`
                flex items-center justify-center w-10 h-10 rounded-xl
                transition-all duration-200
                ${
                  isActive
                    ? 'bg-amber-400/10 text-amber-400'
                    : isDark
                      ? 'text-zinc-500 hover:text-white hover:bg-zinc-800/50'
                      : 'text-zinc-400 hover:text-zinc-900 hover:bg-zinc-100'
                }
              `}
              title={t(item.labelKey)}
            >
              <Icon size={20} strokeWidth={isActive ? 2.5 : 2} />
            </button>
          );
        })}
    </aside>
  );
}

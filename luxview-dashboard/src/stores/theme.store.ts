import { create } from 'zustand';

type Theme = 'dark' | 'light';

interface ThemeState {
  theme: Theme;
  toggleTheme: () => void;
  setTheme: (theme: Theme) => void;
}

function getInitialTheme(): Theme {
  if (typeof window === 'undefined') return 'dark';
  const stored = localStorage.getItem('lv_theme');
  if (stored === 'light' || stored === 'dark') return stored;
  return 'dark';
}

export const useThemeStore = create<ThemeState>((set) => ({
  theme: getInitialTheme(),

  toggleTheme: () =>
    set((state) => {
      const next = state.theme === 'dark' ? 'light' : 'dark';
      localStorage.setItem('lv_theme', next);
      return { theme: next };
    }),

  setTheme: (theme) => {
    localStorage.setItem('lv_theme', theme);
    set({ theme });
  },
}));

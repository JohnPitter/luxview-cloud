import { create } from 'zustand';
import { authApi, type User } from '../api/auth';

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  loading: boolean;
  login: (token: string, user: User) => void;
  logout: () => void;
  fetchMe: () => Promise<void>;
  initialize: () => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  token: localStorage.getItem('lv_token'),
  isAuthenticated: !!localStorage.getItem('lv_token'),
  loading: false,

  login: (token, user) => {
    localStorage.setItem('lv_token', token);
    set({ token, user, isAuthenticated: true });
  },

  logout: () => {
    localStorage.removeItem('lv_token');
    set({ token: null, user: null, isAuthenticated: false });
    authApi.logout().catch(() => {});
  },

  fetchMe: async () => {
    if (get().loading) return;
    set({ loading: true });
    try {
      const user = await authApi.getMe();
      set({ user, isAuthenticated: true, loading: false });
    } catch {
      localStorage.removeItem('lv_token');
      set({ user: null, token: null, isAuthenticated: false, loading: false });
    }
  },

  initialize: () => {
    const token = localStorage.getItem('lv_token');
    if (token) {
      get().fetchMe();
    }
  },
}));

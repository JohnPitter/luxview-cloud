import { create } from 'zustand';
import { appsApi, type App, type CreateAppPayload } from '../api/apps';

interface AppsState {
  apps: App[];
  selectedApp: App | null;
  loading: boolean;
  error: string | null;
  fetchApps: () => Promise<void>;
  fetchApp: (id: string) => Promise<void>;
  createApp: (payload: CreateAppPayload) => Promise<App>;
  deleteApp: (id: string) => Promise<void>;
  deployApp: (id: string) => Promise<void>;
  stopApp: (id: string) => Promise<void>;
  restartApp: (id: string) => Promise<void>;
  setSelectedApp: (app: App | null) => void;
  updateAppInList: (app: App) => void;
}

export const useAppsStore = create<AppsState>((set, get) => ({
  apps: [],
  selectedApp: null,
  loading: false,
  error: null,

  fetchApps: async () => {
    if (get().loading) return;
    set({ loading: true, error: null });
    try {
      const apps = await appsApi.list();
      set({ apps, loading: false });
    } catch (err) {
      set({ error: 'Failed to fetch apps', loading: false });
      throw err;
    }
  },

  fetchApp: async (id) => {
    try {
      const app = await appsApi.get(id);
      set({ selectedApp: app });
      // Also update in list
      get().updateAppInList(app);
    } catch (err) {
      set({ error: 'Failed to fetch app' });
      throw err;
    }
  },

  createApp: async (payload) => {
    const app = await appsApi.create(payload);
    set((state) => ({ apps: [app, ...state.apps] }));
    return app;
  },

  deleteApp: async (id) => {
    await appsApi.delete(id);
    set((state) => ({
      apps: state.apps.filter((a) => a.id !== id),
      selectedApp: state.selectedApp?.id === id ? null : state.selectedApp,
    }));
  },

  deployApp: async (id) => {
    await appsApi.deploy(id);
    get().updateAppInList({ ...get().apps.find((a) => a.id === id)!, status: 'building' });
  },

  stopApp: async (id) => {
    await appsApi.stop(id);
    get().updateAppInList({ ...get().apps.find((a) => a.id === id)!, status: 'stopped' });
  },

  restartApp: async (id) => {
    await appsApi.restart(id);
  },

  setSelectedApp: (app) => set({ selectedApp: app }),

  updateAppInList: (app) =>
    set((state) => ({
      apps: state.apps.map((a) => (a.id === app.id ? app : a)),
      selectedApp: state.selectedApp?.id === app.id ? app : state.selectedApp,
    })),
}));

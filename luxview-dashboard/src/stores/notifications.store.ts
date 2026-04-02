import { create } from 'zustand';
import { persist } from 'zustand/middleware';

export type NotificationType = 'success' | 'error' | 'warning' | 'info';

export interface Notification {
  id: string;
  type: NotificationType;
  title: string;
  message?: string;
  duration?: number;
  createdAt: number;
  read: boolean;
}

interface NotificationsState {
  notifications: Notification[];
  /** Expiration time in hours (0 = never expire). Default: 72h (3 days) */
  expirationHours: number;
  /** Max number of notifications to keep */
  maxNotifications: number;
  add: (notification: Omit<Notification, 'id' | 'createdAt' | 'read'>) => void;
  remove: (id: string) => void;
  markRead: (id: string) => void;
  markAllRead: () => void;
  clear: () => void;
  setExpirationHours: (hours: number) => void;
  setMaxNotifications: (max: number) => void;
  /** Remove expired notifications based on expirationHours */
  pruneExpired: () => void;
}

let notifId = 0;

export const useNotificationsStore = create<NotificationsState>()(
  persist(
    (set, get) => ({
      notifications: [],
      expirationHours: 72,
      maxNotifications: 100,

      add: (notification) => {
        const id = `notif-${Date.now()}-${++notifId}`;
        const { maxNotifications } = get();
        set((state) => ({
          notifications: [
            { ...notification, id, createdAt: Date.now(), read: false },
            ...state.notifications,
          ].slice(0, maxNotifications),
        }));
      },

      remove: (id) =>
        set((state) => ({
          notifications: state.notifications.filter((n) => n.id !== id),
        })),

      markRead: (id) =>
        set((state) => ({
          notifications: state.notifications.map((n) =>
            n.id === id ? { ...n, read: true } : n,
          ),
        })),

      markAllRead: () =>
        set((state) => ({
          notifications: state.notifications.map((n) => ({ ...n, read: true })),
        })),

      clear: () => set({ notifications: [] }),

      setExpirationHours: (hours) => set({ expirationHours: hours }),

      setMaxNotifications: (max) => set({ maxNotifications: max }),

      pruneExpired: () => {
        const { expirationHours } = get();
        if (expirationHours <= 0) return; // 0 = never expire
        const cutoff = Date.now() - expirationHours * 60 * 60 * 1000;
        set((state) => ({
          notifications: state.notifications.filter((n) => n.createdAt > cutoff),
        }));
      },
    }),
    {
      name: 'luxview-notifications',
      version: 2,
      migrate: (persisted, version) => {
        const state = persisted as NotificationsState;
        if (version < 2) {
          return {
            ...state,
            expirationHours: state.expirationHours ?? 72,
            maxNotifications: state.maxNotifications ?? 100,
          };
        }
        return state;
      },
    },
  ),
);

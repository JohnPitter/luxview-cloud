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
  add: (notification: Omit<Notification, 'id' | 'createdAt' | 'read'>) => void;
  remove: (id: string) => void;
  markRead: (id: string) => void;
  markAllRead: () => void;
  clear: () => void;
}

let notifId = 0;

export const useNotificationsStore = create<NotificationsState>()(
  persist(
    (set) => ({
      notifications: [],

      add: (notification) => {
        const id = `notif-${Date.now()}-${++notifId}`;
        set((state) => ({
          notifications: [
            { ...notification, id, createdAt: Date.now(), read: false },
            ...state.notifications,
          ].slice(0, 50), // keep max 50 notifications
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
    }),
    {
      name: 'luxview-notifications',
    },
  ),
);

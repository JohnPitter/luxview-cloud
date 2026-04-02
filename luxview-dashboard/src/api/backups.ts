import { api } from './client';

export interface Backup {
  id: string;
  databases: string[];
  status: 'running' | 'completed' | 'failed' | 'restoring';
  trigger: 'scheduled' | 'manual';
  fileSize: number;
  durationMs: number;
  error?: string;
  startedAt: string;
  completedAt?: string;
  createdBy?: string;
  createdAt: string;
}

export interface BackupSettings {
  enabled: boolean;
  schedule: 'daily' | 'weekly' | 'monthly';
  retentionDays: number;
  databases: string[];
}

export const backupsApi = {
  async list(limit = 20, offset = 0): Promise<{ backups: Backup[]; total: number }> {
    const { data } = await api.get<{ backups: Backup[]; total: number }>('/admin/backups', {
      params: { limit, offset },
    });
    return data;
  },

  async get(id: string): Promise<Backup> {
    const { data } = await api.get<Backup>(`/admin/backups/${id}`);
    return data;
  },

  async trigger(databases?: string[]): Promise<void> {
    await api.post('/admin/backups', { databases });
  },

  async remove(id: string): Promise<void> {
    await api.delete(`/admin/backups/${id}`);
  },

  async restore(id: string, confirm: string): Promise<void> {
    await api.post(`/admin/backups/${id}/restore`, { confirm });
  },

  downloadUrl(id: string): string {
    return `/api/admin/backups/${id}/download`;
  },

  async getSettings(): Promise<BackupSettings> {
    const { data } = await api.get<BackupSettings>('/admin/backups/settings');
    return data;
  },

  async updateSettings(settings: BackupSettings): Promise<void> {
    await api.put('/admin/backups/settings', settings);
  },
};

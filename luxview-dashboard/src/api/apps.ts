import { api } from './client';

export type AppStatus = 'building' | 'running' | 'stopped' | 'error' | 'sleeping' | 'deploying' | 'maintenance';
export type Stack = 'node' | 'python' | 'go' | 'rust' | 'static' | 'docker';

export interface App {
  id: string;
  userId: string;
  name: string;
  subdomain: string;
  repoUrl: string;
  repoBranch: string;
  stack: Stack;
  status: AppStatus;
  containerId?: string;
  internalPort: number;
  assignedPort: number;
  envVars: Record<string, string>;
  resourceLimits: {
    cpu: string;
    memory: string;
    disk: string;
  };
  autoDeploy: boolean;
  customDomain?: string | null;
  createdAt: string;
  updatedAt: string;
  // Computed fields from API
  cpuPercent?: number;
  memoryBytes?: number;
  lastDeployAt?: string;
}

export interface CreateAppPayload {
  name: string;
  subdomain: string;
  repoUrl: string;
  repoBranch: string;
  envVars?: Record<string, string>;
  autoDeploy?: boolean;
}

export const appsApi = {
  async list(limit = 50, offset = 0): Promise<App[]> {
    const { data } = await api.get<{ apps: App[]; total: number }>('/apps', {
      params: { limit, offset },
    });
    return data.apps ?? [];
  },

  async get(id: string): Promise<App> {
    const { data } = await api.get<App>(`/apps/${id}`);
    return data;
  },

  async create(payload: CreateAppPayload): Promise<App> {
    const { data } = await api.post<App>('/apps', payload);
    return data;
  },

  async update(id: string, payload: Partial<App>): Promise<App> {
    const { data } = await api.patch<App>(`/apps/${id}`, payload);
    return data;
  },

  async delete(id: string): Promise<void> {
    await api.delete(`/apps/${id}`);
  },

  async deploy(id: string, source?: 'manual' | 'ai'): Promise<void> {
    await api.post(`/apps/${id}/deploy`, source ? { source } : {});
  },

  async stop(id: string): Promise<void> {
    await api.post(`/apps/${id}/stop`);
  },

  async setMaintenance(id: string, enabled: boolean): Promise<void> {
    await api.put(`/apps/${id}/maintenance`, { enabled });
  },

  async restart(id: string): Promise<void> {
    await api.post(`/apps/${id}/restart`);
  },

  async checkSubdomain(subdomain: string): Promise<{ available: boolean }> {
    const { data } = await api.get<{ available: boolean }>(`/apps/check-subdomain/${subdomain}`);
    return data;
  },

  async updateEnvVars(id: string, envVars: Record<string, string>): Promise<void> {
    await api.patch(`/apps/${id}`, { envVars });
  },

  async containerLogs(id: string, tail = 200): Promise<string> {
    const { data } = await api.get<{ logs: string }>(`/apps/${id}/logs`, {
      params: { tail },
    });
    return data.logs ?? '';
  },

  /** Returns the base URL for SSE log streaming */
  logsStreamUrl(id: string, tail = 100): string {
    const base = api.defaults.baseURL ?? '/api';
    return `${base}/apps/${id}/logs/stream?tail=${tail}`;
  },
};

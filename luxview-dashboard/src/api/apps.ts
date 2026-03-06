import { api } from './client';

export type AppStatus = 'building' | 'running' | 'stopped' | 'error' | 'sleeping' | 'deploying';
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
  containerId: string;
  internalPort: number;
  assignedPort: number;
  envVars: Record<string, string>;
  resourceLimits: {
    cpu: string;
    memory: string;
    disk: string;
  };
  autoDeploy: boolean;
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
}

export const appsApi = {
  async list(limit = 50, offset = 0): Promise<App[]> {
    const { data } = await api.get<App[]>('/apps', { params: { limit, offset } });
    return data;
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

  async deploy(id: string): Promise<void> {
    await api.post(`/apps/${id}/deploy`);
  },

  async stop(id: string): Promise<void> {
    await api.post(`/apps/${id}/stop`);
  },

  async restart(id: string): Promise<void> {
    await api.post(`/apps/${id}/restart`);
  },

  async checkSubdomain(subdomain: string): Promise<{ available: boolean }> {
    const { data } = await api.get<{ available: boolean }>(`/apps/check-subdomain/${subdomain}`);
    return data;
  },

  async updateEnvVars(id: string, envVars: Record<string, string>): Promise<void> {
    await api.put(`/apps/${id}/env`, { envVars });
  },
};

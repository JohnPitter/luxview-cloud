import { api } from './client';

export type DeployStatus = 'pending' | 'building' | 'deploying' | 'live' | 'failed' | 'rolled_back';

export interface Deployment {
  id: string;
  appId: string;
  commitSha: string;
  commitMessage: string;
  status: DeployStatus;
  buildLog: string;
  durationMs: number;
  imageTag: string;
  source: 'ai' | 'auto';
  createdAt: string;
  finishedAt: string;
}

export const deploymentsApi = {
  async list(appId: string, limit = 20, offset = 0): Promise<Deployment[]> {
    const { data } = await api.get<{ deployments: Deployment[]; total: number }>(
      `/apps/${appId}/deployments`,
      { params: { limit, offset } },
    );
    return data.deployments ?? [];
  },

  async get(appId: string, deployId: string): Promise<Deployment> {
    const { data } = await api.get<Deployment>(`/apps/${appId}/deployments/${deployId}`);
    return data;
  },

  async getLogs(deployId: string): Promise<{ id: string; status: string; buildLog: string }> {
    const { data } = await api.get<{ id: string; status: string; buildLog: string }>(
      `/deployments/${deployId}/logs`,
    );
    return data;
  },

  async rollback(_appId: string, deployId: string): Promise<void> {
    await api.post(`/deployments/${deployId}/rollback`);
  },
};

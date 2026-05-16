import { api } from './client';

export type RepositoryVisibility = 'private' | 'public';
export type RemoteSyncStatus = 'pending' | 'success' | 'failed';

export interface LuxViewRepository {
  id: string;
  userId: string;
  name: string;
  slug: string;
  defaultBranch: string;
  visibility: RepositoryVisibility;
  createdAt: string;
  updatedAt: string;
}

export interface RepositoryRemote {
  id: string;
  repositoryId: string;
  provider: string;
  remoteUrl: string;
  mode: 'backup' | 'mirror';
  lastSyncAt?: string;
  lastSyncStatus?: RemoteSyncStatus;
  lastSyncError?: string;
  createdAt: string;
}

export interface CreateRepositoryPayload {
  name: string;
  slug?: string;
  defaultBranch?: string;
  visibility?: RepositoryVisibility;
}

export const repositoriesApi = {
  async list(limit = 30, offset = 0): Promise<LuxViewRepository[]> {
    const { data } = await api.get<{ repositories: LuxViewRepository[]; total: number }>('/repositories', {
      params: { limit, offset },
    });
    return data.repositories ?? [];
  },

  async create(payload: CreateRepositoryPayload): Promise<LuxViewRepository> {
    const { data } = await api.post<LuxViewRepository>('/repositories', payload);
    return data;
  },

  async listBranches(repositoryId: string): Promise<string[]> {
    const { data } = await api.get<string[]>(`/repositories/${repositoryId}/branches`);
    return Array.isArray(data) ? data : [];
  },

  async listRemotes(repositoryId: string): Promise<RepositoryRemote[]> {
    const { data } = await api.get<{ remotes: RepositoryRemote[] }>(`/repositories/${repositoryId}/remotes`);
    return data.remotes ?? [];
  },

  async addRemote(repositoryId: string, provider: string, remoteUrl: string): Promise<RepositoryRemote> {
    const { data } = await api.post<RepositoryRemote>(`/repositories/${repositoryId}/remotes`, {
      provider,
      remote_url: remoteUrl,
    });
    return data;
  },

  async syncRemote(repositoryId: string, remoteId: string): Promise<void> {
    await api.post(`/repositories/${repositoryId}/remotes/${remoteId}/sync`);
  },

  async delete(repositoryId: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}`);
  },
};

import { api } from './client';

export type RepositoryVisibility = 'private' | 'public';

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
};

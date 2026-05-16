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

export type PullRequestStatus = 'open' | 'merged' | 'closed';

export interface PullRequest {
  id: string;
  repositoryId: string;
  authorId: string;
  number: number;
  title: string;
  description: string;
  headBranch: string;
  baseBranch: string;
  headSha: string;
  status: PullRequestStatus;
  mergeCommit?: string;
  createdAt: string;
  updatedAt: string;
  mergedAt?: string;
  closedAt?: string;
  author?: { id: string; username: string; avatarUrl: string };
}

export interface PRCommit {
  sha: string;
  message: string;
  author: string;
  date: string;
}

export interface PRFileDiff {
  path: string;
  additions: number;
  deletions: number;
  patch: string;
}

export interface PRComment {
  id: string;
  pullRequestId: string;
  authorId: string;
  body: string;
  createdAt: string;
  updatedAt: string;
  author?: { id: string; username: string; avatarUrl: string };
}

export const pullRequestsApi = {
  async list(repositoryId: string, status = '', limit = 30, offset = 0): Promise<{ pullRequests: PullRequest[]; total: number }> {
    const { data } = await api.get<{ pull_requests: PullRequest[]; total: number }>(`/repositories/${repositoryId}/pulls`, {
      params: { status, limit, offset },
    });
    return { pullRequests: data.pull_requests ?? [], total: data.total };
  },

  async create(repositoryId: string, payload: { title: string; description?: string; headBranch: string; baseBranch: string }): Promise<PullRequest> {
    const { data } = await api.post<PullRequest>(`/repositories/${repositoryId}/pulls`, payload);
    return data;
  },

  async get(repositoryId: string, number: number): Promise<PullRequest> {
    const { data } = await api.get<PullRequest>(`/repositories/${repositoryId}/pulls/${number}`);
    return data;
  },

  async commits(repositoryId: string, number: number): Promise<PRCommit[]> {
    const { data } = await api.get<{ commits: PRCommit[] }>(`/repositories/${repositoryId}/pulls/${number}/commits`);
    return data.commits ?? [];
  },

  async diff(repositoryId: string, number: number): Promise<PRFileDiff[]> {
    const { data } = await api.get<{ files: PRFileDiff[] }>(`/repositories/${repositoryId}/pulls/${number}/diff`);
    return data.files ?? [];
  },

  async merge(repositoryId: string, number: number): Promise<PullRequest> {
    const { data } = await api.post<PullRequest>(`/repositories/${repositoryId}/pulls/${number}/merge`);
    return data;
  },

  async close(repositoryId: string, number: number): Promise<PullRequest> {
    const { data } = await api.post<PullRequest>(`/repositories/${repositoryId}/pulls/${number}/close`);
    return data;
  },

  async listComments(repositoryId: string, number: number): Promise<PRComment[]> {
    const { data } = await api.get<{ comments: PRComment[] }>(`/repositories/${repositoryId}/pulls/${number}/comments`);
    return data.comments ?? [];
  },

  async addComment(repositoryId: string, number: number, body: string): Promise<PRComment> {
    const { data } = await api.post<PRComment>(`/repositories/${repositoryId}/pulls/${number}/comments`, { body });
    return data;
  },

  async deleteComment(repositoryId: string, number: number, commentId: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/pulls/${number}/comments/${commentId}`);
  },
};

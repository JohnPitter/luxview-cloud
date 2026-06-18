import { api } from './client';

export type RepositoryVisibility = 'private' | 'public';
export type RemoteSyncStatus = 'pending' | 'success' | 'failed';

export interface LuxViewRepository {
  id: string;
  userId: string;
  name: string;
  slug: string;
  description: string;
  defaultBranch: string;
  visibility: RepositoryVisibility;
  ownerUsername: string;
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
  description?: string;
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

  async get(repositoryId: string): Promise<LuxViewRepository> {
    const { data } = await api.get<LuxViewRepository>(`/repositories/${repositoryId}`);
    return data;
  },

  async create(payload: CreateRepositoryPayload): Promise<LuxViewRepository> {
    const { data } = await api.post<LuxViewRepository>('/repositories', payload);
    return data;
  },

  async update(repositoryId: string, payload: { name?: string; description?: string }): Promise<LuxViewRepository> {
    const { data } = await api.patch<LuxViewRepository>(`/repositories/${repositoryId}`, payload);
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

  async updateVisibility(repositoryId: string, visibility: RepositoryVisibility): Promise<LuxViewRepository> {
    const { data } = await api.patch<LuxViewRepository>(`/repositories/${repositoryId}/visibility`, { visibility });
    return data;
  },

  async importFromGitHub(payload: { owner: string; repo: string; name?: string; defaultBranch?: string; visibility?: RepositoryVisibility }): Promise<LuxViewRepository> {
    const { data } = await api.post<LuxViewRepository>('/repositories/import', payload);
    return data;
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

  async merge(repositoryId: string, number: number, strategy: MergeStrategy = 'merge'): Promise<PullRequest> {
    const { data } = await api.post<PullRequest>(`/repositories/${repositoryId}/pulls/${number}/merge`, { strategy });
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

  async listReviews(repositoryId: string, number: number): Promise<PRReview[]> {
    const { data } = await api.get<{ reviews: PRReview[] }>(`/repositories/${repositoryId}/pulls/${number}/reviews`);
    return data.reviews ?? [];
  },

  async addReview(repositoryId: string, number: number, state: ReviewState, body = ''): Promise<PRReview> {
    const { data } = await api.post<PRReview>(`/repositories/${repositoryId}/pulls/${number}/reviews`, { state, body });
    return data;
  },

  async listReviewComments(repositoryId: string, number: number): Promise<ReviewComment[]> {
    const { data } = await api.get<{ comments: ReviewComment[] }>(`/repositories/${repositoryId}/pulls/${number}/review-comments`);
    return data.comments ?? [];
  },

  async addReviewComment(repositoryId: string, number: number, payload: { path: string; line: number; side?: ReviewSide; body: string }): Promise<ReviewComment> {
    const { data } = await api.post<ReviewComment>(`/repositories/${repositoryId}/pulls/${number}/review-comments`, payload);
    return data;
  },

  async resolveReviewComment(repositoryId: string, number: number, commentId: string, resolved: boolean): Promise<void> {
    await api.patch(`/repositories/${repositoryId}/pulls/${number}/review-comments/${commentId}`, { resolved });
  },

  async deleteReviewComment(repositoryId: string, number: number, commentId: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/pulls/${number}/review-comments/${commentId}`);
  },

  async checks(repositoryId: string, number: number): Promise<StatusCheck[]> {
    const { data } = await api.get<{ checks: StatusCheck[] }>(`/repositories/${repositoryId}/pulls/${number}/checks`);
    return data.checks ?? [];
  },
};

export type MergeStrategy = 'merge' | 'squash' | 'rebase';
export type ReviewState = 'approved' | 'changes_requested' | 'commented';
export type ReviewSide = 'old' | 'new';

export interface PRReview {
  id: string;
  pullRequestId: string;
  reviewerId: string;
  state: ReviewState;
  body: string;
  commitSha: string;
  createdAt: string;
  reviewer?: { id: string; username: string; avatarUrl: string };
}

export interface ReviewComment {
  id: string;
  pullRequestId: string;
  authorId: string;
  path: string;
  line: number;
  side: ReviewSide;
  body: string;
  resolved: boolean;
  createdAt: string;
  updatedAt: string;
  author?: { id: string; username: string; avatarUrl: string };
}

export interface StatusCheck {
  name: string;
  status: string;
  runId: string;
  commitSha: string;
  createdAt: string;
  finishedAt?: string;
}

export interface BranchProtectionRule {
  id: string;
  repositoryId: string;
  branch: string;
  requireReviews: boolean;
  requiredApprovals: number;
  dismissStaleReviews: boolean;
  requireStatusChecks: boolean;
  blockForcePush: boolean;
  createdAt: string;
  updatedAt: string;
}

export const branchProtectionApi = {
  async list(repositoryId: string): Promise<BranchProtectionRule[]> {
    const { data } = await api.get<{ rules: BranchProtectionRule[] }>(`/repositories/${repositoryId}/branch-protection`);
    return data.rules ?? [];
  },

  async upsert(repositoryId: string, rule: Omit<BranchProtectionRule, 'id' | 'repositoryId' | 'createdAt' | 'updatedAt'>): Promise<BranchProtectionRule> {
    const { data } = await api.put<BranchProtectionRule>(`/repositories/${repositoryId}/branch-protection`, rule);
    return data;
  },

  async delete(repositoryId: string, branch: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/branch-protection/${encodeURIComponent(branch)}`);
  },
};

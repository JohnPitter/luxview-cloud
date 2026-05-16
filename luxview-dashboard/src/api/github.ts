import { api } from './client';

export interface GithubRepo {
  id: number;
  name: string;
  fullName: string;
  description: string;
  htmlUrl: string;
  language: string;
  defaultBranch: string;
  private: boolean;
  updatedAt: string;
}

export interface GithubBranch {
  name: string;
  commitSha: string;
}

export interface CreateRepoRequest {
  name: string;
  description?: string;
  private?: boolean;
}

export interface CreateRepoResponse {
  htmlUrl: string;
  fullName: string;
  defaultBranch: string;
  cloneUrl: string;
}

export interface CommitWorkflowRequest {
  owner: string;
  repo: string;
  branch?: string;
  workflowName?: string;
  content: string;
}

export interface SyncSecretsRequest {
  owner: string;
  repo: string;
  secrets: Record<string, string>;
}

export const githubApi = {
  async listRepos(page = 1, perPage = 30): Promise<GithubRepo[]> {
    const { data } = await api.get<GithubRepo[]>('/github/repos', {
      params: { page, per_page: perPage },
    });
    return Array.isArray(data) ? data : [];
  },

  async listBranches(repoFullName: string): Promise<GithubBranch[]> {
    const [owner, repo] = repoFullName.split('/');
    const { data } = await api.get<GithubBranch[]>(`/github/repos/${owner}/${repo}/branches`);
    return Array.isArray(data) ? data : [];
  },

  async createRepo(req: CreateRepoRequest): Promise<CreateRepoResponse> {
    const { data } = await api.post<CreateRepoResponse>('/github/repos', req);
    return data;
  },

  async commitWorkflow(req: CommitWorkflowRequest): Promise<void> {
    await api.put('/github/workflow', req);
  },

  async syncSecrets(req: SyncSecretsRequest): Promise<void> {
    await api.post('/github/sync-secrets', req);
  },

  getInstallUrl(): string {
    return '/api/auth/github/app/install';
  },
};

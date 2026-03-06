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

export const githubApi = {
  async listRepos(page = 1, perPage = 30): Promise<GithubRepo[]> {
    const { data } = await api.get<GithubRepo[]>('/github/repos', {
      params: { page, perPage },
    });
    return data;
  },

  async listBranches(repoFullName: string): Promise<GithubBranch[]> {
    const { data } = await api.get<GithubBranch[]>(`/github/repos/${repoFullName}/branches`);
    return data;
  },
};

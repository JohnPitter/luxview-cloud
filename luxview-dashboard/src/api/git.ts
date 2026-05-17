import { api } from './client';

export interface TreeEntry {
  type: 'blob' | 'tree';
  name: string;
  path: string;
  size?: number;
  mode: string;
}

export interface CommitEntry {
  sha: string;
  message: string;
  author: string;
  email: string;
  date: string;
}

export interface FileDiff {
  path: string;
  additions: number;
  deletions: number;
  patch: string;
}

export interface TagEntry {
  name: string;
  sha: string;
  type: 'lightweight' | 'annotated';
  message?: string;
  tagger?: string;
  date?: string;
}

export const gitApi = {
  async tree(repositoryId: string, ref?: string, path?: string): Promise<{ entries: TreeEntry[]; path: string; ref: string }> {
    const { data } = await api.get(`/repositories/${repositoryId}/tree`, {
      params: { ref: ref ?? '', path: path ?? '' },
    });
    return data;
  },

  async blob(repositoryId: string, path: string, ref?: string): Promise<{ content: string; path: string; ref: string }> {
    const { data } = await api.get(`/repositories/${repositoryId}/blob`, {
      params: { path, ref: ref ?? '' },
    });
    return data;
  },

  async commits(repositoryId: string, ref?: string, limit = 30, offset = 0): Promise<{ commits: CommitEntry[] }> {
    const { data } = await api.get(`/repositories/${repositoryId}/commits`, {
      params: { ref: ref ?? '', limit, offset },
    });
    return data;
  },

  async commit(repositoryId: string, sha: string): Promise<{ commit: CommitEntry; files: FileDiff[] }> {
    const { data } = await api.get(`/repositories/${repositoryId}/commits/${sha}`);
    return data;
  },

  async listTags(repositoryId: string): Promise<{ tags: TagEntry[] }> {
    const { data } = await api.get(`/repositories/${repositoryId}/tags`);
    return data;
  },

  async createTag(repositoryId: string, name: string, ref?: string, message?: string): Promise<void> {
    await api.post(`/repositories/${repositoryId}/tags`, { name, ref, message });
  },

  async deleteTag(repositoryId: string, name: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/tags/${encodeURIComponent(name)}`);
  },

  async createBranch(repositoryId: string, name: string, from?: string): Promise<void> {
    await api.post(`/repositories/${repositoryId}/branches`, { name, from });
  },

  async deleteBranch(repositoryId: string, name: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/branches/${encodeURIComponent(name)}`);
  },
};

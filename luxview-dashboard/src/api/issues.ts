import { api } from './client';

export type IssueStatus = 'open' | 'closed';

export interface Label {
  id: string;
  repositoryId: string;
  name: string;
  color: string;
  description: string;
  createdAt: string;
}

export interface Issue {
  id: string;
  repositoryId: string;
  authorId: string;
  number: number;
  title: string;
  body: string;
  status: IssueStatus;
  createdAt: string;
  updatedAt: string;
  closedAt?: string;
  author?: { id: string; username: string; avatarUrl: string };
  labels?: Label[];
}

export interface IssueComment {
  id: string;
  issueId: string;
  authorId: string;
  body: string;
  createdAt: string;
  updatedAt: string;
  author?: { id: string; username: string; avatarUrl: string };
}

export const issuesApi = {
  async list(repositoryId: string, status = '', limit = 30, offset = 0): Promise<{ issues: Issue[]; total: number }> {
    const { data } = await api.get<{ issues: Issue[]; total: number }>(`/repositories/${repositoryId}/issues`, {
      params: { status, limit, offset },
    });
    return { issues: data.issues ?? [], total: data.total ?? 0 };
  },

  async get(repositoryId: string, number: number): Promise<Issue> {
    const { data } = await api.get<Issue>(`/repositories/${repositoryId}/issues/${number}`);
    return data;
  },

  async create(repositoryId: string, payload: { title: string; body?: string; labelIds?: string[] }): Promise<Issue> {
    const { data } = await api.post<Issue>(`/repositories/${repositoryId}/issues`, payload);
    return data;
  },

  async update(repositoryId: string, number: number, payload: { title?: string; body?: string; labelIds?: string[] }): Promise<Issue> {
    const { data } = await api.patch<Issue>(`/repositories/${repositoryId}/issues/${number}`, payload);
    return data;
  },

  async setStatus(repositoryId: string, number: number, status: IssueStatus): Promise<Issue> {
    const { data } = await api.post<Issue>(`/repositories/${repositoryId}/issues/${number}/status`, { status });
    return data;
  },

  async listComments(repositoryId: string, number: number): Promise<IssueComment[]> {
    const { data } = await api.get<{ comments: IssueComment[] }>(`/repositories/${repositoryId}/issues/${number}/comments`);
    return data.comments ?? [];
  },

  async addComment(repositoryId: string, number: number, body: string): Promise<IssueComment> {
    const { data } = await api.post<IssueComment>(`/repositories/${repositoryId}/issues/${number}/comments`, { body });
    return data;
  },

  async deleteComment(repositoryId: string, number: number, commentId: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/issues/${number}/comments/${commentId}`);
  },

  async listLabels(repositoryId: string): Promise<Label[]> {
    const { data } = await api.get<{ labels: Label[] }>(`/repositories/${repositoryId}/labels`);
    return data.labels ?? [];
  },

  async createLabel(repositoryId: string, payload: { name: string; color?: string; description?: string }): Promise<Label> {
    const { data } = await api.post<Label>(`/repositories/${repositoryId}/labels`, payload);
    return data;
  },

  async deleteLabel(repositoryId: string, labelId: string): Promise<void> {
    await api.delete(`/repositories/${repositoryId}/labels/${labelId}`);
  },
};

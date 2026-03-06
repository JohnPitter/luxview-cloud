import { api } from './client';

export interface User {
  id: string;
  githubId: number;
  username: string;
  email: string;
  avatarUrl: string;
  role: 'user' | 'admin';
  createdAt: string;
  lastLoginAt: string;
}

export interface AuthResponse {
  token: string;
  user: User;
}

export const authApi = {
  getGithubAuthUrl(): string {
    return '/api/auth/github';
  },

  async handleCallback(code: string): Promise<AuthResponse> {
    const { data } = await api.get<AuthResponse>(`/auth/github/callback?code=${code}`);
    return data;
  },

  async getMe(): Promise<User> {
    const { data } = await api.get<User>('/auth/me');
    return data;
  },

  async logout(): Promise<void> {
    await api.post('/auth/logout');
  },
};

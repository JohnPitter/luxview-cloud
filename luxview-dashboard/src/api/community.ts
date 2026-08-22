import { api } from './client';

export interface CommunityPost {
  id: string;
  appId: string;
  game: string;
  gameName: string;
  displayName: string;
  title: string;
  body: string;
  createdAt: string;
}

export const communityApi = {
  async listPosts(appId: string): Promise<CommunityPost[]> {
    const { data } = await api.get<CommunityPost[]>(`/apps/${appId}/community/posts`);
    return data ?? [];
  },

  async createPost(appId: string, title: string, body: string): Promise<CommunityPost> {
    const { data } = await api.post<CommunityPost>(`/apps/${appId}/community/posts`, { title, body });
    return data;
  },

  async deletePost(appId: string, postId: string): Promise<void> {
    await api.delete(`/apps/${appId}/community/posts/${postId}`);
  },
};

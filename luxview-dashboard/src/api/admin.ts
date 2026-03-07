import { api } from './client';
import type { AppStatus } from './apps';

export interface AdminStats {
  totalUsers: number;
  totalApps: number;
  runningApps: number;
  totalDeployments: number;
}

export interface AdminUser {
  id: string;
  username: string;
  email: string;
  avatarUrl: string;
  role: 'user' | 'admin';
  createdAt: string;
}

export interface AdminApp {
  id: string;
  userId: string;
  name: string;
  subdomain: string;
  repoUrl: string;
  repoBranch: string;
  stack: string;
  status: AppStatus;
  containerId?: string;
  internalPort: number;
  assignedPort: number;
  resourceLimits: { cpu: string; memory: string; disk: string };
  autoDeploy: boolean;
  createdAt: string;
  updatedAt: string;
}

export interface VPSInfo {
  cpuCores: number;
  goVersion: string;
  os: string;
  arch: string;
  hostname: string;
  totalMemory: number;
  disk: { total: number; used: number; available: number; percent: string } | null;
  platformReservedCpu: number;
  platformReservedMem: number;
  availableCpu: number;
  availableMemory: number;
  allocatedCpu: string;
  allocatedMemory: number;
  totalAppsCounted: number;
  freeCpu: string;
  freeMemory: number;
}

export const adminApi = {
  async stats(): Promise<AdminStats> {
    const { data } = await api.get<AdminStats>('/admin/stats');
    return data;
  },

  async vpsInfo(): Promise<VPSInfo> {
    const { data } = await api.get<VPSInfo>('/admin/vps-info');
    return data;
  },

  async listUsers(limit = 50, offset = 0): Promise<{ users: AdminUser[]; total: number }> {
    const { data } = await api.get<{ users: AdminUser[]; total: number }>('/admin/users', {
      params: { limit, offset },
    });
    return data;
  },

  async listApps(limit = 50, offset = 0): Promise<{ apps: AdminApp[]; total: number }> {
    const { data } = await api.get<{ apps: AdminApp[]; total: number }>('/admin/apps', {
      params: { limit, offset },
    });
    return data;
  },

  async updateUserRole(userId: string, role: 'user' | 'admin'): Promise<void> {
    await api.patch(`/admin/users/${userId}/role`, { role });
  },

  async updateAppLimits(appId: string, resourceLimits: { cpu: string; memory: string; disk: string }): Promise<void> {
    await api.patch(`/admin/apps/${appId}/limits`, { resourceLimits });
  },

  async forceDeleteApp(appId: string): Promise<void> {
    await api.delete(`/admin/apps/${appId}`);
  },
};

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

export interface CleanupSettings {
  enabled: boolean;
  intervalHours: number;
  thresholdPercent: number;
}

export interface CleanupResult {
  imagesRemoved: number;
  containersRemoved: number;
  buildCacheReclaimed: number;
  imagesReclaimed: number;
  totalReclaimed: number;
}

export interface DiskUsage {
  diskTotal: number;
  diskUsed: number;
  diskAvailable: number;
  diskPercent: string;
  docker?: Array<{ type: string; size: string; reclaimable: string }>;
  imageCount?: number;
  activeContainerCount?: number;
}

export const cleanupApi = {
  async getSettings(): Promise<CleanupSettings> {
    const { data } = await api.get<CleanupSettings>('/admin/settings/cleanup');
    return data;
  },

  async updateSettings(settings: Partial<CleanupSettings>): Promise<void> {
    await api.put('/admin/settings/cleanup', settings);
  },

  async trigger(): Promise<CleanupResult> {
    const { data } = await api.post<CleanupResult>('/admin/cleanup/trigger');
    return data;
  },

  async diskUsage(): Promise<DiskUsage> {
    const { data } = await api.get<DiskUsage>('/admin/cleanup/disk-usage');
    return data;
  },
};

export const timezoneApi = {
  async get(): Promise<{ timezone: string }> {
    const { data } = await api.get<{ timezone: string }>('/admin/settings/timezone');
    return data;
  },
  async update(timezone: string): Promise<void> {
    await api.put('/admin/settings/timezone', { timezone });
  },
};

export const authSettingsApi = {
  async get(): Promise<{ requireAuth: boolean }> {
    const { data } = await api.get<{ requireAuth: boolean }>('/auth/settings');
    return data;
  },
  async update(requireAuth: boolean): Promise<void> {
    await api.put('/admin/settings/auth', { requireAuth });
  },
};

export interface AuditLog {
  id: number;
  actorId: string | null;
  actorUsername: string;
  action: string;
  resourceType: string;
  resourceId: string;
  resourceName: string;
  oldValues?: Record<string, unknown>;
  newValues?: Record<string, unknown>;
  ipAddress: string;
  createdAt: string;
}

export interface AuditStats {
  total24h: number;
  byAction: Record<string, number>;
  byResource: Record<string, number>;
}

export interface AuditLogFilters {
  actorId?: string;
  action?: string;
  resourceType?: string;
  search?: string;
  from?: string;
  to?: string;
}

export const auditApi = {
  async list(filters: AuditLogFilters = {}, limit = 50, offset = 0): Promise<{ logs: AuditLog[]; total: number }> {
    const { data } = await api.get<{ logs: AuditLog[]; total: number }>('/admin/audit-logs', {
      params: { ...filters, limit, offset },
    });
    return data;
  },
  async stats(): Promise<AuditStats> {
    const { data } = await api.get<AuditStats>('/admin/audit-logs/stats');
    return data;
  },
};

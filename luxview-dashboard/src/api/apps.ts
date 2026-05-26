import { api } from './client';

export type AppStatus = 'building' | 'running' | 'stopped' | 'error' | 'sleeping' | 'deploying' | 'maintenance';
export type Stack = 'node' | 'python' | 'go' | 'rust' | 'static' | 'docker' | 'game';
export type AppType = 'web' | 'game';

export interface App {
  id: string;
  userId: string;
  name: string;
  subdomain: string;
  repositoryId?: string;
  repoUrl: string;
  repoBranch: string;
  stack: Stack;
  status: AppStatus;
  containerId?: string;
  internalPort: number;
  assignedPort: number;
  envVars: Record<string, string>;
  resourceLimits: {
    cpu: string;
    memory: string;
    disk: string;
  };
  autoDeploy: boolean;
  appType: AppType;
  customDomain?: string | null;
  gameConfig?: GameServerConfig | null;
  createdAt: string;
  updatedAt: string;
  // Computed fields from API
  cpuPercent?: number;
  memoryBytes?: number;
  lastDeployAt?: string;
}

export interface GameServerConfig {
  id: string;
  appId: string;
  templateId: string;
  image: string;
  gamePort: number;
  queryPort?: number;
  dataDir: string;
  dataVolume?: string;
  protocol: string;
  configFields: Record<string, string>;
}

export interface CreateGameServerPayload {
  name: string;
  subdomain: string;
  appType: 'game';
  gameConfig: {
    templateId: string;
    image: string;
    gamePort: number;
    queryPort?: number;
    dataDir?: string;
    dataVolume?: string;
  };
}

export interface CreateAppPayload {
  name: string;
  subdomain: string;
  repositoryId?: string;
  repoUrl?: string;
  repoBranch: string;
  envVars?: Record<string, string>;
  autoDeploy?: boolean;
}

export const appsApi = {
  async list(limit = 50, offset = 0): Promise<App[]> {
    const { data } = await api.get<{ apps: App[]; total: number }>('/apps', {
      params: { limit, offset },
    });
    return data.apps ?? [];
  },

  async get(id: string): Promise<App> {
    const { data } = await api.get<App>(`/apps/${id}`);
    return data;
  },

  async create(payload: CreateAppPayload): Promise<App> {
    const { data } = await api.post<App>('/apps', payload);
    return data;
  },

  async update(id: string, payload: Partial<App>): Promise<App> {
    const { data } = await api.patch<App>(`/apps/${id}`, payload);
    return data;
  },

  async delete(id: string): Promise<void> {
    await api.delete(`/apps/${id}`);
  },

  async deploy(id: string, source?: 'manual' | 'ai'): Promise<void> {
    await api.post(`/apps/${id}/deploy`, source ? { source } : {});
  },

  async stop(id: string): Promise<void> {
    await api.post(`/apps/${id}/stop`);
  },

  async setMaintenance(id: string, enabled: boolean): Promise<void> {
    await api.put(`/apps/${id}/maintenance`, { enabled });
  },

  async restart(id: string): Promise<void> {
    await api.post(`/apps/${id}/restart`);
  },

  async checkSubdomain(subdomain: string): Promise<{ available: boolean }> {
    const { data } = await api.get<{ available: boolean }>(`/apps/check-subdomain/${subdomain}`);
    return data;
  },

  async updateEnvVars(id: string, envVars: Record<string, string>): Promise<void> {
    await api.patch(`/apps/${id}`, { envVars });
  },

  async containerLogs(id: string, tail = 200): Promise<string> {
    const { data } = await api.get<{ logs: string }>(`/apps/${id}/logs`, {
      params: { tail },
    });
    return data.logs ?? '';
  },

  /** Returns the base URL for SSE log streaming */
  logsStreamUrl(id: string, tail = 100): string {
    const base = api.defaults.baseURL ?? '/api';
    return `${base}/apps/${id}/logs/stream?tail=${tail}`;
  },

  async checkDomain(id: string, domain?: string): Promise<DomainCheckResult> {
    const { data } = await api.get<DomainCheckResult>(`/apps/${id}/domain-check`, {
      params: domain ? { domain } : undefined,
    });
    return data;
  },
};

export interface DomainHostStatus {
  host: string;
  ips: string[];
  match: boolean;
  cloudflare_proxied: boolean;
}

export interface DomainCertStatus {
  issued: boolean;
  not_after?: string;
  last_error?: string;
}

export type DomainIssue =
  | 'empty_domain'
  | 'parking_nameservers'
  | 'apex_unresolved'
  | 'apex_wrong_ip'
  | 'cloudflare_proxy_active'
  | 'cert_pending';

export interface DomainCheckResult {
  domain: string;
  expected_ip: string;
  apex: DomainHostStatus;
  www: DomainHostStatus;
  nameservers: string[];
  parking_detected: boolean;
  cert: DomainCertStatus;
  ready: boolean;
  issues: DomainIssue[];
  checked_at: string;
}

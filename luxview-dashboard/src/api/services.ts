import { api, apiRaw } from './client';

export type ServiceType = 'postgres' | 'redis' | 'mongodb' | 'rabbitmq' | 'storage' | 'email';

export interface AppService {
  id: string;
  appId: string;
  serviceType: ServiceType;
  dbName: string;
  credentials: Record<string, string>;
  createdAt: string;
}

export interface AppServiceWithApp extends AppService {
  appName: string;
  appSubdomain: string;
}

// DB Explorer types
export interface TableInfo {
  name: string;
  type: string;
}

export interface ColumnInfo {
  name: string;
  type: string;
  nullable: string;
  default: string | null;
}

export interface TableSchema {
  columns: ColumnInfo[];
  rowCount: number;
}

export interface QueryResult {
  columns: string[];
  rows: Record<string, unknown>[];
  rowCount: number;
  truncated: boolean;
}

export interface StorageUsageInfo {
  used: number;
  limit: number;
  limitStr: string;
}

// Storage Explorer types
export interface StorageFileInfo {
  key: string;
  size: number;
  lastModified: string;
  isDir: boolean;
}

// Mailbox types
export interface Mailbox {
  id: string;
  service_id: string;
  address: string;
  created_at: string;
}

export interface MailboxCreated extends Mailbox {
  password: string;
}

export const servicesApi = {
  async listAll(): Promise<AppServiceWithApp[]> {
    const { data } = await api.get<AppServiceWithApp[]>('/services');
    return data ?? [];
  },

  async list(appId: string): Promise<AppService[]> {
    const { data } = await api.get<AppService[]>(`/apps/${appId}/services`);
    return data ?? [];
  },

  async create(appId: string, serviceType: ServiceType): Promise<AppService> {
    const { data } = await api.post<AppService>(`/apps/${appId}/services`, { serviceType });
    return data;
  },

  async delete(serviceId: string): Promise<void> {
    await api.delete(`/services/${serviceId}`);
  },

  // DB Explorer
  async listTables(serviceId: string): Promise<TableInfo[]> {
    const { data } = await api.get<TableInfo[]>(`/services/${serviceId}/tables`);
    return data ?? [];
  },

  async getTableSchema(serviceId: string, tableName: string): Promise<TableSchema> {
    const { data } = await api.get<TableSchema>(`/services/${serviceId}/tables/${tableName}`);
    return data;
  },

  async executeQuery(serviceId: string, query: string): Promise<QueryResult> {
    // Use apiRaw to bypass snakeToCamel response transform — DB column names must be preserved.
    const { data } = await apiRaw.post<QueryResult>(`/services/${serviceId}/query`, { query });
    return data;
  },

  // Storage Explorer
  async listFiles(serviceId: string, prefix?: string): Promise<StorageFileInfo[]> {
    const params = prefix ? { prefix } : {};
    const { data } = await api.get<StorageFileInfo[]>(`/services/${serviceId}/files`, { params });
    return data ?? [];
  },

  async uploadFile(serviceId: string, file: File, key?: string): Promise<{ key: string }> {
    const formData = new FormData();
    formData.append('file', file);
    if (key) formData.append('key', key);
    const { data } = await api.post<{ key: string }>(`/services/${serviceId}/files/upload`, formData, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000,
    });
    return data;
  },

  async downloadFile(serviceId: string, key: string): Promise<Blob> {
    const { data } = await api.get(`/services/${serviceId}/files/download`, {
      params: { key },
      responseType: 'blob',
    });
    return data;
  },

  async deleteFile(serviceId: string, key: string): Promise<void> {
    await api.delete(`/services/${serviceId}/files`, { params: { key } });
  },

  async getServiceUsage(serviceId: string): Promise<StorageUsageInfo> {
    const { data } = await api.get<StorageUsageInfo>(`/services/${serviceId}/usage`);
    return data;
  },

  // Mailbox API
  async listMailboxes(serviceId: string): Promise<Mailbox[]> {
    const { data } = await api.get<Mailbox[]>(`/services/${serviceId}/mailboxes`);
    return data ?? [];
  },

  async createMailbox(serviceId: string, localPart: string): Promise<MailboxCreated> {
    const { data } = await api.post<MailboxCreated>(`/services/${serviceId}/mailboxes`, { local_part: localPart });
    return data;
  },

  async deleteMailbox(mailboxId: string): Promise<void> {
    await api.delete(`/mailboxes/${mailboxId}`);
  },

  async resetMailboxPassword(mailboxId: string): Promise<{ address: string; password: string }> {
    const { data } = await api.post<{ address: string; password: string }>(`/mailboxes/${mailboxId}/reset-password`);
    return data;
  },
};

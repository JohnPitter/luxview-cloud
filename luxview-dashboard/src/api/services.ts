import { api } from './client';

export type ServiceType = 'postgres' | 'redis' | 'mongodb' | 'rabbitmq' | 'storage';

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
    const { data } = await api.post<QueryResult>(`/services/${serviceId}/query`, { query });
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
};

import { api } from './client';

export type ServiceType = 'postgres' | 'redis' | 'mongodb' | 'rabbitmq';

export interface AppService {
  id: string;
  appId: string;
  serviceType: ServiceType;
  dbName: string;
  credentials: {
    host: string;
    port: string;
    username: string;
    password: string;
    database: string;
    url: string;
  };
  createdAt: string;
}

export const servicesApi = {
  async list(appId: string): Promise<AppService[]> {
    const { data } = await api.get<AppService[]>(`/apps/${appId}/services`);
    return data ?? [];
  },

  async create(appId: string, serviceType: ServiceType): Promise<AppService> {
    const { data } = await api.post<AppService>(`/apps/${appId}/services`, { serviceType });
    return data;
  },

  async delete(appId: string, serviceId: string): Promise<void> {
    await api.delete(`/apps/${appId}/services/${serviceId}`);
  },
};

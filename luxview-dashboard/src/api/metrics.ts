import { api } from './client';

export interface MetricPoint {
  timestamp: string;
  cpuPercent: number;
  memoryBytes: number;
  networkRx: number;
  networkTx: number;
}

export const metricsApi = {
  async get(appId: string, period = '1h'): Promise<MetricPoint[]> {
    const { data } = await api.get<{ metrics: MetricPoint[] }>(`/apps/${appId}/metrics`, {
      params: { period },
    });
    return data.metrics ?? [];
  },
};

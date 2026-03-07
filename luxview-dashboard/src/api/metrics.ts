import { api } from './client';

// Backend returns MetricAggregation with these field names (after camelCase transform)
interface RawAggregation {
  timestamp: string;
  avgCpu: number;
  maxCpu: number;
  avgMemory: number;
  maxMemory: number;
  avgNetworkRx: number;
  avgNetworkTx: number;
}

export interface MetricPoint {
  timestamp: string;
  cpuPercent: number;
  memoryBytes: number;
  networkRx: number;
  networkTx: number;
}

function mapAggregation(raw: RawAggregation): MetricPoint {
  return {
    timestamp: raw.timestamp,
    cpuPercent: raw.avgCpu ?? 0,
    memoryBytes: raw.avgMemory ?? 0,
    networkRx: raw.avgNetworkRx ?? 0,
    networkTx: raw.avgNetworkTx ?? 0,
  };
}

export interface LatestMetric {
  cpuPercent: number;
  memoryBytes: number;
  networkRx: number;
  networkTx: number;
  timestamp: string;
}

export const metricsApi = {
  async get(appId: string, period = '1h'): Promise<MetricPoint[]> {
    const { data } = await api.get<{ metrics: RawAggregation[] }>(`/apps/${appId}/metrics`, {
      params: { period },
    });
    return (data.metrics ?? []).map(mapAggregation);
  },

  async getLatestAll(): Promise<Record<string, LatestMetric>> {
    const { data } = await api.get<{ metrics: Record<string, LatestMetric> }>('/apps/metrics/latest');
    return data.metrics ?? {};
  },
};

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

export type MetricPeriod = '1h' | '24h' | '7d' | '30d' | '365d';

const periodConfig: Record<MetricPeriod, { offsetMs: number; intervalSec: number }> = {
  '1h':   { offsetMs: 60 * 60 * 1000,           intervalSec: 60 },
  '24h':  { offsetMs: 24 * 60 * 60 * 1000,      intervalSec: 900 },
  '7d':   { offsetMs: 7 * 24 * 60 * 60 * 1000,  intervalSec: 3600 },
  '30d':  { offsetMs: 30 * 24 * 60 * 60 * 1000, intervalSec: 14400 },
  '365d': { offsetMs: 365 * 24 * 60 * 60 * 1000, intervalSec: 86400 },
};

export const metricsApi = {
  async get(appId: string, period: MetricPeriod = '1h'): Promise<MetricPoint[]> {
    const { offsetMs, intervalSec } = periodConfig[period];
    const now = new Date();
    const from = new Date(now.getTime() - offsetMs);
    const { data } = await api.get<{ metrics: RawAggregation[] }>(`/apps/${appId}/metrics`, {
      params: {
        from: from.toISOString(),
        to: now.toISOString(),
        interval: intervalSec,
      },
    });
    return (data.metrics ?? []).map(mapAggregation);
  },

  async getLatestAll(): Promise<Record<string, LatestMetric>> {
    const { data } = await api.get<{ metrics: Record<string, LatestMetric> }>('/apps/metrics/latest');
    return data.metrics ?? {};
  },
};

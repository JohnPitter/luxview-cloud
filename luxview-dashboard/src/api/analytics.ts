import { api } from './client';

export interface OverviewData {
  visitors: number;
  pageviews: number;
  bounceRate: number;
  avgDurationMs: number;
  timeSeries: Array<{ bucket: string; views: number; visitors: number }>;
  prevVisitors: number;
  prevPageviews: number;
}

export interface RankedItem {
  name: string;
  views: number;
  visitors: number;
}

export interface LiveData {
  visitors: number;
}

interface AnalyticsParams {
  appId?: string;
  start?: string;
  end?: string;
  granularity?: 'hour' | 'day';
  limit?: number;
}

function buildParams(p: AnalyticsParams): Record<string, string> {
  const params: Record<string, string> = {};
  if (p.appId) params.app_id = p.appId;
  if (p.start) params.start = p.start;
  if (p.end) params.end = p.end;
  if (p.granularity) params.granularity = p.granularity;
  if (p.limit) params.limit = String(p.limit);
  return params;
}

export const analyticsApi = {
  async overview(params: AnalyticsParams): Promise<OverviewData> {
    const { data } = await api.get<OverviewData>('/analytics/overview', {
      params: buildParams(params),
    });
    return data;
  },

  async pages(params: AnalyticsParams): Promise<RankedItem[]> {
    const { data } = await api.get<{ pages: RankedItem[] }>('/analytics/pages', {
      params: buildParams(params),
    });
    return data.pages ?? [];
  },

  async geo(params: AnalyticsParams): Promise<RankedItem[]> {
    const { data } = await api.get<{ countries: RankedItem[] }>('/analytics/geo', {
      params: buildParams(params),
    });
    return data.countries ?? [];
  },

  async browsers(params: AnalyticsParams): Promise<RankedItem[]> {
    const { data } = await api.get<{ browsers: RankedItem[] }>('/analytics/browsers', {
      params: buildParams(params),
    });
    return data.browsers ?? [];
  },

  async os(params: AnalyticsParams): Promise<RankedItem[]> {
    const { data } = await api.get<{ os: RankedItem[] }>('/analytics/os', {
      params: buildParams(params),
    });
    return data.os ?? [];
  },

  async devices(params: AnalyticsParams): Promise<RankedItem[]> {
    const { data } = await api.get<{ devices: RankedItem[] }>('/analytics/devices', {
      params: buildParams(params),
    });
    return data.devices ?? [];
  },

  async referers(params: AnalyticsParams): Promise<RankedItem[]> {
    const { data } = await api.get<{ referers: RankedItem[] }>('/analytics/referers', {
      params: buildParams(params),
    });
    return data.referers ?? [];
  },

  async live(appId?: string): Promise<LiveData> {
    const params: Record<string, string> = {};
    if (appId) params.app_id = appId;
    const { data } = await api.get<LiveData>('/analytics/live', { params });
    return data;
  },
};

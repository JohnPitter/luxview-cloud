import { useState, useEffect, useCallback } from 'react';
import { metricsApi, type MetricPoint } from '../api/metrics';

export function useMetricsLive(appId: string, period = '1h', refreshInterval = 30000) {
  const [metrics, setMetrics] = useState<MetricPoint[]>([]);
  const [loading, setLoading] = useState(false);

  const fetch = useCallback(async () => {
    setLoading(true);
    try {
      const data = await metricsApi.get(appId, period);
      setMetrics(data);
    } catch {
      // silently fail
    } finally {
      setLoading(false);
    }
  }, [appId, period]);

  useEffect(() => {
    fetch();
    const interval = setInterval(fetch, refreshInterval);
    return () => clearInterval(interval);
  }, [fetch, refreshInterval]);

  return { metrics, loading, refresh: fetch };
}

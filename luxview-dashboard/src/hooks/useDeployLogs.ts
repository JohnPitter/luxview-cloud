import { useState, useEffect, useCallback } from 'react';
import { useWebSocket } from './useWebSocket';

export interface LogLine {
  timestamp: string;
  level: 'info' | 'error' | 'warn' | 'debug';
  message: string;
}

export function useDeployLogs(appId: string, deployId?: string) {
  const [logs, setLogs] = useState<LogLine[]>([]);
  const [connected, setConnected] = useState(false);
  const wsUrl = `${window.location.protocol === 'https:' ? 'wss:' : 'ws:'}//${window.location.host}/ws/logs/${appId}${deployId ? `/${deployId}` : ''}`;
  const wsRef = useWebSocket(wsUrl);

  useEffect(() => {
    const client = wsRef.current;
    if (!client) return;

    const unsubLog = client.on('log', (data) => {
      setLogs((prev) => [...prev, data as LogLine]);
    });

    const unsubConnected = client.on('connected', () => {
      setConnected(true);
    });

    return () => {
      unsubLog();
      unsubConnected();
    };
  }, [wsRef]);

  const clear = useCallback(() => setLogs([]), []);

  return { logs, connected, clear };
}

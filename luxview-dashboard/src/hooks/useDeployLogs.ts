import { useState, useCallback } from 'react';

export interface LogLine {
  timestamp: string;
  level: 'info' | 'error' | 'warn' | 'debug';
  message: string;
}

// WebSocket log streaming is not yet implemented on the backend.
// This hook returns an empty log list until the /ws/logs endpoint is added.
export function useDeployLogs(_appId: string, _deployId?: string) {
  const [logs, setLogs] = useState<LogLine[]>([]);
  const connected = false;
  const clear = useCallback(() => setLogs([]), []);

  return { logs, connected, clear };
}

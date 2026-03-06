import { useEffect, useRef } from 'react';
import { WebSocketClient } from '../lib/websocket';

export function useWebSocket(url: string) {
  const clientRef = useRef<WebSocketClient | null>(null);

  useEffect(() => {
    const client = new WebSocketClient(url);
    client.connect();
    clientRef.current = client;

    return () => {
      client.disconnect();
      clientRef.current = null;
    };
  }, [url]);

  return clientRef;
}

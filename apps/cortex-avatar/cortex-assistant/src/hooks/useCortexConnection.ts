import { useEffect, useRef, useCallback } from 'react';
import { useCortexStore, useSettingsStore } from '@/store';
import { getCortexClient, resetCortexClient } from '@/services/cortex';

const PING_INTERVAL = 30000;
const RECONNECT_DELAY = 5000;

export function useCortexConnection() {
  const { settings } = useSettingsStore();
  const { status, setStatus, recordPing, setError } = useCortexStore();
  const pingIntervalRef = useRef<number | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);

  const checkConnection = useCallback(async () => {
    try {
      setStatus('connecting');
      const client = getCortexClient({ baseUrl: settings.cortexUrl });
      const latency = await client.ping();
      recordPing(latency);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Connection failed');
      setStatus('offline');

      reconnectTimeoutRef.current = window.setTimeout(() => {
        checkConnection();
      }, RECONNECT_DELAY);
    }
  }, [settings.cortexUrl, setStatus, recordPing, setError]);

  useEffect(() => {
    resetCortexClient();
    checkConnection();

    pingIntervalRef.current = window.setInterval(checkConnection, PING_INTERVAL);

    return () => {
      if (pingIntervalRef.current) {
        clearInterval(pingIntervalRef.current);
      }
      if (reconnectTimeoutRef.current) {
        clearTimeout(reconnectTimeoutRef.current);
      }
    };
  }, [settings.cortexUrl, checkConnection]);

  return {
    status,
    reconnect: checkConnection,
  };
}

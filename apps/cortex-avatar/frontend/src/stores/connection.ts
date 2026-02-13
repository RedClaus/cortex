import { writable } from 'svelte/store';

export interface ConnectionState {
  isConnected: boolean;
  serverUrl: string;
  agentName: string;
  agentVersion: string;
  lastPing: number;
  error: string | null;
}

export const connectionState = writable<ConnectionState>({
  isConnected: false,
  serverUrl: 'http://localhost:8080',
  agentName: '',
  agentVersion: '',
  lastPing: 0,
  error: null,
});

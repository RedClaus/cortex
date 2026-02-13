import { create } from 'zustand';
import type { ConnectionStatus } from '@/models';

interface CortexState {
  status: ConnectionStatus;
  lastPing: number | null;
  latency: number | null;
  error: string | null;
  isAnalyzing: boolean;
  isCommitting: boolean;
  isIngesting: boolean;
  isSearching: boolean;

  setStatus: (status: ConnectionStatus) => void;
  setLatency: (latency: number) => void;
  setError: (error: string | null) => void;
  setAnalyzing: (analyzing: boolean) => void;
  setCommitting: (committing: boolean) => void;
  setIngesting: (ingesting: boolean) => void;
  setSearching: (searching: boolean) => void;
  recordPing: (latency: number) => void;
}

export const useCortexStore = create<CortexState>((set) => ({
  status: 'offline',
  lastPing: null,
  latency: null,
  error: null,
  isAnalyzing: false,
  isCommitting: false,
  isIngesting: false,
  isSearching: false,

  setStatus: (status) => set({ status }),

  setLatency: (latency) => set({ latency }),

  setError: (error) =>
    set({
      error,
      status: error ? 'degraded' : 'connected',
    }),

  setAnalyzing: (isAnalyzing) => set({ isAnalyzing }),

  setCommitting: (isCommitting) => set({ isCommitting }),

  setIngesting: (isIngesting) => set({ isIngesting }),

  setSearching: (isSearching) => set({ isSearching }),

  recordPing: (latency) =>
    set({
      lastPing: Date.now(),
      latency,
      status: 'connected',
      error: null,
    }),
}));

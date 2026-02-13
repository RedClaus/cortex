import { create } from 'zustand';
import type { TranscriptionMode } from '@/models';

interface TranscriptionState {
  isListening: boolean;
  mode: TranscriptionMode;
  language: string;
  error: string | null;
  isSupported: boolean;
  confidence: number;
  
  setListening: (listening: boolean) => void;
  setMode: (mode: TranscriptionMode) => void;
  setLanguage: (language: string) => void;
  setError: (error: string | null) => void;
  setSupported: (supported: boolean) => void;
  setConfidence: (confidence: number) => void;
  reset: () => void;
}

export const useTranscriptionStore = create<TranscriptionState>((set) => ({
  isListening: false,
  mode: 'web_speech',
  language: 'en-US',
  error: null,
  isSupported: typeof window !== 'undefined' && 'webkitSpeechRecognition' in window,
  confidence: 0,

  setListening: (isListening) => set({ isListening }),
  setMode: (mode) => set({ mode }),
  setLanguage: (language) => set({ language }),
  setError: (error) => set({ error }),
  setSupported: (isSupported) => set({ isSupported }),
  setConfidence: (confidence) => set({ confidence }),
  reset: () =>
    set({
      isListening: false,
      error: null,
      confidence: 0,
    }),
}));

import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import type { AppSettings, TranscriptionMode, RedactionPattern } from '@/models';
import { DEFAULT_SETTINGS } from '@/models';

interface SettingsState {
  settings: AppSettings;
  setTheme: (theme: AppSettings['theme']) => void;
  setCortexUrl: (url: string) => void;
  setTranscriptionMode: (mode: TranscriptionMode) => void;
  setAutoSave: (enabled: boolean, interval?: number) => void;
  setDefaultLanguage: (language: string) => void;
  toggleKeyboardShortcuts: () => void;
  updateRedactionPattern: (pattern: RedactionPattern) => void;
  addRedactionPattern: (pattern: RedactionPattern) => void;
  removeRedactionPattern: (id: string) => void;
  resetSettings: () => void;
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      settings: DEFAULT_SETTINGS,

      setTheme: (theme) =>
        set((state) => ({
          settings: { ...state.settings, theme },
        })),

      setCortexUrl: (cortexUrl) =>
        set((state) => ({
          settings: { ...state.settings, cortexUrl },
        })),

      setTranscriptionMode: (transcriptionMode) =>
        set((state) => ({
          settings: { ...state.settings, transcriptionMode },
        })),

      setAutoSave: (autoSaveEnabled, autoSaveInterval) =>
        set((state) => ({
          settings: {
            ...state.settings,
            autoSaveEnabled,
            ...(autoSaveInterval !== undefined && { autoSaveInterval }),
          },
        })),

      setDefaultLanguage: (defaultLanguage) =>
        set((state) => ({
          settings: { ...state.settings, defaultLanguage },
        })),

      toggleKeyboardShortcuts: () =>
        set((state) => ({
          settings: {
            ...state.settings,
            keyboardShortcutsEnabled: !state.settings.keyboardShortcutsEnabled,
          },
        })),

      updateRedactionPattern: (pattern) =>
        set((state) => ({
          settings: {
            ...state.settings,
            redactionPatterns: state.settings.redactionPatterns.map((p) =>
              p.id === pattern.id ? pattern : p
            ),
          },
        })),

      addRedactionPattern: (pattern) =>
        set((state) => ({
          settings: {
            ...state.settings,
            redactionPatterns: [...state.settings.redactionPatterns, pattern],
          },
        })),

      removeRedactionPattern: (id) =>
        set((state) => ({
          settings: {
            ...state.settings,
            redactionPatterns: state.settings.redactionPatterns.filter(
              (p) => p.id !== id
            ),
          },
        })),

      resetSettings: () =>
        set({
          settings: DEFAULT_SETTINGS,
        }),
    }),
    {
      name: 'cortex-assistant-settings',
    }
  )
);

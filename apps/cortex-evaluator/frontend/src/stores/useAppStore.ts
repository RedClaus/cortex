import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { AppStore, ApiSettings } from './types';

interface AuthSlice {
  userId: string | null;
  setUserId: (userId: string | null) => void;
}

interface ThemeSlice {
  isDarkMode: boolean;
  toggleTheme: () => void;
}

interface ProviderSlice {
  selectedProvider: string;
  setSelectedProvider: (provider: string) => void;
}

interface SettingsSlice {
  apiSettings: ApiSettings;
  isSettingsOpen: boolean;
  setApiSettings: (settings: Partial<ApiSettings>) => void;
  setSettingsOpen: (open: boolean) => void;
}

const defaultApiSettings: ApiSettings = {
  openaiApiKey: '',
  anthropicApiKey: '',
  geminiApiKey: '',
  groqApiKey: '',
  ollamaBaseUrl: 'http://localhost:11434',
};

const createAuthSlice: (
  set: (partial: Partial<AppStore>) => void
) => AuthSlice = (set) => ({
  userId: null,
  setUserId: (userId) => set({ userId }),
});

const createThemeSlice: (
  set: (partial: Partial<AppStore> | ((state: AppStore) => Partial<AppStore>)) => void,
  get: () => AppStore
) => ThemeSlice = (set) => ({
  isDarkMode: false,
  toggleTheme: () => set((state) => ({ isDarkMode: !state.isDarkMode })),
});

const createProviderSlice: (
  set: (partial: Partial<AppStore>) => void
) => ProviderSlice = (set) => ({
  selectedProvider: 'openai',
  setSelectedProvider: (provider) => set({ selectedProvider: provider }),
});

const createSettingsSlice: (
  set: (partial: Partial<AppStore> | ((state: AppStore) => Partial<AppStore>)) => void,
  get: () => AppStore
) => SettingsSlice = (set, get) => ({
  apiSettings: defaultApiSettings,
  isSettingsOpen: false,
  setApiSettings: (settings) => set((state) => ({
    apiSettings: { ...state.apiSettings, ...settings }
  })),
  setSettingsOpen: (open) => set({ isSettingsOpen: open }),
});

export const useAppStore = create<AppStore>()(
  persist(
    (set, get) => ({
      ...createAuthSlice(set),
      ...createThemeSlice(set, get),
      ...createProviderSlice(set),
      ...createSettingsSlice(set, get),
    }),
    {
      name: 'cortex-evaluator-app',
      partialize: (state) => ({
        isDarkMode: state.isDarkMode,
        selectedProvider: state.selectedProvider,
        apiSettings: state.apiSettings,
      }),
    }
  )
);

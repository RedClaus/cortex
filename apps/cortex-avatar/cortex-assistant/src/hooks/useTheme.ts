import { useEffect, useCallback } from 'react';
import { useSettingsStore } from '@/store';

export function useTheme() {
  const { settings, setTheme } = useSettingsStore();

  const applyTheme = useCallback((theme: 'light' | 'dark' | 'system') => {
    const root = document.documentElement;

    if (theme === 'system') {
      const systemDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
      root.classList.toggle('dark', systemDark);
    } else {
      root.classList.toggle('dark', theme === 'dark');
    }
  }, []);

  useEffect(() => {
    applyTheme(settings.theme);

    if (settings.theme === 'system') {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)');
      const handler = () => applyTheme('system');
      mediaQuery.addEventListener('change', handler);
      return () => mediaQuery.removeEventListener('change', handler);
    }
  }, [settings.theme, applyTheme]);

  const toggleTheme = useCallback(() => {
    const newTheme = settings.theme === 'dark' ? 'light' : 'dark';
    setTheme(newTheme);
  }, [settings.theme, setTheme]);

  const isDark =
    settings.theme === 'dark' ||
    (settings.theme === 'system' &&
      typeof window !== 'undefined' &&
      window.matchMedia('(prefers-color-scheme: dark)').matches);

  return {
    theme: settings.theme,
    isDark,
    setTheme,
    toggleTheme,
  };
}

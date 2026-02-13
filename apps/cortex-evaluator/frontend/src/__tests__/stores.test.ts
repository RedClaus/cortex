import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia, createPinia } from 'pinia';
import { createTestingPinia } from '@pinia/testing';

describe('App Store', () => {
  let useAppStore: ReturnType<typeof import('../stores/useAppStore')['useAppStore']>;
  let pinia: ReturnType<typeof import('pinia')['createPinia']>;

  beforeEach(() => {
    pinia = createPinia();
    setActivePinia(pinia);
    vi.useFakeTimers();
  });

  it('should initialize with default state', () => {
    const { useAppStore: useAppStoreFactory } = await import('../stores/useAppStore');
    useAppStore = useAppStoreFactory();
    const { userId, isDarkMode, selectedProvider } = useAppStore();

    expect(userId).toBe(null);
    expect(isDarkMode).toBe(false);
    expect(selectedProvider).toBe('openai');
  });

  it('should set userId', () => {
    const { useAppStore: useAppStoreFactory } = await import('../stores/useAppStore');
    useAppStore = useAppStoreFactory();
    const { setUserId, userId } = useAppStore();

    setUserId('user-123');

    expect(userId).toBe('user-123');
  });

  it('should set userId to null', () => {
    const { useAppStore: useAppStoreFactory } = await import('../stores/useAppStore');
    useAppStore = useAppStoreFactory();
    const { setUserId, userId } = useAppStore();

    setUserId(null);

    expect(userId).toBe(null);
  });

  it('should toggle theme', () => {
    const { useAppStore: useAppStoreFactory } = await import('../stores/useAppStore');
    useAppStore = useAppStoreFactory();
    const { toggleTheme, isDarkMode } = useAppStore();

    expect(isDarkMode).toBe(false);

    toggleTheme();

    expect(isDarkMode).toBe(true);

    toggleTheme();

    expect(isDarkMode).toBe(false);
  });

  it('should set selected provider', () => {
    const { useAppStore: useAppStoreFactory } = await import('../stores/useAppStore');
    useAppStore = useAppStoreFactory();
    const { setSelectedProvider, selectedProvider } = useAppStore();

    setSelectedProvider('anthropic');

    expect(selectedProvider).toBe('anthropic');
  });

  it('should persist state to localStorage', () => {
    const { useAppStore: useAppStoreFactory } = await import('../stores/useAppStore');
    useAppStore = useAppStoreFactory();
    const { setUserId } = useAppStore();

    setUserId('persisted-user');

    vi.runAllTimers();

    expect(localStorage.getItem('cortex-evaluator-app')).toBeTruthy();
  });
});

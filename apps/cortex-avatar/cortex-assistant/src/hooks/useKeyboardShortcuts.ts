import { useEffect, useCallback } from 'react';
import { useSettingsStore, useUIStore, useMeetingStore } from '@/store';

interface ShortcutHandler {
  key: string;
  ctrl?: boolean;
  shift?: boolean;
  alt?: boolean;
  handler: () => void;
  description: string;
}

export function useKeyboardShortcuts(shortcuts: ShortcutHandler[] = []) {
  const { settings } = useSettingsStore();
  const { toggleCommandPalette } = useUIStore();
  const {
    recordingStatus,
    startRecording,
    stopRecording,
    pauseRecording,
    resumeRecording,
    toggleAutoScroll,
  } = useMeetingStore();

  const handleKeyDown = useCallback(
    (event: KeyboardEvent) => {
      if (!settings.keyboardShortcutsEnabled) return;

      const isInputFocused =
        document.activeElement?.tagName === 'INPUT' ||
        document.activeElement?.tagName === 'TEXTAREA' ||
        (document.activeElement as HTMLElement)?.isContentEditable;

      if (isInputFocused && !event.ctrlKey && !event.metaKey) return;

      const isMac = navigator.platform.toUpperCase().indexOf('MAC') >= 0;
      const ctrlOrCmd = isMac ? event.metaKey : event.ctrlKey;

      if (ctrlOrCmd && event.key === 'k') {
        event.preventDefault();
        toggleCommandPalette();
        return;
      }

      if (ctrlOrCmd && event.shiftKey && event.key === 'r') {
        event.preventDefault();
        if (recordingStatus === 'idle') {
          startRecording();
        } else if (recordingStatus === 'recording') {
          stopRecording();
        }
        return;
      }

      if (ctrlOrCmd && event.shiftKey && event.key === 'p') {
        event.preventDefault();
        if (recordingStatus === 'recording') {
          pauseRecording();
        } else if (recordingStatus === 'paused') {
          resumeRecording();
        }
        return;
      }

      if (ctrlOrCmd && event.key === 'j') {
        event.preventDefault();
        toggleAutoScroll();
        return;
      }

      for (const shortcut of shortcuts) {
        const ctrlMatch = shortcut.ctrl ? ctrlOrCmd : !ctrlOrCmd || !shortcut.ctrl;
        const shiftMatch = shortcut.shift ? event.shiftKey : !event.shiftKey || !shortcut.shift;
        const altMatch = shortcut.alt ? event.altKey : !event.altKey || !shortcut.alt;

        if (
          event.key.toLowerCase() === shortcut.key.toLowerCase() &&
          ctrlMatch &&
          shiftMatch &&
          altMatch
        ) {
          event.preventDefault();
          shortcut.handler();
          return;
        }
      }
    },
    [
      settings.keyboardShortcutsEnabled,
      shortcuts,
      toggleCommandPalette,
      recordingStatus,
      startRecording,
      stopRecording,
      pauseRecording,
      resumeRecording,
      toggleAutoScroll,
    ]
  );

  useEffect(() => {
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [handleKeyDown]);
}

export const GLOBAL_SHORTCUTS = [
  { key: 'K', ctrl: true, description: 'Open command palette' },
  { key: 'R', ctrl: true, shift: true, description: 'Start/stop recording' },
  { key: 'P', ctrl: true, shift: true, description: 'Pause/resume recording' },
  { key: 'S', ctrl: true, description: 'Save meeting' },
  { key: 'J', ctrl: true, description: 'Toggle auto-scroll' },
  { key: 'E', ctrl: true, shift: true, description: 'Export meeting' },
  { key: 'A', ctrl: true, shift: true, description: 'Analyze meeting' },
];

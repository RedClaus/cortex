import { useEffect, useRef, useCallback } from 'react';
import { useMeetingStore, useSettingsStore } from '@/store';
import { setAutoSaveMeeting, clearAutoSaveMeeting } from '@/services/meeting';

export function useAutoSave() {
  const { currentMeeting, recordingStatus } = useMeetingStore();
  const { settings } = useSettingsStore();
  const intervalRef = useRef<number | null>(null);
  const lastSaveRef = useRef<string | null>(null);

  const save = useCallback(async () => {
    if (!currentMeeting) return;

    const meetingHash = JSON.stringify({
      id: currentMeeting.id,
      segmentsLength: currentMeeting.segments.length,
      updatedAt: currentMeeting.updatedAt,
    });

    if (meetingHash === lastSaveRef.current) return;

    try {
      await setAutoSaveMeeting(currentMeeting);
      lastSaveRef.current = meetingHash;
    } catch (err) {
      console.error('Auto-save failed:', err);
    }
  }, [currentMeeting]);

  useEffect(() => {
    if (!settings.autoSaveEnabled || !currentMeeting) {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
      return;
    }

    if (recordingStatus === 'recording' || recordingStatus === 'paused') {
      save();

      intervalRef.current = window.setInterval(save, settings.autoSaveInterval);
    }

    return () => {
      if (intervalRef.current) {
        clearInterval(intervalRef.current);
        intervalRef.current = null;
      }
    };
  }, [
    settings.autoSaveEnabled,
    settings.autoSaveInterval,
    currentMeeting,
    recordingStatus,
    save,
  ]);

  const clearAutoSave = useCallback(async () => {
    await clearAutoSaveMeeting();
    lastSaveRef.current = null;
  }, []);

  return { save, clearAutoSave };
}

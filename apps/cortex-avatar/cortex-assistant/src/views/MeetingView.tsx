import { useEffect } from 'react';
import { useMeetingStore } from '@/store';
import { useAutoSave, useKeyboardShortcuts } from '@/hooks';
import { getAutoSaveMeeting, clearAutoSaveMeeting } from '@/services/meeting';
import {
  MeetingHeader,
  TranscriptView,
  RecordingControls,
  AnalysisPanel,
} from '@/components/meeting';

export function MeetingView() {
  const { currentMeeting, createNewMeeting, loadMeeting } = useMeetingStore();

  useAutoSave();
  useKeyboardShortcuts();

  useEffect(() => {
    const init = async () => {
      if (!currentMeeting) {
        const autoSaved = await getAutoSaveMeeting();
        if (autoSaved) {
          const restore = window.confirm(
            'Found an auto-saved meeting. Would you like to restore it?'
          );
          if (restore) {
            loadMeeting(autoSaved);
          } else {
            await clearAutoSaveMeeting();
            createNewMeeting();
          }
        } else {
          createNewMeeting();
        }
      }
    };

    init();
  }, [currentMeeting, createNewMeeting, loadMeeting]);

  if (!currentMeeting) {
    return (
      <div className="flex items-center justify-center h-full">
        <div className="animate-pulse text-gray-500">Loading...</div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      <MeetingHeader />

      <div className="flex-1 flex overflow-hidden">
        <div className="flex-1 flex flex-col min-w-0">
          <TranscriptView className="flex-1" />
          <RecordingControls />
        </div>

        <aside className="w-80 border-l border-gray-200 dark:border-surface-700 bg-white dark:bg-surface-900 overflow-hidden">
          <AnalysisPanel />
        </aside>
      </div>
    </div>
  );
}

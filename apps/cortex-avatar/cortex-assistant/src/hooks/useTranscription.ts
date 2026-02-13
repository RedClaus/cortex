import { useCallback, useEffect } from 'react';
import { useMeetingStore, useTranscriptionStore, useSettingsStore } from '@/store';
import { transcriptionService } from '@/services/transcription';
import type { TranscriptSegment } from '@/models';

export function useTranscription() {
  const {
    currentMeeting,
    recordingStatus,
    addSegment,
    setInterimText,
    startRecording: startMeetingRecording,
    stopRecording: stopMeetingRecording,
    pauseRecording: pauseMeetingRecording,
    resumeRecording: resumeMeetingRecording,
  } = useMeetingStore();

  const {
    isListening,
    mode,
    error,
    isSupported,
    setListening,
    setError,
    setConfidence,
  } = useTranscriptionStore();

  const { settings } = useSettingsStore();

  const handleTranscript = useCallback(
    (segment: TranscriptSegment | null, interim: string) => {
      if (segment) {
        addSegment(segment);
        setConfidence(segment.confidence || 0);
      } else {
        setInterimText(interim);
      }
    },
    [addSegment, setInterimText, setConfidence]
  );

  const handleError = useCallback(
    (err: string) => {
      setError(err);
    },
    [setError]
  );

  useEffect(() => {
    transcriptionService.setCallbacks(handleTranscript, handleError);
    transcriptionService.setMode(settings.transcriptionMode);
    transcriptionService.setLanguage(settings.defaultLanguage);
  }, [settings.transcriptionMode, settings.defaultLanguage, handleTranscript, handleError]);

  const start = useCallback(async () => {
    if (!currentMeeting) return;

    try {
      setError(null);
      const speaker = currentMeeting.speakers[0];
      transcriptionService.setSpeaker(speaker?.id || 'speaker-1', speaker?.label || 'Speaker 1');

      await transcriptionService.start();
      setListening(true);
      startMeetingRecording();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to start transcription');
    }
  }, [currentMeeting, setListening, setError, startMeetingRecording]);

  const stop = useCallback(() => {
    transcriptionService.stop();
    setListening(false);
    stopMeetingRecording();
  }, [setListening, stopMeetingRecording]);

  const pause = useCallback(() => {
    transcriptionService.pause();
    pauseMeetingRecording();
  }, [pauseMeetingRecording]);

  const resume = useCallback(() => {
    transcriptionService.resume();
    resumeMeetingRecording();
  }, [resumeMeetingRecording]);

  const changeSpeaker = useCallback(
    (speakerId: string) => {
      const speaker = currentMeeting?.speakers.find((s) => s.id === speakerId);
      if (speaker) {
        transcriptionService.setSpeaker(speaker.id, speaker.label);
      }
    },
    [currentMeeting]
  );

  return {
    isListening,
    mode,
    error,
    isSupported,
    recordingStatus,
    start,
    stop,
    pause,
    resume,
    changeSpeaker,
  };
}

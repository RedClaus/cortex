import { useEffect, useRef } from 'react';
import { clsx } from 'clsx';
import {
  Mic,
  Pause,
  Play,
  Square,
  Save,
  Sparkles,
  Download,
  Users,
} from 'lucide-react';
import { useMeetingStore, useCortexStore, useUIStore } from '@/store';
import { useTranscription, useAutoSave } from '@/hooks';
import { saveMeeting } from '@/services/meeting';
import { getCortexClient } from '@/services/cortex';
import { formatDuration } from '@/utils/format';
import { Button, Select } from '@/components/ui';

export function RecordingControls() {
  const {
    currentMeeting,
    recordingStatus,
    elapsedTime,
    updateElapsedTime,
    setAnalysis,
    toggleAutoScroll,
    autoScrollEnabled,
    setSelectedSpeaker,
    selectedSpeakerId,
    addSpeaker,
  } = useMeetingStore();

  const { isAnalyzing, setAnalyzing, status: cortexStatus } = useCortexStore();
  const { openExportModal } = useUIStore();
  const { start, stop, pause, resume, isSupported, error } = useTranscription();
  const { save } = useAutoSave();

  const timerRef = useRef<number | null>(null);
  const startTimeRef = useRef<number>(0);

  useEffect(() => {
    if (recordingStatus === 'recording') {
      startTimeRef.current = Date.now() - elapsedTime;
      timerRef.current = window.setInterval(() => {
        updateElapsedTime(Date.now() - startTimeRef.current);
      }, 1000);
    } else if (timerRef.current) {
      clearInterval(timerRef.current);
      timerRef.current = null;
    }

    return () => {
      if (timerRef.current) {
        clearInterval(timerRef.current);
      }
    };
  }, [recordingStatus, elapsedTime, updateElapsedTime]);

  const handleStartStop = () => {
    if (recordingStatus === 'recording' || recordingStatus === 'paused') {
      stop();
    } else {
      start();
    }
  };

  const handlePauseResume = () => {
    if (recordingStatus === 'recording') {
      pause();
    } else if (recordingStatus === 'paused') {
      resume();
    }
  };

  const handleSave = async () => {
    if (currentMeeting) {
      await saveMeeting(currentMeeting);
      save();
    }
  };

  const handleAnalyze = async () => {
    if (!currentMeeting || isAnalyzing) return;

    setAnalyzing(true);
    try {
      const client = getCortexClient();
      const analysis = await client.analyzeMeeting(currentMeeting);
      setAnalysis(analysis);
    } catch (err) {
      console.error('Analysis failed:', err);
    } finally {
      setAnalyzing(false);
    }
  };

  const handleAddSpeaker = () => {
    const count = currentMeeting?.speakers.length || 0;
    addSpeaker(`Speaker ${count + 1}`);
  };

  const speakers = currentMeeting?.speakers || [];
  const speakerOptions = [
    { value: '', label: 'All Speakers' },
    ...speakers.map((s) => ({ value: s.id, label: s.label })),
  ];

  const isRecording = recordingStatus === 'recording';
  const isPaused = recordingStatus === 'paused';
  const hasContent = (currentMeeting?.segments.length || 0) > 0;

  return (
    <div className="flex flex-col gap-4 p-4 bg-white dark:bg-surface-900 border-t border-gray-200 dark:border-surface-700">
      {error && (
        <div className="px-3 py-2 text-sm text-red-600 bg-red-50 dark:bg-red-900/20 dark:text-red-400 rounded-lg">
          {error}
        </div>
      )}

      <div className="flex items-center justify-between">
        <div className="flex items-center gap-4">
          <div
            className={clsx(
              'text-2xl font-mono tabular-nums',
              isRecording && 'text-red-500'
            )}
          >
            {formatDuration(elapsedTime)}
          </div>

          {isRecording && (
            <div className="flex items-center gap-2">
              <div className="relative">
                <div className="w-3 h-3 bg-red-500 rounded-full" />
                <div className="absolute inset-0 w-3 h-3 bg-red-500 rounded-full animate-ping" />
              </div>
              <span className="text-sm font-medium text-red-500">REC</span>
            </div>
          )}

          {isPaused && (
            <span className="text-sm font-medium text-yellow-500">PAUSED</span>
          )}
        </div>

        <div className="flex items-center gap-2">
          <Select
            value={selectedSpeakerId || ''}
            onChange={setSelectedSpeaker}
            options={speakerOptions}
            className="w-40"
          />
          <Button
            variant="ghost"
            size="sm"
            onClick={handleAddSpeaker}
            icon={<Users className="w-4 h-4" />}
            title="Add Speaker"
          />
        </div>
      </div>

      <div className="flex items-center justify-between gap-4">
        <div className="flex items-center gap-2">
          <Button
            variant={isRecording || isPaused ? 'danger' : 'primary'}
            onClick={handleStartStop}
            disabled={!isSupported}
            icon={
              isRecording || isPaused ? (
                <Square className="w-4 h-4" />
              ) : (
                <Mic className="w-4 h-4" />
              )
            }
          >
            {isRecording || isPaused ? 'Stop' : 'Start Recording'}
          </Button>

          {(isRecording || isPaused) && (
            <Button
              variant="secondary"
              onClick={handlePauseResume}
              icon={isPaused ? <Play className="w-4 h-4" /> : <Pause className="w-4 h-4" />}
            >
              {isPaused ? 'Resume' : 'Pause'}
            </Button>
          )}

          <Button
            variant="ghost"
            onClick={toggleAutoScroll}
            className={clsx(autoScrollEnabled && 'text-primary-500')}
          >
            Auto-scroll: {autoScrollEnabled ? 'On' : 'Off'}
          </Button>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="secondary"
            onClick={handleSave}
            disabled={!hasContent}
            icon={<Save className="w-4 h-4" />}
          >
            Save
          </Button>

          <Button
            variant="secondary"
            onClick={handleAnalyze}
            disabled={!hasContent || isAnalyzing || cortexStatus !== 'connected'}
            loading={isAnalyzing}
            icon={<Sparkles className="w-4 h-4" />}
          >
            Analyze
          </Button>

          <Button
            variant="secondary"
            onClick={openExportModal}
            disabled={!hasContent}
            icon={<Download className="w-4 h-4" />}
          >
            Export
          </Button>
        </div>
      </div>
    </div>
  );
}

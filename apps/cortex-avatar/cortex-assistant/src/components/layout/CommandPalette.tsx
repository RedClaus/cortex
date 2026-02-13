import { useCallback, useMemo } from 'react';
import { Command } from 'cmdk';
import { createPortal } from 'react-dom';
import { useNavigate } from 'react-router-dom';
import {
  Mic,
  MicOff,
  Save,
  Download,
  Search,
  Settings,
  Moon,
  Sun,
  Plus,
  History,
  Brain,
  ListTodo,
  Home,
  Sparkles,
} from 'lucide-react';
import { useUIStore, useMeetingStore, useCortexStore } from '@/store';
import { useTheme, useTranscription } from '@/hooks';
import { saveMeeting } from '@/services/meeting';
import { getCortexClient } from '@/services/cortex';

export function CommandPalette() {
  const navigate = useNavigate();
  const { commandPaletteOpen, closeCommandPalette, openExportModal, openSettings } = useUIStore();
  const { currentMeeting, createNewMeeting, setAnalysis } = useMeetingStore();
  const { isAnalyzing, setAnalyzing } = useCortexStore();
  const { isDark, toggleTheme } = useTheme();
  const { isListening, start, stop, recordingStatus } = useTranscription();

  const handleSelect = useCallback(
    (value: string) => {
      closeCommandPalette();

      switch (value) {
        case 'nav-home':
          navigate('/');
          break;
        case 'nav-history':
          navigate('/history');
          break;
        case 'nav-tasks':
          navigate('/tasks');
          break;
        case 'nav-memory':
          navigate('/memory');
          break;
        case 'new-meeting':
          createNewMeeting();
          navigate('/meeting');
          break;
        case 'toggle-recording':
          if (recordingStatus === 'recording') {
            stop();
          } else if (currentMeeting) {
            start();
          }
          break;
        case 'save-meeting':
          if (currentMeeting) {
            saveMeeting(currentMeeting);
          }
          break;
        case 'analyze-meeting':
          if (currentMeeting && !isAnalyzing) {
            setAnalyzing(true);
            const client = getCortexClient();
            client
              .analyzeMeeting(currentMeeting)
              .then((analysis) => {
                setAnalysis(analysis);
              })
              .finally(() => {
                setAnalyzing(false);
              });
          }
          break;
        case 'export':
          openExportModal();
          break;
        case 'settings':
          openSettings();
          break;
        case 'toggle-theme':
          toggleTheme();
          break;
      }
    },
    [
      closeCommandPalette,
      navigate,
      createNewMeeting,
      recordingStatus,
      start,
      stop,
      currentMeeting,
      isAnalyzing,
      setAnalyzing,
      setAnalysis,
      openExportModal,
      openSettings,
      toggleTheme,
    ]
  );

  const groups = useMemo(
    () => [
      {
        heading: 'Navigation',
        items: [
          { value: 'nav-home', icon: Home, label: 'Go to Dashboard' },
          { value: 'nav-history', icon: History, label: 'Meeting History' },
          { value: 'nav-tasks', icon: ListTodo, label: 'Tasks' },
          { value: 'nav-memory', icon: Brain, label: 'Memory Search' },
        ],
      },
      {
        heading: 'Meeting',
        items: [
          { value: 'new-meeting', icon: Plus, label: 'New Meeting' },
          {
            value: 'toggle-recording',
            icon: isListening ? MicOff : Mic,
            label: isListening ? 'Stop Recording' : 'Start Recording',
            disabled: !currentMeeting,
          },
          {
            value: 'save-meeting',
            icon: Save,
            label: 'Save Meeting',
            disabled: !currentMeeting,
          },
          {
            value: 'analyze-meeting',
            icon: Sparkles,
            label: 'Analyze with Cortex',
            disabled: !currentMeeting || isAnalyzing,
          },
          {
            value: 'export',
            icon: Download,
            label: 'Export Meeting',
            disabled: !currentMeeting,
          },
        ],
      },
      {
        heading: 'Settings',
        items: [
          { value: 'settings', icon: Settings, label: 'Open Settings' },
          {
            value: 'toggle-theme',
            icon: isDark ? Sun : Moon,
            label: isDark ? 'Light Mode' : 'Dark Mode',
          },
        ],
      },
    ],
    [isListening, currentMeeting, isAnalyzing, isDark]
  );

  if (!commandPaletteOpen) return null;

  return createPortal(
    <div className="fixed inset-0 z-50">
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={closeCommandPalette}
      />
      <div className="relative flex items-start justify-center pt-[20vh]">
        <Command
          className="w-full max-w-lg bg-white dark:bg-surface-900 rounded-xl shadow-2xl border border-gray-200 dark:border-surface-700 overflow-hidden animate-slide-down"
          loop
        >
          <div className="flex items-center gap-2 px-4 py-3 border-b border-gray-200 dark:border-surface-700">
            <Search className="w-4 h-4 text-gray-400" />
            <Command.Input
              placeholder="Type a command or search..."
              className="flex-1 bg-transparent border-0 outline-none text-gray-900 dark:text-white placeholder:text-gray-400"
              autoFocus
            />
            <kbd className="hidden sm:inline-flex items-center gap-1 px-2 py-0.5 text-xs font-medium text-gray-400 bg-gray-100 dark:bg-surface-800 rounded">
              ESC
            </kbd>
          </div>
          <Command.List className="max-h-[300px] overflow-y-auto p-2 scrollbar-thin">
            <Command.Empty className="py-6 text-center text-sm text-gray-500">
              No results found.
            </Command.Empty>
            {groups.map((group) => (
              <Command.Group key={group.heading} heading={group.heading}>
                <div className="px-2 py-1 text-xs font-medium text-gray-400 dark:text-gray-500">
                  {group.heading}
                </div>
                {group.items.map((item) => (
                  <Command.Item
                    key={item.value}
                    value={item.value}
                    disabled={item.disabled}
                    onSelect={handleSelect}
                    className="flex items-center gap-3 px-3 py-2 text-sm rounded-lg cursor-pointer text-gray-700 dark:text-gray-300 data-[selected=true]:bg-primary-50 dark:data-[selected=true]:bg-primary-900/20 data-[selected=true]:text-primary-600 dark:data-[selected=true]:text-primary-400 data-[disabled=true]:opacity-50 data-[disabled=true]:cursor-not-allowed"
                  >
                    <item.icon className="w-4 h-4" />
                    <span>{item.label}</span>
                  </Command.Item>
                ))}
              </Command.Group>
            ))}
          </Command.List>
        </Command>
      </div>
    </div>,
    document.body
  );
}

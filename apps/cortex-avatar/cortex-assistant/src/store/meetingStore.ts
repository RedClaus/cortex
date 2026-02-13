import { create } from 'zustand';
import type {
  MeetingSession,
  TranscriptSegment,
  MeetingAnalysis,
  Speaker,
  Participant,
  RecordingStatus,
} from '@/models';
import { createDefaultMeetingSession, generateId, SPEAKER_COLORS } from '@/models';

interface MeetingState {
  currentMeeting: MeetingSession | null;
  recentMeetings: MeetingSession[];
  recordingStatus: RecordingStatus;
  recordingStartTime: number | null;
  elapsedTime: number;
  autoScrollEnabled: boolean;
  selectedSpeakerId: string | null;
  interimText: string;

  createNewMeeting: (title?: string) => MeetingSession;
  loadMeeting: (meeting: MeetingSession) => void;
  updateMeetingTitle: (title: string) => void;
  updateMeetingDescription: (description: string) => void;
  addParticipant: (participant: Participant) => void;
  removeParticipant: (id: string) => void;
  addSpeaker: (label: string) => Speaker;
  updateSpeaker: (id: string, updates: Partial<Speaker>) => void;
  removeSpeaker: (id: string) => void;
  addSegment: (segment: TranscriptSegment) => void;
  updateSegment: (id: string, updates: Partial<TranscriptSegment>) => void;
  deleteSegment: (id: string) => void;
  setInterimText: (text: string) => void;
  setRecordingStatus: (status: RecordingStatus) => void;
  startRecording: () => void;
  stopRecording: () => void;
  pauseRecording: () => void;
  resumeRecording: () => void;
  updateElapsedTime: (time: number) => void;
  setAnalysis: (analysis: MeetingAnalysis) => void;
  clearAnalysis: () => void;
  toggleAutoScroll: () => void;
  setSelectedSpeaker: (speakerId: string | null) => void;
  addTag: (tag: string) => void;
  removeTag: (tag: string) => void;
  closeMeeting: () => void;
  setRecentMeetings: (meetings: MeetingSession[]) => void;
  addToRecentMeetings: (meeting: MeetingSession) => void;
}

export const useMeetingStore = create<MeetingState>((set, get) => ({
  currentMeeting: null,
  recentMeetings: [],
  recordingStatus: 'idle',
  recordingStartTime: null,
  elapsedTime: 0,
  autoScrollEnabled: true,
  selectedSpeakerId: null,
  interimText: '',

  createNewMeeting: (title) => {
    const id = generateId();
    const meeting = createDefaultMeetingSession(id);
    if (title) meeting.title = title;
    set({ currentMeeting: meeting, recordingStatus: 'idle', elapsedTime: 0 });
    return meeting;
  },

  loadMeeting: (meeting) => {
    set({
      currentMeeting: meeting,
      recordingStatus: 'idle',
      elapsedTime: meeting.duration,
    });
  },

  updateMeetingTitle: (title) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          title,
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  updateMeetingDescription: (description) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          description,
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  addParticipant: (participant) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          participants: [...state.currentMeeting.participants, participant],
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  removeParticipant: (id) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          participants: state.currentMeeting.participants.filter((p) => p.id !== id),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  addSpeaker: (label) => {
    const state = get();
    const colorIndex = state.currentMeeting?.speakers.length || 0;
    const speaker: Speaker = {
      id: generateId(),
      label,
      color: SPEAKER_COLORS[colorIndex % SPEAKER_COLORS.length],
    };
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          speakers: [...state.currentMeeting.speakers, speaker],
          updatedAt: new Date().toISOString(),
        },
      };
    });
    return speaker;
  },

  updateSpeaker: (id, updates) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          speakers: state.currentMeeting.speakers.map((s) =>
            s.id === id ? { ...s, ...updates } : s
          ),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  removeSpeaker: (id) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          speakers: state.currentMeeting.speakers.filter((s) => s.id !== id),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  addSegment: (segment) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          segments: [...state.currentMeeting.segments, segment],
          updatedAt: new Date().toISOString(),
        },
        interimText: '',
      };
    });
  },

  updateSegment: (id, updates) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          segments: state.currentMeeting.segments.map((s) =>
            s.id === id
              ? {
                  ...s,
                  ...updates,
                  isEdited: updates.text !== undefined && updates.text !== s.text,
                  originalText:
                    updates.text !== undefined && updates.text !== s.text && !s.originalText
                      ? s.text
                      : s.originalText,
                }
              : s
          ),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  deleteSegment: (id) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          segments: state.currentMeeting.segments.filter((s) => s.id !== id),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  setInterimText: (interimText) => set({ interimText }),

  setRecordingStatus: (recordingStatus) => set({ recordingStatus }),

  startRecording: () => {
    const now = Date.now();
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        recordingStatus: 'recording',
        recordingStartTime: now,
        currentMeeting: {
          ...state.currentMeeting,
          startedAt: new Date(now).toISOString(),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  stopRecording: () => {
    const now = Date.now();
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        recordingStatus: 'idle',
        recordingStartTime: null,
        currentMeeting: {
          ...state.currentMeeting,
          endedAt: new Date(now).toISOString(),
          duration: state.elapsedTime,
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  pauseRecording: () => set({ recordingStatus: 'paused' }),

  resumeRecording: () => set({ recordingStatus: 'recording' }),

  updateElapsedTime: (elapsedTime) => set({ elapsedTime }),

  setAnalysis: (analysis) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          analysis,
          isAnalyzed: true,
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  clearAnalysis: () => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          analysis: undefined,
          isAnalyzed: false,
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  toggleAutoScroll: () =>
    set((state) => ({ autoScrollEnabled: !state.autoScrollEnabled })),

  setSelectedSpeaker: (selectedSpeakerId) => set({ selectedSpeakerId }),

  addTag: (tag) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      if (state.currentMeeting.tags.includes(tag)) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          tags: [...state.currentMeeting.tags, tag],
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  removeTag: (tag) => {
    set((state) => {
      if (!state.currentMeeting) return state;
      return {
        currentMeeting: {
          ...state.currentMeeting,
          tags: state.currentMeeting.tags.filter((t) => t !== tag),
          updatedAt: new Date().toISOString(),
        },
      };
    });
  },

  closeMeeting: () =>
    set({
      currentMeeting: null,
      recordingStatus: 'idle',
      recordingStartTime: null,
      elapsedTime: 0,
      interimText: '',
    }),

  setRecentMeetings: (recentMeetings) => set({ recentMeetings }),

  addToRecentMeetings: (meeting) => {
    set((state) => {
      const filtered = state.recentMeetings.filter((m) => m.id !== meeting.id);
      return {
        recentMeetings: [meeting, ...filtered].slice(0, 10),
      };
    });
  },
}));

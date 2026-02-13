export const SCHEMA_VERSION = 1;

export type Priority = 'low' | 'medium' | 'high' | 'critical';
export type TaskStatus = 'pending' | 'in_progress' | 'completed' | 'cancelled';
export type TranscriptionMode = 'web_speech' | 'cortex_stt';
export type ConnectionStatus = 'connected' | 'connecting' | 'degraded' | 'offline';
export type RecordingStatus = 'idle' | 'recording' | 'paused' | 'processing';
export type SentimentType = 'positive' | 'neutral' | 'negative' | 'mixed';

export interface Participant {
  id: string;
  name: string;
  email?: string;
  role?: string;
}

export interface Speaker {
  id: string;
  label: string;
  color: string;
  participantId?: string;
}

export interface TranscriptSegment {
  id: string;
  startTime: number;
  endTime: number;
  speakerId: string;
  speakerLabel: string;
  text: string;
  confidence?: number;
  source: TranscriptionMode;
  isEdited: boolean;
  originalText?: string;
}

export interface MeetingSettings {
  transcriptionMode: TranscriptionMode;
  autoSaveInterval: number;
  enableAutoScroll: boolean;
  showTimestamps: boolean;
  showConfidence: boolean;
  language: string;
}

export interface MeetingSession {
  id: string;
  title: string;
  description?: string;
  createdAt: string;
  updatedAt: string;
  startedAt?: string;
  endedAt?: string;
  duration: number;
  participants: Participant[];
  speakers: Speaker[];
  segments: TranscriptSegment[];
  settings: MeetingSettings;
  tags: string[];
  templateId?: string;
  isAnalyzed: boolean;
  analysis?: MeetingAnalysis;
  schemaVersion: number;
}

export interface ActionItem {
  id: string;
  text: string;
  assignee?: string;
  dueDate?: string;
  priority: Priority;
  status: TaskStatus;
  sourceSegmentIds: string[];
  meetingId: string;
  createdAt: string;
  updatedAt: string;
  notes?: string;
}

export interface Decision {
  id: string;
  text: string;
  context?: string;
  madeBy?: string;
  sourceSegmentIds: string[];
}

export interface KeyPoint {
  id: string;
  text: string;
  category?: string;
  sourceSegmentIds: string[];
}

export interface Risk {
  id: string;
  text: string;
  severity: Priority;
  mitigation?: string;
  sourceSegmentIds: string[];
}

export interface FollowUp {
  id: string;
  text: string;
  assignee?: string;
  deadline?: string;
}

export interface Topic {
  id: string;
  name: string;
  segmentIds: string[];
  duration: number;
}

export interface MeetingAnalysis {
  id: string;
  meetingId: string;
  summary: string;
  decisions: Decision[];
  actionItems: ActionItem[];
  keyPoints: KeyPoint[];
  sentiment: SentimentType;
  sentimentScore: number;
  risks: Risk[];
  topics: Topic[];
  followUps: FollowUp[];
  nextAgendaSuggestions: string[];
  generatedAt: string;
  modelUsed?: string;
}

export interface MemoryCommitPayload {
  meetingId: string;
  title: string;
  summary: string;
  decisions: Decision[];
  actionItems: ActionItem[];
  keyPoints: KeyPoint[];
  tags: string[];
  redactionsApplied: boolean;
  timestamp: string;
  participants: string[];
}

export interface KnowledgeIngestPayload {
  meetingId: string;
  title: string;
  participants: Participant[];
  fullText: string;
  segments: TranscriptSegment[];
  metadata: Record<string, unknown>;
  tags: string[];
  timestamp: string;
}

export interface SearchResult {
  id: string;
  type: 'meeting' | 'memory' | 'knowledge';
  title: string;
  snippet: string;
  relevanceScore: number;
  metadata: Record<string, unknown>;
  source: string;
  timestamp: string;
}

export interface SearchResults {
  query: string;
  results: SearchResult[];
  totalCount: number;
  sources: {
    local: number;
    cortexMemory: number;
    cortexKnowledge: number;
  };
  searchDuration: number;
}

export interface CommitResult {
  success: boolean;
  memoryId?: string;
  message: string;
  timestamp: string;
}

export interface IngestResult {
  success: boolean;
  knowledgeId?: string;
  message: string;
  chunksCreated: number;
  timestamp: string;
}

export interface TranscriptChunk {
  text: string;
  isFinal: boolean;
  confidence: number;
  startTime: number;
  endTime: number;
}

export interface MeetingTemplate {
  id: string;
  name: string;
  description: string;
  defaultDuration: number;
  suggestedAgenda: string[];
  defaultTags: string[];
}

export interface AppSettings {
  theme: 'light' | 'dark' | 'system';
  cortexUrl: string;
  transcriptionMode: TranscriptionMode;
  autoSaveEnabled: boolean;
  autoSaveInterval: number;
  defaultLanguage: string;
  keyboardShortcutsEnabled: boolean;
  redactionPatterns: RedactionPattern[];
  notificationsEnabled: boolean;
}

export interface RedactionPattern {
  id: string;
  name: string;
  pattern: string;
  replacement: string;
  enabled: boolean;
}

export const DEFAULT_SETTINGS: AppSettings = {
  theme: 'system',
  cortexUrl: 'http://localhost:8080',
  transcriptionMode: 'web_speech',
  autoSaveEnabled: true,
  autoSaveInterval: 30000,
  defaultLanguage: 'en-US',
  keyboardShortcutsEnabled: true,
  redactionPatterns: [
    {
      id: 'email',
      name: 'Email Addresses',
      pattern: '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}',
      replacement: '[EMAIL REDACTED]',
      enabled: true,
    },
    {
      id: 'phone',
      name: 'Phone Numbers',
      pattern: '(\\+?1?[-.]?)?\\(?\\d{3}\\)?[-.]?\\d{3}[-.]?\\d{4}',
      replacement: '[PHONE REDACTED]',
      enabled: true,
    },
    {
      id: 'ssn',
      name: 'Social Security Numbers',
      pattern: '\\d{3}-\\d{2}-\\d{4}',
      replacement: '[SSN REDACTED]',
      enabled: true,
    },
  ],
  notificationsEnabled: true,
};

export const SPEAKER_COLORS = [
  '#3B82F6',
  '#10B981',
  '#F59E0B',
  '#EF4444',
  '#8B5CF6',
  '#EC4899',
  '#06B6D4',
  '#F97316',
];

export function createDefaultMeetingSession(id: string): MeetingSession {
  return {
    id,
    title: 'Untitled Meeting',
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    duration: 0,
    participants: [],
    speakers: [
      { id: 'speaker-1', label: 'Speaker 1', color: SPEAKER_COLORS[0] },
    ],
    segments: [],
    settings: {
      transcriptionMode: 'web_speech',
      autoSaveInterval: 30000,
      enableAutoScroll: true,
      showTimestamps: true,
      showConfidence: false,
      language: 'en-US',
    },
    tags: [],
    isAnalyzed: false,
    schemaVersion: SCHEMA_VERSION,
  };
}

export function generateId(): string {
  return `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`;
}

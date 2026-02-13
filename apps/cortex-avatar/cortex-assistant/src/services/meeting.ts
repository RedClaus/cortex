import { openDB, DBSchema, IDBPDatabase } from 'idb';
import type { MeetingSession } from '@/models';
import { SCHEMA_VERSION } from '@/models';

interface MeetingDB extends DBSchema {
  meetings: {
    key: string;
    value: MeetingSession;
    indexes: {
      'by-date': string;
      'by-title': string;
      'by-analyzed': number;
    };
  };
  metadata: {
    key: string;
    value: {
      key: string;
      value: unknown;
    };
  };
}

const DB_NAME = 'cortex-assistant-db';
const DB_VERSION = 1;

let dbInstance: IDBPDatabase<MeetingDB> | null = null;

async function getDB(): Promise<IDBPDatabase<MeetingDB>> {
  if (dbInstance) return dbInstance;

  dbInstance = await openDB<MeetingDB>(DB_NAME, DB_VERSION, {
    upgrade(db) {
      if (!db.objectStoreNames.contains('meetings')) {
        const meetingsStore = db.createObjectStore('meetings', { keyPath: 'id' });
        meetingsStore.createIndex('by-date', 'createdAt');
        meetingsStore.createIndex('by-title', 'title');
        meetingsStore.createIndex('by-analyzed', 'isAnalyzed');
      }

      if (!db.objectStoreNames.contains('metadata')) {
        db.createObjectStore('metadata', { keyPath: 'key' });
      }
    },
  });

  return dbInstance;
}

export async function saveMeeting(meeting: MeetingSession): Promise<void> {
  const db = await getDB();
  const updatedMeeting = {
    ...meeting,
    updatedAt: new Date().toISOString(),
    schemaVersion: SCHEMA_VERSION,
  };
  await db.put('meetings', updatedMeeting);
}

export async function getMeeting(id: string): Promise<MeetingSession | undefined> {
  const db = await getDB();
  return db.get('meetings', id);
}

export async function deleteMeeting(id: string): Promise<void> {
  const db = await getDB();
  await db.delete('meetings', id);
}

export async function getAllMeetings(): Promise<MeetingSession[]> {
  const db = await getDB();
  return db.getAll('meetings');
}

export async function getRecentMeetings(limit: number = 10): Promise<MeetingSession[]> {
  const db = await getDB();
  const tx = db.transaction('meetings', 'readonly');
  const index = tx.store.index('by-date');
  const meetings: MeetingSession[] = [];

  let cursor = await index.openCursor(null, 'prev');
  while (cursor && meetings.length < limit) {
    meetings.push(cursor.value);
    cursor = await cursor.continue();
  }

  return meetings;
}

export async function searchMeetings(query: string): Promise<MeetingSession[]> {
  const db = await getDB();
  const allMeetings = await db.getAll('meetings');
  const lowerQuery = query.toLowerCase();

  return allMeetings.filter((meeting) => {
    if (meeting.title.toLowerCase().includes(lowerQuery)) return true;
    if (meeting.description?.toLowerCase().includes(lowerQuery)) return true;
    if (meeting.tags.some((tag) => tag.toLowerCase().includes(lowerQuery))) return true;
    if (
      meeting.segments.some((seg) => seg.text.toLowerCase().includes(lowerQuery))
    )
      return true;
    return false;
  });
}

export async function getMeetingsByDateRange(
  startDate: Date,
  endDate: Date
): Promise<MeetingSession[]> {
  const db = await getDB();
  const tx = db.transaction('meetings', 'readonly');
  const index = tx.store.index('by-date');
  const meetings: MeetingSession[] = [];

  const range = IDBKeyRange.bound(
    startDate.toISOString(),
    endDate.toISOString()
  );

  let cursor = await index.openCursor(range);
  while (cursor) {
    meetings.push(cursor.value);
    cursor = await cursor.continue();
  }

  return meetings;
}

export async function getUnanalyzedMeetings(): Promise<MeetingSession[]> {
  const db = await getDB();
  const tx = db.transaction('meetings', 'readonly');
  const index = tx.store.index('by-analyzed');
  const meetings: MeetingSession[] = [];

  let cursor = await index.openCursor(IDBKeyRange.only(0));
  while (cursor) {
    meetings.push(cursor.value);
    cursor = await cursor.continue();
  }

  return meetings;
}

export async function getMeetingStats(): Promise<{
  total: number;
  analyzed: number;
  totalDuration: number;
  averageDuration: number;
  meetingsThisWeek: number;
  meetingsThisMonth: number;
}> {
  const meetings = await getAllMeetings();
  const now = new Date();
  const weekAgo = new Date(now.getTime() - 7 * 24 * 60 * 60 * 1000);
  const monthAgo = new Date(now.getTime() - 30 * 24 * 60 * 60 * 1000);

  const totalDuration = meetings.reduce((sum, m) => sum + m.duration, 0);

  return {
    total: meetings.length,
    analyzed: meetings.filter((m) => m.isAnalyzed).length,
    totalDuration,
    averageDuration: meetings.length > 0 ? totalDuration / meetings.length : 0,
    meetingsThisWeek: meetings.filter(
      (m) => new Date(m.createdAt) >= weekAgo
    ).length,
    meetingsThisMonth: meetings.filter(
      (m) => new Date(m.createdAt) >= monthAgo
    ).length,
  };
}

const AUTOSAVE_KEY = 'autosave-meeting';

export async function setAutoSaveMeeting(meeting: MeetingSession): Promise<void> {
  const db = await getDB();
  await db.put('metadata', {
    key: AUTOSAVE_KEY,
    value: meeting,
  });
}

export async function getAutoSaveMeeting(): Promise<MeetingSession | null> {
  const db = await getDB();
  const result = await db.get('metadata', AUTOSAVE_KEY);
  return result?.value as MeetingSession | null;
}

export async function clearAutoSaveMeeting(): Promise<void> {
  const db = await getDB();
  await db.delete('metadata', AUTOSAVE_KEY);
}

export async function hasAutoSaveMeeting(): Promise<boolean> {
  const db = await getDB();
  const result = await db.get('metadata', AUTOSAVE_KEY);
  return result !== undefined;
}

export async function exportMeetingAsJSON(meeting: MeetingSession): Promise<string> {
  return JSON.stringify(meeting, null, 2);
}

export async function importMeetingFromJSON(json: string): Promise<MeetingSession> {
  const meeting = JSON.parse(json) as MeetingSession;

  if (!meeting.id || !meeting.title || !Array.isArray(meeting.segments)) {
    throw new Error('Invalid meeting format');
  }

  if (meeting.schemaVersion && meeting.schemaVersion > SCHEMA_VERSION) {
    console.warn('Meeting was created with a newer schema version');
  }

  await saveMeeting(meeting);
  return meeting;
}

export async function duplicateMeeting(id: string): Promise<MeetingSession | null> {
  const original = await getMeeting(id);
  if (!original) return null;

  const duplicate: MeetingSession = {
    ...original,
    id: `${Date.now()}-${Math.random().toString(36).substr(2, 9)}`,
    title: `${original.title} (Copy)`,
    createdAt: new Date().toISOString(),
    updatedAt: new Date().toISOString(),
    isAnalyzed: false,
    analysis: undefined,
  };

  await saveMeeting(duplicate);
  return duplicate;
}

export async function clearAllMeetings(): Promise<void> {
  const db = await getDB();
  await db.clear('meetings');
}

export function closeDB(): void {
  if (dbInstance) {
    dbInstance.close();
    dbInstance = null;
  }
}

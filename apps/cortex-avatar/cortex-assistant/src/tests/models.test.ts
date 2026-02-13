import { describe, it, expect } from 'vitest';
import {
  generateId,
  createDefaultMeetingSession,
  SCHEMA_VERSION,
  SPEAKER_COLORS,
} from '@/models';

describe('Models', () => {
  describe('generateId', () => {
    it('should generate unique IDs', () => {
      const id1 = generateId();
      const id2 = generateId();
      expect(id1).not.toBe(id2);
    });

    it('should include timestamp in ID', () => {
      const before = Date.now();
      const id = generateId();
      const after = Date.now();
      const timestamp = parseInt(id.split('-')[0]);
      expect(timestamp).toBeGreaterThanOrEqual(before);
      expect(timestamp).toBeLessThanOrEqual(after);
    });
  });

  describe('createDefaultMeetingSession', () => {
    it('should create a meeting with default values', () => {
      const meeting = createDefaultMeetingSession('test-123');

      expect(meeting.id).toBe('test-123');
      expect(meeting.title).toBe('Untitled Meeting');
      expect(meeting.segments).toHaveLength(0);
      expect(meeting.participants).toHaveLength(0);
      expect(meeting.speakers).toHaveLength(1);
      expect(meeting.isAnalyzed).toBe(false);
      expect(meeting.schemaVersion).toBe(SCHEMA_VERSION);
    });

    it('should have a default speaker', () => {
      const meeting = createDefaultMeetingSession('test');
      expect(meeting.speakers[0].label).toBe('Speaker 1');
      expect(meeting.speakers[0].color).toBe(SPEAKER_COLORS[0]);
    });

    it('should set default settings', () => {
      const meeting = createDefaultMeetingSession('test');
      expect(meeting.settings.transcriptionMode).toBe('web_speech');
      expect(meeting.settings.enableAutoScroll).toBe(true);
    });
  });

  describe('SPEAKER_COLORS', () => {
    it('should have 8 colors', () => {
      expect(SPEAKER_COLORS).toHaveLength(8);
    });

    it('should be valid hex colors', () => {
      SPEAKER_COLORS.forEach((color) => {
        expect(color).toMatch(/^#[0-9A-F]{6}$/i);
      });
    });
  });
});

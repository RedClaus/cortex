import { describe, it, expect } from 'vitest';
import {
  formatDuration,
  formatTime,
  truncateText,
  pluralize,
} from '@/utils/format';
import { applyRedactions, detectSensitiveContent } from '@/utils/redaction';
import type { RedactionPattern } from '@/models';

describe('Format Utils', () => {
  describe('formatDuration', () => {
    it('should format seconds correctly', () => {
      expect(formatDuration(30000)).toBe('0:30');
      expect(formatDuration(90000)).toBe('1:30');
    });

    it('should format hours correctly', () => {
      expect(formatDuration(3600000)).toBe('1:00:00');
      expect(formatDuration(3660000)).toBe('1:01:00');
    });
  });

  describe('formatTime', () => {
    it('should format time correctly', () => {
      expect(formatTime(0)).toBe('0:00');
      expect(formatTime(60000)).toBe('1:00');
      expect(formatTime(125000)).toBe('2:05');
    });
  });

  describe('truncateText', () => {
    it('should not truncate short text', () => {
      expect(truncateText('Hello', 10)).toBe('Hello');
    });

    it('should truncate long text with ellipsis', () => {
      expect(truncateText('Hello World!', 8)).toBe('Hello...');
    });
  });

  describe('pluralize', () => {
    it('should return singular for 1', () => {
      expect(pluralize(1, 'item')).toBe('item');
    });

    it('should return plural for other numbers', () => {
      expect(pluralize(0, 'item')).toBe('items');
      expect(pluralize(5, 'item')).toBe('items');
    });

    it('should use custom plural', () => {
      expect(pluralize(2, 'person', 'people')).toBe('people');
    });
  });
});

describe('Redaction Utils', () => {
  const testPatterns: RedactionPattern[] = [
    {
      id: 'email',
      name: 'Email',
      pattern: '[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}',
      replacement: '[EMAIL]',
      enabled: true,
    },
    {
      id: 'phone',
      name: 'Phone',
      pattern: '\\d{3}-\\d{3}-\\d{4}',
      replacement: '[PHONE]',
      enabled: true,
    },
  ];

  describe('applyRedactions', () => {
    it('should redact emails', () => {
      const result = applyRedactions('Contact me at test@example.com', testPatterns);
      expect(result.text).toBe('Contact me at [EMAIL]');
      expect(result.count).toBe(1);
    });

    it('should redact phone numbers', () => {
      const result = applyRedactions('Call 123-456-7890', testPatterns);
      expect(result.text).toBe('Call [PHONE]');
      expect(result.count).toBe(1);
    });

    it('should redact multiple patterns', () => {
      const result = applyRedactions(
        'Email: test@test.com, Phone: 555-123-4567',
        testPatterns
      );
      expect(result.text).toBe('Email: [EMAIL], Phone: [PHONE]');
      expect(result.count).toBe(2);
    });

    it('should respect disabled patterns', () => {
      const patterns = [{ ...testPatterns[0], enabled: false }];
      const result = applyRedactions('test@example.com', patterns);
      expect(result.text).toBe('test@example.com');
      expect(result.count).toBe(0);
    });
  });

  describe('detectSensitiveContent', () => {
    it('should detect sensitive content', () => {
      const detections = detectSensitiveContent(
        'Contact: test@example.com or 123-456-7890',
        testPatterns
      );
      expect(detections).toHaveLength(2);
    });

    it('should return unique matches', () => {
      const detections = detectSensitiveContent(
        'test@a.com and test@a.com',
        testPatterns
      );
      expect(detections[0].matches).toHaveLength(1);
    });
  });
});

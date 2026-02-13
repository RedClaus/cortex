import type { RedactionPattern } from '@/models';

export function applyRedactions(
  text: string,
  patterns: RedactionPattern[]
): { text: string; count: number } {
  let redactedText = text;
  let totalCount = 0;

  for (const pattern of patterns) {
    if (!pattern.enabled) continue;

    try {
      const regex = new RegExp(pattern.pattern, 'gi');
      const matches = redactedText.match(regex);
      if (matches) {
        totalCount += matches.length;
        redactedText = redactedText.replace(regex, pattern.replacement);
      }
    } catch {
      console.warn(`Invalid redaction pattern: ${pattern.name}`);
    }
  }

  return { text: redactedText, count: totalCount };
}

export function detectSensitiveContent(
  text: string,
  patterns: RedactionPattern[]
): { pattern: string; matches: string[] }[] {
  const detections: { pattern: string; matches: string[] }[] = [];

  for (const pattern of patterns) {
    if (!pattern.enabled) continue;

    try {
      const regex = new RegExp(pattern.pattern, 'gi');
      const matches = text.match(regex);
      if (matches && matches.length > 0) {
        detections.push({
          pattern: pattern.name,
          matches: [...new Set(matches)],
        });
      }
    } catch {
      continue;
    }
  }

  return detections;
}

export const DEFAULT_REDACTION_PATTERNS: RedactionPattern[] = [
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
  {
    id: 'credit_card',
    name: 'Credit Card Numbers',
    pattern: '\\d{4}[- ]?\\d{4}[- ]?\\d{4}[- ]?\\d{4}',
    replacement: '[CARD REDACTED]',
    enabled: true,
  },
  {
    id: 'ip_address',
    name: 'IP Addresses',
    pattern: '\\b(?:\\d{1,3}\\.){3}\\d{1,3}\\b',
    replacement: '[IP REDACTED]',
    enabled: false,
  },
];

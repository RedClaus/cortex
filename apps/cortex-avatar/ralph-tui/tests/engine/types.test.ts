/**
 * ABOUTME: Tests for engine type utilities and converters.
 * Tests type conversion functions and state utilities.
 */

import { describe, test, expect } from 'bun:test';
import { toEngineSubagentState } from '../../src/engine/types.js';
import type { SubagentState as ParserSubagentState } from '../../src/plugins/agents/tracing/types.js';

describe('engine types', () => {
  describe('toEngineSubagentState', () => {
    test('converts parser state to engine state', () => {
      const parserState: ParserSubagentState = {
        id: 'subagent-001',
        agentType: 'Bash',
        description: 'Running npm install',
        status: 'running',
        spawnedAt: '2024-01-15T10:00:00.000Z',
        childIds: [],
      };

      const result = toEngineSubagentState(parserState, 1);

      expect(result.id).toBe('subagent-001');
      expect(result.type).toBe('Bash');
      expect(result.description).toBe('Running npm install');
      expect(result.status).toBe('running');
      expect(result.startedAt).toBe('2024-01-15T10:00:00.000Z');
      expect(result.depth).toBe(1);
      expect(result.children).toEqual([]);
    });

    test('converts completed state with duration', () => {
      const parserState: ParserSubagentState = {
        id: 'subagent-002',
        agentType: 'Explore',
        description: 'Searching files',
        status: 'completed',
        spawnedAt: '2024-01-15T10:00:00.000Z',
        endedAt: '2024-01-15T10:00:05.000Z',
        durationMs: 5000,
        childIds: ['subagent-003'],
      };

      const result = toEngineSubagentState(parserState, 2);

      expect(result.status).toBe('completed');
      expect(result.completedAt).toBe('2024-01-15T10:00:05.000Z');
      expect(result.durationMs).toBe(5000);
      expect(result.depth).toBe(2);
      expect(result.children).toEqual(['subagent-003']);
    });

    test('handles error status', () => {
      const parserState: ParserSubagentState = {
        id: 'subagent-003',
        agentType: 'Bash',
        description: 'Failed command',
        status: 'error',
        spawnedAt: '2024-01-15T10:00:00.000Z',
        endedAt: '2024-01-15T10:00:02.000Z',
        durationMs: 2000,
        childIds: [],
      };

      const result = toEngineSubagentState(parserState, 1);

      expect(result.status).toBe('error');
      expect(result.completedAt).toBe('2024-01-15T10:00:02.000Z');
    });

    test('preserves parent ID', () => {
      const parserState: ParserSubagentState = {
        id: 'subagent-004',
        agentType: 'Read',
        description: 'Reading file.ts',
        status: 'running',
        spawnedAt: '2024-01-15T10:00:00.000Z',
        parentId: 'subagent-001',
        childIds: [],
      };

      const result = toEngineSubagentState(parserState, 3);

      expect(result.parentId).toBe('subagent-001');
      expect(result.depth).toBe(3);
    });

    test('creates independent copy of children array', () => {
      const childIds = ['child-001', 'child-002'];
      const parserState: ParserSubagentState = {
        id: 'subagent-005',
        agentType: 'Plan',
        description: 'Planning task',
        status: 'running',
        spawnedAt: '2024-01-15T10:00:00.000Z',
        childIds,
      };

      const result = toEngineSubagentState(parserState, 1);

      // Modify original array
      childIds.push('child-003');

      // Result should not be affected
      expect(result.children).toEqual(['child-001', 'child-002']);
    });
  });
});

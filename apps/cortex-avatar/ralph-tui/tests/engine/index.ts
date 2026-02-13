/**
 * ABOUTME: Engine test module index.
 * Re-exports all engine-related tests for discovery.
 */

// Tests are auto-discovered by bun:test based on *.test.ts pattern
// This file provides documentation of what tests are available

/**
 * Engine test coverage:
 *
 * 1. rate-limit-detector.test.ts
 *    - Tests for RateLimitDetector
 *    - Claude-specific patterns
 *    - OpenCode-specific patterns
 *    - Generic rate limit patterns
 *    - Retry-after extraction
 *    - False positive prevention
 *
 * 2. types.test.ts
 *    - Type conversion utilities
 *    - toEngineSubagentState function
 *
 * 3. execution-engine.test.ts
 *    - State machine transitions (idle → running → paused → stopped)
 *    - Iteration logic (add/remove iterations)
 *    - Error classification
 *    - Event system
 *    - Active agent state management
 *    - Task management
 *
 * 4. integration.test.ts
 *    - SELECT → BUILD → EXECUTE → DETECT cycle
 *    - Error handling strategies (skip, abort, retry)
 *    - Output streaming
 *    - Completion detection (<promise>COMPLETE</promise>)
 *    - Task status management
 */

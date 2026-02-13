/**
 * ABOUTME: Test fixtures index for static test data.
 * Provides paths and loaders for sample configs and PRD files.
 */

import { readFileSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const __dirname = dirname(fileURLToPath(import.meta.url));

/**
 * Path to fixtures directory
 */
export const FIXTURES_DIR = __dirname;

/**
 * Get the absolute path to a fixture file
 */
export function getFixturePath(filename: string): string {
  return join(FIXTURES_DIR, filename);
}

/**
 * Load a JSON fixture file
 */
export function loadJsonFixture<T = unknown>(filename: string): T {
  const content = readFileSync(getFixturePath(filename), 'utf-8');
  return JSON.parse(content) as T;
}

/**
 * Load a text fixture file
 */
export function loadTextFixture(filename: string): string {
  return readFileSync(getFixturePath(filename), 'utf-8');
}

// Exported fixture file paths
export const SAMPLE_PRD_PATH = getFixturePath('sample-prd.json');
export const SAMPLE_CONFIG_PATH = getFixturePath('sample-config.yaml');
export const SAMPLE_PROGRESS_PATH = getFixturePath('sample-progress.md');

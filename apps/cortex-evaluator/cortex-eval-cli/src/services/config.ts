import fs from 'fs-extra';
import path from 'path';

export interface CortexEvalConfig {
  projectId: string;
  projectName: string;
  codebaseId?: string;
  apiBaseUrl?: string;
  createdAt: string;
  updatedAt: string;
}

export const CONFIG_FILE = '.cortex-eval.json';

export async function readConfig(dir: string = process.cwd()): Promise<CortexEvalConfig | null> {
  const configPath = path.join(dir, CONFIG_FILE);
  try {
    if (await fs.pathExists(configPath)) {
      const configContent = await fs.readFile(configPath, 'utf-8');
      return JSON.parse(configContent);
    }
    return null;
  } catch (error) {
    throw new Error(`Failed to read config file: ${(error as Error).message}`);
  }
}

export async function writeConfig(config: CortexEvalConfig, dir: string = process.cwd()): Promise<void> {
  const configPath = path.join(dir, CONFIG_FILE);
  try {
    await fs.writeFile(configPath, JSON.stringify(config, null, 2), 'utf-8');
  } catch (error) {
    throw new Error(`Failed to write config file: ${(error as Error).message}`);
  }
}

export async function updateConfig(updates: Partial<CortexEvalConfig>, dir: string = process.cwd()): Promise<CortexEvalConfig> {
  const config = await readConfig(dir);
  if (!config) {
    throw new Error('No config file found. Run `cortex-eval init` first.');
  }
  const updated = { ...config, ...updates, updatedAt: new Date().toISOString() };
  await writeConfig(updated, dir);
  return updated;
}

export function generateProjectId(): string {
  return `proj_${Date.now()}_${Math.random().toString(36).substring(2, 9)}`;
}

export async function findConfigFile(dir: string = process.cwd()): Promise<string | null> {
  let currentDir = dir;
  while (currentDir !== path.parse(currentDir).root) {
    const configPath = path.join(currentDir, CONFIG_FILE);
    if (await fs.pathExists(configPath)) {
      return currentDir;
    }
    currentDir = path.dirname(currentDir);
  }
  return null;
}

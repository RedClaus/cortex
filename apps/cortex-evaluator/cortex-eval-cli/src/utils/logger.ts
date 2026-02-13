import chalk from 'chalk';
import ora, { Ora } from 'ora';

export interface LoggerOptions {
  silent?: boolean;
  debug?: boolean;
  timestamp?: boolean;
}

const createLogger = (options: LoggerOptions = {}): Logger => {
  const { silent = false, debug = false, timestamp = false } = options;

  const log = (
    color: typeof chalk,
    level: string,
    args: unknown[]
  ) => {
    if (silent) return;
    const time = timestamp ? `\t[${new Date().toISOString()}]` : '';
    console.log(color(`[${level}]${time}`), ...args);
  };

  return {
    debug(...args: unknown[]): void {
      if (!debug) return;
      log(chalk.cyan, 'DEBUG', args);
    },

    info(...args: unknown[]): void {
      log(chalk.blue, 'INFO', args);
    },

    success(...args: unknown[]): void {
      log(chalk.green, 'SUCCESS', args);
    },

    warn(...args: unknown[]): void {
      console.warn(chalk.yellow(`[WARN]${timestamp ? `\t[${new Date().toISOString()}]` : ''}`), ...args);
    },

    error(...args: unknown[]): void {
      console.error(chalk.red(`[ERROR]${timestamp ? `\t[${new Date().toISOString()}]` : ''}`), ...args);
    },

    spinner(text?: string): Ora {
      return ora({ 
        text, 
        color: 'cyan',
        spinner: 'dots',
      });
    },
  };
};

export interface Logger {
  debug(...args: unknown[]): void;
  info(...args: unknown[]): void;
  success(...args: unknown[]): void;
  warn(...args: unknown[]): void;
  error(...args: unknown[]): void;
  spinner(text?: string): Ora;
}

export const logger = createLogger();


export enum SDLCStage {
  WORKSHOP = 'WORKSHOP',
  DEVELOPMENT = 'DEVELOPMENT',
  TEST = 'TEST',
  PRODUCTION = 'PRODUCTION',
  ARCHIVE = 'ARCHIVE',
  DOCTOR = 'DOCTOR'
}

export enum AICLI {
  GEMINI = 'Gemini CLI',
  CLAUDE = 'Claude Code CLI',
  OPENCODE = 'OpenCode CLI',
  DEEPSEEK = 'DeepSeek CLI'
}

export interface ProjectFile {
  name: string;
  path: string;
  content: string;
  status: 'clean' | 'modified' | 'error';
}

export interface DiagnosticReport {
  id: string;
  timestamp: string;
  findings: string[];
  severity: 'low' | 'medium' | 'high' | 'critical';
  summary: string;
  suggestedFix: string;
}

export interface BrainSnippet {
  id: string;
  title: string;
  code: string;
  language: string;
}

export interface BrainSession {
  sessionId: string;
  authStatus: 'authenticated' | 'anonymous';
  runbacks: { id: string; command: string; timestamp: number }[];
  codeSnippets: BrainSnippet[];
  stats: {
    linesScanned: number;
    bugsFixed: number;
    promptsSent: number;
  };
}

export interface TerminalMessage {
  role: 'user' | 'ai';
  text: string;
  timestamp: number;
  cli: AICLI;
}

export interface Blueprint {
  id: string;
  title: string;
  description: string;
  tags: string[];
}

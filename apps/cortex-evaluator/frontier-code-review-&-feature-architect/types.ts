
export type Provider = 'gemini' | 'openai' | 'anthropic' | 'groq' | 'grok' | 'z-ai';

export interface CodeFile {
  name: string;
  content: string;
  path: string;
  type: string;
}

export interface ReviewInput {
  type: 'pdf' | 'repo' | 'snippet';
  name: string;
  content: string;
  fileData?: {
    data: string; // base64 encoded
    mimeType: string;
  };
  metadata?: any;
}

export interface SystemDocumentation {
  overview: string;
  architecture: string;
  keyModules: { name: string; responsibility: string }[];
  techStack: string[];
}

export interface AnalysisResult {
  valueScore: number;
  executiveSummary: string;
  technicalFeasibility: string;
  gapAnalysis: string;
  suggestedCR: string;
}

export interface IndexingStatus {
  isIndexing: boolean;
  totalFiles: number;
  processedFiles: number;
  currentFile: string;
}

export interface AppState {
  codebase: CodeFile[];
  inputs: ReviewInput[];
  selectedProvider: Provider;
  isAnalyzing: boolean;
  results: AnalysisResult | null;
  indexingStatus: IndexingStatus;
  systemDocumentation: SystemDocumentation | null;
}

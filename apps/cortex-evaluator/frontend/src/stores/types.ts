export interface CodeFile {
  id: string;
  name: string;
  path: string;
  content: string;
  language: string;
  size: number;
  lastModified: Date;
}

export interface SystemDocumentation {
  id: string;
  title: string;
  content: string;
  sections: DocumentationSection[];
  lastUpdated: Date;
}

export interface DocumentationSection {
  id: string;
  title: string;
  content: string;
  order: number;
}

export interface Workspace {
  id: string;
  name: string;
  description: string;
  codebaseId?: string;
  createdAt: Date;
  updatedAt: Date;
  settings: WorkspaceSettings;
}

export interface WorkspaceSettings {
  autoSave: boolean;
  theme: 'light' | 'dark';
  defaultProvider: string;
}

export interface Evaluation {
  id: string;
  workspaceId: string;
  name: string;
  criteria: EvaluationCriteria[];
  status: 'pending' | 'running' | 'completed' | 'failed';
  createdAt: Date;
  completedAt?: Date;
}

export interface EvaluationCriteria {
  id: string;
  category: string;
  description: string;
  weight: number;
  metrics: EvaluationMetric[];
}

export interface EvaluationMetric {
  id: string;
  name: string;
  target: number;
  actual?: number;
  unit?: string;
}

export interface EvaluationResult {
  id: string;
  evaluationId: string;
  criteriaId: string;
  metricId: string;
  score: number;
  notes: string;
  feedback: string;
  timestamp: Date;
}

export interface Node {
  id: string;
  type: 'idea' | 'question' | 'answer' | 'reference' | 'note';
  content: string;
  position: { x: number; y: number };
  color?: string;
  connections: string[];
  metadata: Record<string, unknown>;
}

export interface Edge {
  id: string;
  source: string;
  target: string;
  label?: string;
  type: 'directed' | 'undirected';
  weight?: number;
}

export interface BrainstormSession {
  id: string;
  name: string;
  nodes: Node[];
  edges: Edge[];
  createdAt: Date;
  updatedAt: Date;
}

export interface Viewport {
  x: number;
  y: number;
  zoom: number;
}

export interface ApiSettings {
  openaiApiKey: string;
  anthropicApiKey: string;
  geminiApiKey: string;
  groqApiKey: string;
  ollamaBaseUrl: string;
}

export interface AppStore {
  userId: string | null;
  isDarkMode: boolean;
  selectedProvider: string;
  apiSettings: ApiSettings;
  isSettingsOpen: boolean;
  setUserId: (userId: string | null) => void;
  toggleTheme: () => void;
  setSelectedProvider: (provider: string) => void;
  setApiSettings: (settings: Partial<ApiSettings>) => void;
  setSettingsOpen: (open: boolean) => void;
}

export interface CodebaseStore {
  files: CodeFile[];
  systemDoc: SystemDocumentation | null;
  setFiles: (files: CodeFile[]) => void;
  addFile: (file: CodeFile) => void;
  removeFile: (fileId: string) => void;
  setSystemDoc: (doc: SystemDocumentation | null) => void;
  scanDirectory: (directoryHandle: FileSystemDirectoryHandle) => Promise<CodeFile[]>;
  fetchGitHubRepo: (url: string) => Promise<CodeFile[]>;
}

export interface SessionStore {
  workspaces: Workspace[];
  currentWorkspaceId: string | null;
  createWorkspace: (name: string, description: string) => Workspace;
  loadWorkspace: (workspaceId: string) => void;
  deleteWorkspace: (workspaceId: string) => void;
  setCurrentWorkspace: (workspaceId: string | null) => void;
}

export interface BrainstormStore {
  nodes: Node[];
  edges: Edge[];
  currentSession: BrainstormSession | null;
  viewport: Viewport;
  setNodes: (nodes: Node[]) => void;
  addNode: (node: Node) => void;
  updateNode: (nodeId: string, updates: Partial<Node>) => void;
  deleteNode: (nodeId: string) => void;
  setEdges: (edges: Edge[]) => void;
  addEdge: (edge: Edge) => void;
  deleteEdge: (edgeId: string) => void;
  setViewport: (viewport: Partial<Viewport>) => void;
  createSession: (name: string) => BrainstormSession;
  loadSession: (sessionId: string) => void;
  saveSession: () => void;
}

export interface EvaluationStore {
  evaluations: Evaluation[];
  currentEvaluation: Evaluation | null;
  results: EvaluationResult[];
  addEvaluation: (evaluation: Evaluation) => void;
  setCurrentEvaluation: (evaluation: Evaluation | null) => void;
  updateEvaluation: (evaluationId: string, updates: Partial<Evaluation>) => void;
  deleteEvaluation: (evaluationId: string) => void;
  addResult: (result: EvaluationResult) => void;
  getResult: (evaluationId: string) => EvaluationResult[];
  fetchEvaluations: (workspaceId: string) => Promise<void>;
  runEvaluation: (evaluationId: string) => Promise<void>;
}

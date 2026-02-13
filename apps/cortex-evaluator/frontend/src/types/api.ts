export interface CodeFile {
  id: string
  name: string
  path: string
  content: string
  language: string
  size: number
  lastModified: Date
}

export interface SystemDocumentation {
  id: string
  title: string
  content: string
  sections: DocumentationSection[]
  lastUpdated: Date
  overview: string
  architecture: string
  keyModules: { name: string; responsibility: string }[]
  techStack: string[]
}

export interface DocumentationSection {
  id: string
  title: string
  content: string
  order: number
}

export interface Evaluation {
  id: string
  codebaseId: string
  inputType: 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url'
  inputName: string
  inputContent: string
  status: 'pending' | 'running' | 'completed' | 'failed'
  createdAt: Date
  completedAt?: Date
  providerId: string
}

export interface EvaluationResult {
  id: string
  evaluationId: string
  valueScore: number
  executiveSummary: string
  technicalFeasibility: string
  gapAnalysis: string
  suggestedCR: string
  timestamp: Date
}

export interface AnalysisRequest {
  codebaseId: string
  inputType: 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url'
  inputContent?: string
  fileData?: { data: string; mimeType: string }
  providerPreference?: string
  userIntent?: 'strong' | 'local' | 'cheap'
}

export interface ArxivPaper {
  id: string
  title: string
  authors: string[]
  abstract: string
  published: Date
  categories: string[]
  pdfUrl: string
  content?: string
}

export interface BrainstormSession {
  id: string
  projectId: string
  title: string
  nodes: Node[]
  edges: Edge[]
  viewport?: { x: number; y: number; zoom: number }
  createdAt: Date
  updatedAt: Date
}

export interface Node {
  id: string
  type: 'idea' | 'question' | 'answer' | 'reference' | 'note' | 'problem' | 'solution' | 'constraint'
  content: string
  position: { x: number; y: number }
  color?: string
  connections: string[]
  metadata: Record<string, unknown>
}

export interface Edge {
  id: string
  source: string
  target: string
  label?: string
  type: 'directed' | 'undirected' | 'default' | 'floating'
  weight?: number
}

export interface SearchResult {
  id: string
  score: number
  metadata: Record<string, unknown>
}

export interface CodebaseInitRequest {
  type: 'local' | 'github'
  directory?: string
  githubUrl?: string
  name?: string
}

export interface CodebaseInitResponse {
  codebaseId: string
  status: 'pending' | 'indexing' | 'completed' | 'failed'
  message: string
  fileCount: number
}

export interface CodebaseInfo {
  id: string
  name: string
  type: string
  sourceUrl?: string
  metadata: Record<string, unknown>
  createdAt: Date
  fileCount: number
  files: CodeFile[]
}

export interface APIError {
  message: string
  status?: number
  code?: string
}

export interface IndexingProgress {
  isIndexing: boolean
  totalFiles: number
  processedFiles: number
  currentFile: string
  phase: 'scanning' | 'documenting' | 'vectorizing' | 'idle'
}

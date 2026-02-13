import axios, { AxiosInstance, AxiosError } from 'axios';

export interface APIError {
  message: string;
  status?: number;
  code?: string;
}

export interface CodeFile {
  id: string;
  name: string;
  path: string;
  content: string;
  language: string;
  size: number;
  lastModified: Date;
}

export interface CodebaseInitRequest {
  type: 'local' | 'github';
  directory?: string;
  githubUrl?: string;
  name?: string;
}

export interface CodebaseInitResponse {
  codebaseId: string;
  status: 'pending' | 'indexing' | 'completed' | 'failed';
  message: string;
  fileCount: number;
}

export interface CodebaseInfo {
  id: string;
  name: string;
  type: string;
  sourceUrl?: string;
  metadata: Record<string, unknown>;
  createdAt: Date;
  fileCount: number;
  files: CodeFile[];
}

export interface AnalysisRequest {
  codebaseId: string;
  inputType: 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url';
  inputContent?: string;
  fileData?: { data: string; mimeType: string };
  providerPreference?: string;
  userIntent?: 'strong' | 'local' | 'cheap';
}

export interface ArxivPaper {
  id: string;
  title: string;
  authors: string[];
  abstract: string;
  published: Date;
  categories: string[];
  pdfUrl: string;
  content?: string;
}

export interface AnalysisResult {
  id: string;
  evaluationId: string;
  valueScore: number;
  executiveSummary: string;
  technicalFeasibility: string;
  gapAnalysis: string;
  suggestedCR: string;
  providerUsed: string;
  similarEvaluations?: Array<{ id: string; score: number }>;
}

class APIErrorImpl extends Error implements APIError {
  constructor(
    public message: string,
    public status?: number,
    public code?: string
  ) {
    super(message);
    this.name = 'APIError';
  }
}

class APIClient {
  private client: AxiosInstance;
  private baseURL: string;

  constructor(baseURL?: string) {
    this.baseURL = baseURL || 'http://localhost:8000';
    this.client = axios.create({
      baseURL: this.baseURL,
      headers: {
        'Content-Type': 'application/json',
      },
    });
  }

  setBaseURL(baseURL: string) {
    this.baseURL = baseURL;
    this.client.defaults.baseURL = baseURL;
  }

  private async handleRequest<T>(request: Promise<any>): Promise<T> {
    try {
      const response = await request;
      return response.data;
    } catch (error) {
      const err = error as AxiosError;
      if (err) {
        const errorMessage =
          (err.response?.data as any)?.detail ||
          (err.response?.data as any)?.message ||
          err.message;
        throw new APIErrorImpl(
          errorMessage,
          err.response?.status,
          err.code
        );
      }
      throw error;
    }
  }

  async healthCheck(): Promise<{ status: string; version: string; services: Record<string, string> }> {
    return this.handleRequest(this.client.get('/health'));
  }

  async initializeCodebase(request: CodebaseInitRequest): Promise<CodebaseInitResponse> {
    return this.handleRequest(this.client.post('/api/codebases/initialize', request));
  }

  async getCodebase(codebaseId: string): Promise<CodebaseInfo> {
    return this.handleRequest(this.client.get(`/api/codebases/${codebaseId}`));
  }

  async listCodebases(
    projectId?: string,
    limit = 50,
    offset = 0
  ): Promise<{ codebases: CodebaseInfo[]; total: number; limit: number; offset: number }> {
    const params: Record<string, string | number> = { limit, offset };
    if (projectId) {
      params.project_id = projectId;
    }
    return this.handleRequest(this.client.get('/api/codebases/', { params }));
  }

  async deleteCodebase(codebaseId: string): Promise<{ codebaseId: string; status: string; filesDeleted: number }> {
    return this.handleRequest(this.client.delete(`/api/codebases/${codebaseId}`));
  }

  async generateSystemDocumentation(
    codebaseId: string,
    includeTests = false,
    maxFiles = 15
  ): Promise<{
    codebaseId: string;
    documentation: {
      id: string;
      title: string;
      content: string;
      sections: Array<{ id: string; title: string; content: string; order: number }>;
      lastUpdated: Date;
      overview: string;
      architecture: string;
      keyModules: Array<{ name: string; responsibility: string }>;
      techStack: string[];
    };
    fileCount: number;
    generatedAt: string;
  }> {
    return this.handleRequest(
      this.client.post(`/api/codebases/${codebaseId}/generate-docs`, {
        includeTests,
        maxFiles,
      })
    );
  }

  async analyzeEvaluation(request: AnalysisRequest): Promise<AnalysisResult> {
    return this.handleRequest(this.client.post('/api/evaluations/analyze', request));
  }

  async getEvaluation(evaluationId: string): Promise<{
    id: string;
    codebaseId: string;
    inputType: string;
    inputName: string;
    inputContent: string;
    status: string;
    createdAt: Date;
    completedAt?: Date;
    providerId: string;
    result?: AnalysisResult;
  }> {
    return this.handleRequest(this.client.get(`/api/evaluations/${evaluationId}`));
  }

  async searchArxiv(
    query: string,
    maxResults = 10,
    categories?: string[]
  ): Promise<{ query: string; papers: ArxivPaper[]; count: number }> {
    return this.handleRequest(
      this.client.post('/api/arxiv/search', {
        query,
        maxResults,
        categories,
      })
    );
  }

  async getArxivPaper(paperId: string): Promise<ArxivPaper> {
    return this.handleRequest(this.client.post('/api/arxiv/paper', { paperId }));
  }

  async getArxivCategories(): Promise<Record<string, { name: string; description: string }>> {
    return this.handleRequest(this.client.get('/api/arxiv/categories'));
  }

  async findSimilarPapers(query: string, limit = 5): Promise<{
    query: string;
    similarPapers: Array<{ id: string; score: number; metadata: Record<string, unknown> }>;
    count: number;
  }> {
    return this.handleRequest(this.client.post(`/api/arxiv/similarity?limit=${limit}`, { query }));
  }

  async createBrainstormSession(
    projectId: string,
    title: string
  ): Promise<{
    id: string;
    projectId: string;
    title: string;
    nodes: Array<{
      id: string;
      type: string;
      content: string;
      position: { x: number; y: number };
      color?: string;
      connections: string[];
      metadata: Record<string, unknown>;
    }>;
    edges: Array<{
      id: string;
      source: string;
      target: string;
      label?: string;
      type: string;
      weight?: number;
    }>;
    createdAt: Date;
    updatedAt: Date;
  }> {
    return this.handleRequest(this.client.post('/api/sessions/', { projectId, title }));
  }

  async generateBrainstormIdeas(
    topic: string,
    constraints?: string[],
    providerPreference?: string
  ): Promise<{ ideas: unknown[]; providerUsed: string; topic: string }> {
    return this.handleRequest(
      this.client.post('/api/brainstorm/ideas', {
        topic,
        constraints,
        providerPreference,
      })
    );
  }

  async evaluateIdeas(
    ideas: string[],
    criteria?: string[]
  ): Promise<{ ideas: unknown[]; criteria: string[]; topIdea: unknown }> {
    return this.handleRequest(this.client.post('/api/brainstorm/evaluate', { ideas, criteria }));
  }
}

export const apiClient = new APIClient();

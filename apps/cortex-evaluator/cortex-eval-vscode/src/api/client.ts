import axios, { AxiosInstance, AxiosError } from 'axios';

export interface APIError {
  message: string;
  status?: number;
  code?: string;
}

export interface AnalysisRequest {
  codebaseId: string;
  inputType: 'pdf' | 'repo' | 'snippet' | 'arxiv' | 'url';
  inputContent?: string;
  fileData?: { data: string; mimeType: string };
  providerPreference?: string;
  userIntent?: 'strong' | 'local' | 'cheap';
}

export interface AnalysisResult {
  id: string;
  valueScore: number;
  executiveSummary: string;
  technicalFeasibility: string;
  gapAnalysis: string;
  suggestedCR: string;
  providerUsed: string;
  similarEvaluations?: Array<{ id: string; score: number }>;
}

export interface SearchResult {
  id: string;
  score: number;
  metadata?: Record<string, unknown>;
}

export interface IssueRequest {
  platform: string;
  title: string;
  body: string;
  metadata?: Record<string, unknown>;
}

export interface IssueResponse {
  url: string;
  id: string;
  status: string;
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
      timeout: 60000,
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

  async analyzeEvaluation(request: AnalysisRequest): Promise<AnalysisResult> {
    return this.handleRequest(this.client.post('/api/evaluations/analyze', request));
  }

  async searchEvaluations(
    query: string,
    semantic = true,
    filters?: Record<string, unknown>,
    limit = 10
  ): Promise<{
    query: string;
    searchType: string;
    results: SearchResult[];
    count: number;
  }> {
    const params = new URLSearchParams({
      query,
      semantic: semantic.toString(),
      limit: limit.toString()
    });
    return this.handleRequest(this.client.get(`/api/history/search?${params.toString()}`));
  }

  async getEvaluation(evaluationId: string): Promise<{
    id: string;
    project_id: string;
    input_type: string;
    input_name: string;
    input_content: string;
    status: string;
    created_at: string;
    completed_at?: string;
    provider_id: string;
    result?: AnalysisResult;
  }> {
    return this.handleRequest(this.client.get(`/api/evaluations/${evaluationId}`));
  }

  async getSimilarEvaluations(evaluationId: string, limit = 10): Promise<{
    evaluation_id: string;
    similar_evaluations: SearchResult[];
    count: number;
  }> {
    return this.handleRequest(
      this.client.get(`/api/evaluations/${evaluationId}/similar?limit=${limit}`)
    );
  }

  async createIssue(request: IssueRequest): Promise<IssueResponse> {
    return this.handleRequest(this.client.post('/api/integrations/issues', request));
  }

  async getCodebase(codebaseId: string): Promise<{
    id: string;
    name: string;
    type: string;
    source_url?: string;
    metadata: Record<string, unknown>;
    created_at: string;
    file_count: number;
    files: Array<{
      id: string;
      name: string;
      path: string;
      content: string;
      file_type: string;
    }>;
  }> {
    return this.handleRequest(this.client.get(`/api/codebases/${codebaseId}`));
  }

  async listCodebases(
    projectId?: string,
    limit = 50,
    offset = 0
  ): Promise<{
    codebases: Array<{
      id: string;
      name: string;
      type: string;
      created_at: string;
      file_count: number;
    }>;
    total: number;
    limit: number;
    offset: number;
  }> {
    const params: Record<string, string | number> = { limit, offset };
    if (projectId) {
      params.project_id = projectId;
    }
    return this.handleRequest(this.client.get('/api/codebases/', { params }));
  }
}

export const apiClient = new APIClient();

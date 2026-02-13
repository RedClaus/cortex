import type {
  CodeFile,
  SystemDocumentation,
  Evaluation,
  EvaluationResult,
  AnalysisRequest,
  ArxivPaper,
  BrainstormSession,
  Node,
  Edge,
  SearchResult,
  CodebaseInitRequest,
  CodebaseInitResponse,
  CodebaseInfo,
  APIError
} from '../types/api'

class APIErrorImpl extends Error implements APIError {
  constructor(
    public message: string,
    public status?: number,
    public code?: string
  ) {
    super(message)
    this.name = 'APIError'
  }
}

class APIClient {
  private baseURL: string
  private authHeaders: Record<string, string>

  constructor() {
    this.baseURL = (globalThis as any).import?.meta?.env?.VITE_API_URL || 'http://localhost:8000'
    this.authHeaders = {}
  }

  configure(baseURL: string, authHeaders: Record<string, string> = {}) {
    this.baseURL = baseURL
    this.authHeaders = authHeaders
  }

  private async handleResponse<T>(response: Response): Promise<T> {
    if (!response.ok) {
      const errorData = await response.json().catch(() => ({ message: response.statusText }))
      throw new APIErrorImpl(
        errorData.message || errorData.detail || 'An error occurred',
        response.status,
        errorData.code
      )
    }

    const contentType = response.headers.get('content-type')
    if (contentType && contentType.includes('application/json')) {
      return response.json()
    }

    return response.text() as unknown as T
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseURL}${endpoint}`

    const config: RequestInit = {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...this.authHeaders,
        ...options.headers
      }
    }

    try {
      const response = await fetch(url, config)
      return await this.handleResponse<T>(response)
    } catch (error) {
      if (error instanceof APIErrorImpl) {
        throw error
      }
      throw new APIErrorImpl(
        error instanceof Error ? error.message : 'Network error',
        undefined,
        'NETWORK_ERROR'
      )
    }
  }

  private get<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET' })
  }

  private post<T>(endpoint: string, data: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: JSON.stringify(data)
    })
  }

  private put<T>(endpoint: string, data: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: JSON.stringify(data)
    })
  }

  private delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' })
  }

  async healthCheck() {
    return this.get<{ status: string; version: string; services: Record<string, string> }>('/health')
  }

  async initializeCodebase(request: CodebaseInitRequest): Promise<CodebaseInitResponse> {
    return this.post<CodebaseInitResponse>('/api/codebases/initialize', request)
  }

  async getCodebase(codebaseId: string): Promise<CodebaseInfo> {
    return this.get<CodebaseInfo>(`/api/codebases/${codebaseId}`)
  }

  async generateSystemDocumentation(
    codebaseId: string,
    includeTests = false,
    maxFiles = 15
  ): Promise<SystemDocumentation> {
    const response = await this.post<{
      codebaseId: string
      documentation: SystemDocumentation
      fileCount: number
      generatedAt: string
    }>(`/api/codebases/${codebaseId}/generate-docs`, {
      includeTests,
      maxFiles
    })
    return response.documentation
  }

  async deleteCodebase(codebaseId: string): Promise<{ codebaseId: string; status: string; filesDeleted: number }> {
    return this.delete<{ codebaseId: string; status: string; filesDeleted: number }>(`/api/codebases/${codebaseId}`)
  }

  async listCodebases(
    projectId?: string,
    limit = 50,
    offset = 0
  ): Promise<{ codebases: CodebaseInfo[]; total: number; limit: number; offset: number }> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      offset: offset.toString()
    })
    if (projectId) {
      params.append('project_id', projectId)
    }
    return this.get(`/api/codebases/?${params.toString()}`)
  }

  async reindexCodebase(codebaseId: string): Promise<{ codebaseId: string; status: string; message: string }> {
    return this.post<{ codebaseId: string; status: string; message: string }>(`/api/codebases/${codebaseId}/reindex`, {})
  }

  async analyzeEvaluation(request: AnalysisRequest): Promise<{
    id: string
    valueScore: number
    executiveSummary: string
    technicalFeasibility: string
    gapAnalysis: string
    suggestedCR: string
    providerUsed: string
    similarEvaluations?: SearchResult[]
  }> {
    return this.post('/api/evaluations/analyze', request)
  }

  async getEvaluationHistory(
    projectId?: string,
    limit = 50,
    offset = 0
  ): Promise<{ evaluations: Evaluation[]; total: number; limit: number; offset: number }> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      offset: offset.toString()
    })
    if (projectId) {
      params.append('project_id', projectId)
    }
    return this.get(`/api/evaluations/history?${params.toString()}`)
  }

  async getEvaluation(evaluationId: string): Promise<Evaluation> {
    return this.get<Evaluation>(`/api/evaluations/${evaluationId}`)
  }

  async getSimilarEvaluations(evaluationId: string, limit = 10): Promise<{ evaluationId: string; similarEvaluations: SearchResult[]; count: number }> {
    return this.get<{ evaluationId: string; similarEvaluations: SearchResult[]; count: number }>(
      `/api/evaluations/${evaluationId}/similar?limit=${limit}`
    )
  }

  async searchArxiv(query: string, maxResults = 10, categories?: string[]): Promise<{
    query: string
    papers: ArxivPaper[]
    count: number
  }> {
    return this.post('/api/arxiv/search', {
      query,
      maxResults,
      categories
    })
  }

  async getArxivPaper(paperId: string): Promise<ArxivPaper> {
    return this.post<ArxivPaper>('/api/arxiv/paper', { paperId })
  }

  async getArxivCategories(): Promise<Record<string, { name: string; description: string }>> {
    return this.get('/api/arxiv/categories')
  }

  async findSimilarPapers(query: string, limit = 5): Promise<{
    query: string
    similarPapers: SearchResult[]
    count: number
  }> {
    return this.post(`/api/arxiv/similarity?limit=${limit}`, { query })
  }

  async createBrainstormSession(projectId: string, title: string): Promise<BrainstormSession> {
    return this.post('/api/sessions/', { projectId, title })
  }

  async listBrainstormSessions(
    projectId?: string,
    limit = 50,
    offset = 0
  ): Promise<{ sessions: BrainstormSession[]; total: number; limit: number; offset: number }> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      offset: offset.toString()
    })
    if (projectId) {
      params.append('project_id', projectId)
    }
    return this.get(`/api/sessions/?${params.toString()}`)
  }

  async getBrainstormSession(sessionId: string): Promise<BrainstormSession> {
    return this.get<BrainstormSession>(`/api/sessions/${sessionId}`)
  }

  async updateBrainstormSession(
    sessionId: string,
    updates: Partial<{ title: string; nodes: Node[]; edges: Edge[]; viewport: { x: number; y: number; zoom: number } }>
  ): Promise<BrainstormSession> {
    return this.put<BrainstormSession>(`/api/sessions/${sessionId}`, updates)
  }

  async deleteBrainstormSession(sessionId: string): Promise<{ sessionId: string; status: string }> {
    return this.delete<{ sessionId: string; status: string }>(`/api/sessions/${sessionId}`)
  }

  async generateBrainstormIdeas(
    topic: string,
    constraints?: string[],
    providerPreference?: string
  ): Promise<{ ideas: unknown[]; providerUsed: string; topic: string }> {
    return this.post('/api/brainstorm/ideas', {
      topic,
      constraints,
      providerPreference
    })
  }

  async expandIdea(idea: string, context?: string): Promise<{
    title: string
    description: string
    considerations: string[]
    nextSteps: string[]
  }> {
    return this.post('/api/brainstorm/expand', { idea, context })
  }

  async evaluateIdeas(
    ideas: string[],
    criteria?: string[]
  ): Promise<{ ideas: unknown[]; criteria: string[]; topIdea: unknown }> {
    return this.post('/api/brainstorm/evaluate', { ideas, criteria })
  }

  async connectIdeas(
    ideaA: string,
    ideaB: string,
    relationship = 'related'
  ): Promise<{ ideaA: string; ideaB: string; relationship: string; analysis: unknown }> {
    return this.post('/api/brainstorm/connect', {
      idea_a: ideaA,
      idea_b: ideaB,
      relationship
    })
  }

  async searchEvaluations(
    query: string,
    semantic = true,
    filters?: Record<string, unknown>,
    limit = 10
  ): Promise<{
    query: string
    searchType: string
    results: SearchResult[]
    count: number
  }> {
    const params = new URLSearchParams({
      query,
      semantic: semantic.toString(),
      limit: limit.toString()
    })
    return this.get(`/api/history/search?${params.toString()}`)
  }

  async getEvaluationStats(
    projectId?: string,
    dateFrom?: string,
    dateTo?: string
  ): Promise<{
    totalEvaluations: number
    avgValueScore: number
    medianValueScore: number
    providerUsage: Record<string, number>
    typeDistribution: Record<string, number>
    implementationRate: { total: number; implemented: number; inProgress: number; pending: number; rate: number }
    trend: { last7Days: number; last30Days: number; last90Days: number }
  }> {
    const params = new URLSearchParams()
    if (projectId) params.append('project_id', projectId)
    if (dateFrom) params.append('date_from', dateFrom)
    if (dateTo) params.append('date_to', dateTo)
    return this.get(`/api/history/stats?${params.toString()}`)
  }

  async getEvaluationTimeline(
    projectId?: string,
    days = 30
  ): Promise<{ projectId?: string; days: number; timeline: Array<{ date: string; count: number; avgScore: number }> }> {
    const params = new URLSearchParams({ days: days.toString() })
    if (projectId) params.append('project_id', projectId)
    return this.get(`/api/history/timeline?${params.toString()}`)
  }

  async getTopEvaluations(
    projectId?: string,
    limit = 10,
    sortBy = 'value_score'
  ): Promise<{ projectId?: string; sortBy: string; limit: number; evaluations: unknown[] }> {
    const params = new URLSearchParams({
      limit: limit.toString(),
      sort_by: sortBy
    })
    if (projectId) params.append('project_id', projectId)
    return this.get(`/api/history/top-evaluations?${params.toString()}`)
  }

  async exportHistory(
    projectId?: string,
    format: 'json' | 'csv' = 'json',
    dateFrom?: string,
    dateTo?: string
  ): Promise<unknown> {
    const params = new URLSearchParams({ format })
    if (projectId) params.append('project_id', projectId)
    if (dateFrom) params.append('date_from', dateFrom)
    if (dateTo) params.append('date_to', dateTo)
    return this.get(`/api/history/export?${params.toString()}`)
  }

  async fetchGitHubRepo(url: string): Promise<CodeFile[]> {
    return this.initializeCodebase({
      type: 'github',
      githubUrl: url
    }).then(async (response) => {
      const codebase = await this.getCodebase(response.codebaseId)
      return codebase.files
    })
  }
}

export const apiClient = new APIClient()

export { APIErrorImpl as APIError, APIErrorImpl }

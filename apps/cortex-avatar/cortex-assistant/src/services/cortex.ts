import type {
  MeetingSession,
  MeetingAnalysis,
  MemoryCommitPayload,
  CommitResult,
  KnowledgeIngestPayload,
  IngestResult,
  SearchResults,
  TranscriptChunk,
  ActionItem,
  Decision,
  KeyPoint,
  Risk,
  FollowUp,
  Topic,
} from '@/models';

export interface CortexClientConfig {
  baseUrl: string;
  timeout?: number;
  retryAttempts?: number;
  retryDelay?: number;
}

export interface CortexClient {
  analyzeMeeting(session: MeetingSession): Promise<MeetingAnalysis>;
  commitMemory(payload: MemoryCommitPayload): Promise<CommitResult>;
  ingestKnowledge(payload: KnowledgeIngestPayload): Promise<IngestResult>;
  searchMemory(query: string, options?: SearchOptions): Promise<SearchResults>;
  sttTranscribe(audioBlob: Blob, options?: STTOptions): Promise<TranscriptChunk>;
  ping(): Promise<number>;
  getAgentCard(): Promise<AgentCard | null>;
}

export interface SearchOptions {
  limit?: number;
  offset?: number;
  sources?: ('memory' | 'knowledge' | 'local')[];
  dateFrom?: string;
  dateTo?: string;
}

export interface STTOptions {
  language?: string;
  model?: string;
  prompt?: string;
}

export interface AgentCard {
  name: string;
  version: string;
  description: string;
  protocolVersion: string;
  capabilities: {
    streaming: boolean;
    stt: boolean;
    memory: boolean;
    knowledge: boolean;
  };
}

const DEFAULT_CONFIG: CortexClientConfig = {
  baseUrl: 'http://localhost:8080',
  timeout: 30000,
  retryAttempts: 3,
  retryDelay: 1000,
};

async function fetchWithRetry(
  url: string,
  options: RequestInit,
  config: CortexClientConfig
): Promise<Response> {
  const { timeout = 30000, retryAttempts = 3, retryDelay = 1000 } = config;
  let lastError: Error | null = null;

  for (let attempt = 0; attempt < retryAttempts; attempt++) {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), timeout);

    try {
      const response = await fetch(url, {
        ...options,
        signal: controller.signal,
      });
      clearTimeout(timeoutId);

      if (response.ok || response.status < 500) {
        return response;
      }

      lastError = new Error(`HTTP ${response.status}: ${response.statusText}`);
    } catch (err) {
      clearTimeout(timeoutId);
      lastError = err instanceof Error ? err : new Error(String(err));

      if (err instanceof Error && err.name === 'AbortError') {
        lastError = new Error('Request timeout');
      }
    }

    if (attempt < retryAttempts - 1) {
      await new Promise((resolve) => setTimeout(resolve, retryDelay * (attempt + 1)));
    }
  }

  throw lastError || new Error('Request failed after retries');
}

export class HttpCortexClient implements CortexClient {
  private config: CortexClientConfig;

  constructor(config: Partial<CortexClientConfig> = {}) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  private async request<T>(
    method: string,
    endpoint: string,
    body?: unknown
  ): Promise<T> {
    const url = `${this.config.baseUrl}${endpoint}`;
    const options: RequestInit = {
      method,
      headers: {
        'Content-Type': 'application/json',
        Accept: 'application/json',
      },
    };

    if (body) {
      options.body = JSON.stringify(body);
    }

    const response = await fetchWithRetry(url, options, this.config);

    if (!response.ok) {
      const errorText = await response.text();
      throw new Error(`Cortex API error: ${response.status} - ${errorText}`);
    }

    return response.json();
  }

  async analyzeMeeting(session: MeetingSession): Promise<MeetingAnalysis> {
    const fullText = session.segments.map((s) => s.text).join(' ');

    const payload = {
      jsonrpc: '2.0',
      method: 'message/send',
      params: {
        message: {
          role: 'user',
          parts: [
            {
              kind: 'text',
              text: `Analyze this meeting transcript and provide:
1. A concise summary (2-3 paragraphs)
2. Key decisions made
3. Action items with assignees if mentioned
4. Key discussion points
5. Overall sentiment and tone
6. Any risks or blockers identified
7. Topics discussed with approximate time spent
8. Follow-up items and suggested agenda for next meeting

Meeting: ${session.title}
Participants: ${session.participants.map((p) => p.name).join(', ') || 'Unknown'}
Duration: ${Math.floor(session.duration / 60)} minutes

Transcript:
${fullText}

Please respond with a structured JSON analysis.`,
            },
          ],
          metadata: {
            analysisRequest: true,
            meetingId: session.id,
          },
        },
      },
      id: Date.now(),
    };

    const result = await this.request<{
      result: { status: { message: { parts: Array<{ text?: string }> } } };
    }>('POST', '/', payload);

    const responseText =
      result.result?.status?.message?.parts?.[0]?.text || '';
    return this.parseAnalysisResponse(responseText, session.id);
  }

  private parseAnalysisResponse(text: string, meetingId: string): MeetingAnalysis {
    try {
      const jsonMatch = text.match(/```json\s*([\s\S]*?)\s*```/) || 
                        text.match(/\{[\s\S]*\}/);
      if (jsonMatch) {
        const parsed = JSON.parse(jsonMatch[1] || jsonMatch[0]);
        return {
          id: `analysis-${Date.now()}`,
          meetingId,
          summary: parsed.summary || '',
          decisions: parsed.decisions || [],
          actionItems: parsed.actionItems || [],
          keyPoints: parsed.keyPoints || [],
          sentiment: parsed.sentiment || 'neutral',
          sentimentScore: parsed.sentimentScore || 0.5,
          risks: parsed.risks || [],
          topics: parsed.topics || [],
          followUps: parsed.followUps || [],
          nextAgendaSuggestions: parsed.nextAgendaSuggestions || [],
          generatedAt: new Date().toISOString(),
        };
      }
    } catch {
    }

    return {
      id: `analysis-${Date.now()}`,
      meetingId,
      summary: text,
      decisions: [],
      actionItems: [],
      keyPoints: [],
      sentiment: 'neutral',
      sentimentScore: 0.5,
      risks: [],
      topics: [],
      followUps: [],
      nextAgendaSuggestions: [],
      generatedAt: new Date().toISOString(),
    };
  }

  async commitMemory(payload: MemoryCommitPayload): Promise<CommitResult> {
    const rpcPayload = {
      jsonrpc: '2.0',
      method: 'memory/commit',
      params: {
        type: 'meeting_summary',
        content: {
          meetingId: payload.meetingId,
          title: payload.title,
          summary: payload.summary,
          decisions: payload.decisions,
          actionItems: payload.actionItems,
          keyPoints: payload.keyPoints,
          participants: payload.participants,
          tags: payload.tags,
        },
        metadata: {
          redacted: payload.redactionsApplied,
          timestamp: payload.timestamp,
        },
      },
      id: Date.now(),
    };

    try {
      const result = await this.request<{ result: { id: string } }>(
        'POST',
        '/',
        rpcPayload
      );
      return {
        success: true,
        memoryId: result.result?.id,
        message: 'Memory committed successfully',
        timestamp: new Date().toISOString(),
      };
    } catch (err) {
      return {
        success: false,
        message: err instanceof Error ? err.message : 'Failed to commit memory',
        timestamp: new Date().toISOString(),
      };
    }
  }

  async ingestKnowledge(payload: KnowledgeIngestPayload): Promise<IngestResult> {
    const rpcPayload = {
      jsonrpc: '2.0',
      method: 'knowledge/ingest',
      params: {
        type: 'meeting_transcript',
        title: payload.title,
        content: payload.fullText,
        metadata: {
          meetingId: payload.meetingId,
          participants: payload.participants,
          segments: payload.segments.length,
          tags: payload.tags,
          ...payload.metadata,
        },
      },
      id: Date.now(),
    };

    try {
      const result = await this.request<{
        result: { id: string; chunks: number };
      }>('POST', '/', rpcPayload);
      return {
        success: true,
        knowledgeId: result.result?.id,
        message: 'Knowledge ingested successfully',
        chunksCreated: result.result?.chunks || 0,
        timestamp: new Date().toISOString(),
      };
    } catch (err) {
      return {
        success: false,
        message: err instanceof Error ? err.message : 'Failed to ingest knowledge',
        chunksCreated: 0,
        timestamp: new Date().toISOString(),
      };
    }
  }

  async searchMemory(
    query: string,
    options: SearchOptions = {}
  ): Promise<SearchResults> {
    const rpcPayload = {
      jsonrpc: '2.0',
      method: 'memory/search',
      params: {
        query,
        limit: options.limit || 20,
        offset: options.offset || 0,
        sources: options.sources || ['memory', 'knowledge'],
        filters: {
          dateFrom: options.dateFrom,
          dateTo: options.dateTo,
        },
      },
      id: Date.now(),
    };

    const startTime = Date.now();

    try {
      const result = await this.request<{
        result: {
          results: Array<{
            id: string;
            type: string;
            title: string;
            snippet: string;
            score: number;
            metadata: Record<string, unknown>;
            source: string;
            timestamp: string;
          }>;
          total: number;
        };
      }>('POST', '/', rpcPayload);

      return {
        query,
        results: (result.result?.results || []).map((r) => ({
          id: r.id,
          type: r.type as 'meeting' | 'memory' | 'knowledge',
          title: r.title,
          snippet: r.snippet,
          relevanceScore: r.score,
          metadata: r.metadata,
          source: r.source,
          timestamp: r.timestamp,
        })),
        totalCount: result.result?.total || 0,
        sources: {
          local: 0,
          cortexMemory: result.result?.results?.filter((r) => r.source === 'memory')
            .length || 0,
          cortexKnowledge: result.result?.results?.filter(
            (r) => r.source === 'knowledge'
          ).length || 0,
        },
        searchDuration: Date.now() - startTime,
      };
    } catch {
      return {
        query,
        results: [],
        totalCount: 0,
        sources: { local: 0, cortexMemory: 0, cortexKnowledge: 0 },
        searchDuration: Date.now() - startTime,
      };
    }
  }

  async sttTranscribe(audioBlob: Blob, options: STTOptions = {}): Promise<TranscriptChunk> {
    const formData = new FormData();
    formData.append('audio', audioBlob, 'audio.webm');
    formData.append('language', options.language || 'en');
    if (options.model) formData.append('model', options.model);
    if (options.prompt) formData.append('prompt', options.prompt);

    const url = `${this.config.baseUrl}/stt/transcribe`;
    const response = await fetchWithRetry(
      url,
      {
        method: 'POST',
        body: formData,
      },
      this.config
    );

    const result = await response.json();

    return {
      text: result.text || '',
      isFinal: true,
      confidence: result.confidence || 0.9,
      startTime: result.start || 0,
      endTime: result.end || 0,
    };
  }

  async ping(): Promise<number> {
    const start = Date.now();
    await this.getAgentCard();
    return Date.now() - start;
  }

  async getAgentCard(): Promise<AgentCard | null> {
    try {
      const response = await fetchWithRetry(
        `${this.config.baseUrl}/.well-known/agent-card.json`,
        { method: 'GET' },
        { ...this.config, retryAttempts: 1 }
      );
      return response.json();
    } catch {
      return null;
    }
  }
}

export class MockCortexClient implements CortexClient {
  private delay = 500;

  private async simulateDelay(): Promise<void> {
    await new Promise((resolve) => setTimeout(resolve, this.delay));
  }

  async analyzeMeeting(session: MeetingSession): Promise<MeetingAnalysis> {
    await this.simulateDelay();

    const actionItems: ActionItem[] = [
      {
        id: `action-${Date.now()}-1`,
        text: 'Review the project timeline and update milestones',
        assignee: session.participants[0]?.name || 'Unassigned',
        priority: 'high',
        status: 'pending',
        sourceSegmentIds: session.segments.slice(0, 2).map((s) => s.id),
        meetingId: session.id,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
      {
        id: `action-${Date.now()}-2`,
        text: 'Schedule follow-up meeting with stakeholders',
        priority: 'medium',
        status: 'pending',
        sourceSegmentIds: [],
        meetingId: session.id,
        createdAt: new Date().toISOString(),
        updatedAt: new Date().toISOString(),
      },
    ];

    const decisions: Decision[] = [
      {
        id: `decision-${Date.now()}-1`,
        text: 'Agreed to proceed with the proposed approach',
        context: 'After discussing alternatives',
        sourceSegmentIds: [],
      },
    ];

    const keyPoints: KeyPoint[] = [
      {
        id: `keypoint-${Date.now()}-1`,
        text: 'Main discussion focused on project timeline',
        category: 'planning',
        sourceSegmentIds: [],
      },
    ];

    const risks: Risk[] = [
      {
        id: `risk-${Date.now()}-1`,
        text: 'Potential delay in delivery due to resource constraints',
        severity: 'medium',
        sourceSegmentIds: [],
      },
    ];

    const topics: Topic[] = [
      {
        id: `topic-${Date.now()}-1`,
        name: 'Project Status',
        segmentIds: session.segments.slice(0, 3).map((s) => s.id),
        duration: Math.floor(session.duration * 0.4),
      },
      {
        id: `topic-${Date.now()}-2`,
        name: 'Next Steps',
        segmentIds: session.segments.slice(3).map((s) => s.id),
        duration: Math.floor(session.duration * 0.3),
      },
    ];

    const followUps: FollowUp[] = [
      {
        id: `followup-${Date.now()}-1`,
        text: 'Prepare status report',
        assignee: session.participants[0]?.name,
        deadline: new Date(Date.now() + 7 * 24 * 60 * 60 * 1000).toISOString(),
      },
    ];

    return {
      id: `analysis-${Date.now()}`,
      meetingId: session.id,
      summary: `This meeting covered ${session.title || 'various topics'}. The team discussed the current status and identified key action items. ${session.segments.length} transcript segments were analyzed.`,
      decisions,
      actionItems,
      keyPoints,
      sentiment: 'positive',
      sentimentScore: 0.7,
      risks,
      topics,
      followUps,
      nextAgendaSuggestions: [
        'Review progress on action items',
        'Address any blockers identified',
        'Plan next sprint activities',
      ],
      generatedAt: new Date().toISOString(),
      modelUsed: 'mock-model-v1',
    };
  }

  async commitMemory(_payload: MemoryCommitPayload): Promise<CommitResult> {
    await this.simulateDelay();
    return {
      success: true,
      memoryId: `memory-${Date.now()}`,
      message: 'Memory committed successfully (mock)',
      timestamp: new Date().toISOString(),
    };
  }

  async ingestKnowledge(_payload: KnowledgeIngestPayload): Promise<IngestResult> {
    await this.simulateDelay();
    const chunksCreated = Math.ceil(_payload.fullText.length / 500);
    return {
      success: true,
      knowledgeId: `knowledge-${Date.now()}`,
      message: 'Knowledge ingested successfully (mock)',
      chunksCreated,
      timestamp: new Date().toISOString(),
    };
  }

  async searchMemory(query: string, _options?: SearchOptions): Promise<SearchResults> {
    await this.simulateDelay();
    return {
      query,
      results: [
        {
          id: 'result-1',
          type: 'memory',
          title: 'Previous Meeting Summary',
          snippet: `...related to "${query}"...`,
          relevanceScore: 0.85,
          metadata: {},
          source: 'cortex-memory',
          timestamp: new Date(Date.now() - 86400000).toISOString(),
        },
        {
          id: 'result-2',
          type: 'knowledge',
          title: 'Related Discussion',
          snippet: `...context about "${query}"...`,
          relevanceScore: 0.72,
          metadata: {},
          source: 'cortex-knowledge',
          timestamp: new Date(Date.now() - 172800000).toISOString(),
        },
      ],
      totalCount: 2,
      sources: {
        local: 0,
        cortexMemory: 1,
        cortexKnowledge: 1,
      },
      searchDuration: this.delay,
    };
  }

  async sttTranscribe(): Promise<TranscriptChunk> {
    await this.simulateDelay();
    return {
      text: 'Mock transcription result',
      isFinal: true,
      confidence: 0.95,
      startTime: 0,
      endTime: 2000,
    };
  }

  async ping(): Promise<number> {
    const start = Date.now();
    await this.simulateDelay();
    return Date.now() - start;
  }

  async getAgentCard(): Promise<AgentCard> {
    await this.simulateDelay();
    return {
      name: 'Cortex-02 (Mock)',
      version: '2.0.0',
      description: 'Mock Cortex client for development',
      protocolVersion: 'A2A v0.3.0',
      capabilities: {
        streaming: true,
        stt: true,
        memory: true,
        knowledge: true,
      },
    };
  }
}

let clientInstance: CortexClient | null = null;

export function getCortexClient(config?: Partial<CortexClientConfig>): CortexClient {
  if (!clientInstance) {
    const useMock = import.meta.env.VITE_USE_MOCK_CORTEX === 'true';
    clientInstance = useMock
      ? new MockCortexClient()
      : new HttpCortexClient(config);
  }
  return clientInstance;
}

export function resetCortexClient(): void {
  clientInstance = null;
}

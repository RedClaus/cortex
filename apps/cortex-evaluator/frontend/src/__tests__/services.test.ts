import { describe, it, expect, beforeEach, vi } from 'vitest';
import { APIClient, APIError } from '../services/api';

describe('API Client', () => {
  let apiClient: APIClient;
  let mockFetch: any;

  beforeEach(() => {
    apiClient = new APIClient();
    mockFetch = vi.fn();
    global.fetch = mockFetch;
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should configure baseURL and auth headers', () => {
    const newBaseURL = 'https://api.example.com';
    const authHeaders = { 'Authorization': 'Bearer token123' };

    apiClient.configure(newBaseURL, authHeaders);

    expect((apiClient as any).baseURL).toBe(newBaseURL);
    expect((apiClient as any).authHeaders).toEqual(authHeaders);
  });

  it('should handle successful GET request', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ status: 'ok' })
    } as Response);

    const result = await apiClient.healthCheck();

    expect(mockFetch).toHaveBeenCalledWith('http://localhost:8000/health', {
      method: 'GET',
      headers: { 'Content-Type': 'application/json' }
    });
    expect(result).toEqual({ status: 'ok' });
  });

  it('should handle successful POST request', async () => {
    const mockData = { codebaseId: 'cb-001' };
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockData
    } as Response);

    const result = await apiClient.initializeCodebase({
      type: 'github',
      githubUrl: 'https://github.com/user/repo'
    });

    expect(mockFetch).toHaveBeenCalledWith('http://localhost:8000/api/codebases/initialize', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ type: 'github', githubUrl: 'https://github.com/user/repo' })
    });
    expect(result).toEqual(mockData);
  });

  it('should handle error responses', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: false,
      status: 404,
      json: async () => ({ error: 'Not found' })
    } as Response);

    await expect(apiClient.getCodebase('nonexistent')).rejects.toThrow(APIError);
  });

  it('should handle network errors', async () => {
    mockFetch.mockRejectedValueOnce(new Error('Network error'));

    await expect(apiClient.getCodebase('cb-001')).rejects.toThrow();
  });

  it('should parse JSON responses', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      headers: new Headers({ 'content-type': 'application/json' }),
      json: async () => ({ data: 'test' })
    } as Response);

    const result = await apiClient.getEvaluationHistory();

    expect(result).toEqual({ data: 'test' });
  });

  it('should handle text responses', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      headers: new Headers({ 'content-type': 'text/plain' }),
      text: async () => 'plain text response'
    } as Response);

    const result = await apiClient.getCodebase('cb-001');

    expect(result).toBe('plain text response');
  });

  it('should include auth headers in all requests', async () => {
    const authHeaders = { 'X-API-Key': 'secret123' };
    apiClient.configure('http://localhost:8000', authHeaders);

    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({})
    } as Response);

    await apiClient.healthCheck();

    expect(mockFetch).toHaveBeenCalledWith(
      'http://localhost:8000/health',
      expect.objectContaining({
        headers: expect.objectContaining(authHeaders)
      })
    );
  });

  it('should build correct query parameters', async () => {
    mockFetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({ evaluations: [] })
    } as Response);

    await apiClient.getEvaluationHistory('proj-123', 10, 0);

    const url = mockFetch.mock.calls[0][0];
    expect(url).toContain('project_id=proj-123');
    expect(url).toContain('limit=10');
    expect(url).toContain('offset=0');
  });
});

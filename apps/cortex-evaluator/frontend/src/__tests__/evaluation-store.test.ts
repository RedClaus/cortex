import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia } from 'pinia';

describe('Evaluation Store', () => {
  let useEvaluationStore: any;
  let pinia: any;

  beforeEach(async () => {
    pinia = (await import('pinia')).createPinia();
    setActivePinia(pinia);
    vi.useFakeTimers();
    global.fetch = vi.fn() as any;
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('should initialize with empty evaluations and results', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { evaluations, currentEvaluation, results } = useEvaluationStore();

    expect(evaluations).toEqual([]);
    expect(currentEvaluation).toBe(null);
    expect(results).toEqual([]);
  });

  it('should add evaluation', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addEvaluation, evaluations } = useEvaluationStore();

    const newEvaluation = {
      id: 'eval-1',
      workspaceId: 'workspace-1',
      name: 'Test Evaluation',
      criteria: [],
      status: 'pending' as const,
      createdAt: new Date()
    };

    addEvaluation(newEvaluation);

    expect(evaluations.length).toBe(1);
    expect(evaluations[0]).toEqual(newEvaluation);
  });

  it('should set current evaluation', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { setCurrentEvaluation, currentEvaluation } = useEvaluationStore();

    const evaluation = {
      id: 'eval-1',
      workspaceId: 'workspace-1',
      name: 'Current',
      criteria: [],
      status: 'running' as const,
      createdAt: new Date()
    };

    setCurrentEvaluation(evaluation);

    expect(currentEvaluation).toEqual(evaluation);
  });

  it('should update evaluation', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addEvaluation, updateEvaluation, evaluations, currentEvaluation } = useEvaluationStore();

    const evaluation = {
      id: 'eval-1',
      workspaceId: 'workspace-1',
      name: 'Test',
      criteria: [],
      status: 'pending' as const,
      createdAt: new Date()
    };

    addEvaluation(evaluation);
    setCurrentEvaluation(evaluation);

    updateEvaluation('eval-1', { status: 'running' as const, completedAt: new Date() });

    expect(evaluations[0].status).toBe('running');
    expect(currentEvaluation.status).toBe('running');
  });

  it('should delete evaluation', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addEvaluation, deleteEvaluation, evaluations, currentEvaluation } = useEvaluationStore();

    const eval1 = {
      id: 'eval-1',
      workspaceId: 'workspace-1',
      name: 'Test 1',
      criteria: [],
      status: 'pending' as const,
      createdAt: new Date()
    };

    const eval2 = {
      id: 'eval-2',
      workspaceId: 'workspace-1',
      name: 'Test 2',
      criteria: [],
      status: 'pending' as const,
      createdAt: new Date()
    };

    addEvaluation(eval1);
    addEvaluation(eval2);
    setCurrentEvaluation(eval1);

    expect(evaluations.length).toBe(2);

    deleteEvaluation('eval-1');

    expect(evaluations.length).toBe(1);
    expect(evaluations.find((e: any) => e.id === 'eval-1')).toBeUndefined();
    expect(currentEvaluation).toBe(null);
  });

  it('should add result', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addResult, results } = useEvaluationStore();

    const newResult = {
      id: 'result-1',
      evaluationId: 'eval-1',
      criteriaId: 'criteria-1',
      metricId: 'metric-1',
      score: 85,
      notes: 'Good performance',
      feedback: 'Consider optimization',
      timestamp: new Date()
    };

    addResult(newResult);

    expect(results.length).toBe(1);
    expect(results[0]).toEqual(newResult);
  });

  it('should get results by evaluation id', () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addResult, getResult } = useEvaluationStore();

    addResult({ id: 'result-1', evaluationId: 'eval-1', criteriaId: 'c1', metricId: 'm1', score: 80, notes: '', feedback: '', timestamp: new Date() });
    addResult({ id: 'result-2', evaluationId: 'eval-2', criteriaId: 'c2', metricId: 'm2', score: 90, notes: '', feedback: '', timestamp: new Date() });
    addResult({ id: 'result-3', evaluationId: 'eval-1', criteriaId: 'c1', metricId: 'm2', score: 85, notes: '', feedback: '', timestamp: new Date() });

    const eval1Results = getResult('eval-1');

    expect(eval1Results.length).toBe(2);
    expect(eval1Results.every((r: any) => r.evaluationId === 'eval-1')).toBe(true);
  });

  it('should fetch evaluations from API', async () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { fetchEvaluations, evaluations } = useEvaluationStore();

    const mockResponse = [
      {
        id: 'eval-1',
        workspaceId: 'workspace-1',
        name: 'Test 1',
        criteria: [],
        status: 'completed' as const,
        createdAt: new Date()
      }
    ];

    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => mockResponse
    } as Response);

    await fetchEvaluations('workspace-1');

    expect(global.fetch).toHaveBeenCalledWith('/api/evaluations?workspaceId=workspace-1');
    expect(evaluations).toEqual(mockResponse);
  });

  it('should handle fetch evaluations error', async () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { fetchEvaluations } = useEvaluationStore();

    global.fetch.mockRejectedValueOnce(new Error('Network error'));

    await expect(fetchEvaluations('workspace-1')).rejects.toThrow('Network error');
  });

  it('should run evaluation', async () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addEvaluation, runEvaluation, evaluations, updateEvaluation } = useEvaluationStore();

    const evaluation = {
      id: 'eval-1',
      workspaceId: 'workspace-1',
      name: 'Test',
      criteria: [],
      status: 'pending' as const,
      createdAt: new Date()
    };

    addEvaluation(evaluation);

    global.fetch.mockResolvedValueOnce({
      ok: true,
      json: async () => ({
        results: [
          { id: 'result-1', evaluationId: 'eval-1', score: 85 }
        ]
      })
    } as Response);

    await runEvaluation('eval-1');

    expect(updateEvaluation).toHaveBeenCalledWith('eval-1', { status: 'running' as const });
    expect(global.fetch).toHaveBeenCalledWith('/api/evaluations/eval-1/run', { method: 'POST' });
  });

  it('should handle run evaluation failure', async () => {
    const { useEvaluationStore } = await import('../stores/useEvaluationStore');
    useEvaluationStore = useEvaluationStore();
    const { addEvaluation, runEvaluation, evaluations, updateEvaluation } = useEvaluationStore();

    const evaluation = {
      id: 'eval-1',
      workspaceId: 'workspace-1',
      name: 'Test',
      criteria: [],
      status: 'pending' as const,
      createdAt: new Date()
    };

    addEvaluation(evaluation);

    global.fetch.mockRejectedValueOnce(new Error('API error'));

    await expect(runEvaluation('eval-1')).rejects.toThrow();

    expect(evaluations[0].status).toBe('failed');
  });
});

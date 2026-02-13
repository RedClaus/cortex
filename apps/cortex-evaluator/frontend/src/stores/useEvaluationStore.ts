import { create } from 'zustand';
import { EvaluationStore, Evaluation, EvaluationResult } from './types';

interface EvaluationSlice {
  evaluations: Evaluation[];
  currentEvaluation: Evaluation | null;
  addEvaluation: (evaluation: Evaluation) => void;
  setCurrentEvaluation: (evaluation: Evaluation | null) => void;
  updateEvaluation: (evaluationId: string, updates: Partial<Evaluation>) => void;
  deleteEvaluation: (evaluationId: string) => void;
  fetchEvaluations: (workspaceId: string) => Promise<void>;
  runEvaluation: (evaluationId: string) => Promise<void>;
}

interface ResultSlice {
  results: EvaluationResult[];
  addResult: (result: EvaluationResult) => void;
  getResult: (evaluationId: string) => EvaluationResult[];
}

const createEvaluationSlice: (
  set: (partial: Partial<EvaluationStore> | ((state: EvaluationStore) => Partial<EvaluationStore>)) => void,
  get: () => EvaluationStore
) => EvaluationSlice = (set, get) => ({
  evaluations: [],
  currentEvaluation: null,
  addEvaluation: (evaluation) =>
    set((state) => ({
      evaluations: [...state.evaluations, evaluation],
    })),
  setCurrentEvaluation: (evaluation) => set({ currentEvaluation: evaluation }),
  updateEvaluation: (evaluationId, updates) =>
    set((state) => ({
      evaluations: state.evaluations.map((e) =>
        e.id === evaluationId ? { ...e, ...updates } : e
      ),
      currentEvaluation:
        state.currentEvaluation?.id === evaluationId
          ? { ...state.currentEvaluation, ...updates }
          : state.currentEvaluation,
    })),
  deleteEvaluation: (evaluationId) =>
    set((state) => ({
      evaluations: state.evaluations.filter((e) => e.id !== evaluationId),
      currentEvaluation:
        state.currentEvaluation?.id === evaluationId
          ? null
          : state.currentEvaluation,
    })),
  fetchEvaluations: async (workspaceId) => {
    try {
      const response = await fetch(`/api/evaluations?workspaceId=${workspaceId}`);
      if (!response.ok) {
        throw new Error('Failed to fetch evaluations');
      }
      const evaluations: Evaluation[] = await response.json();
      set({ evaluations });
    } catch (error) {
      console.error('Error fetching evaluations:', error);
      throw error;
    }
  },
  runEvaluation: async (evaluationId) => {
    const { updateEvaluation } = get();
    
    updateEvaluation(evaluationId, { status: 'running' });
    
    try {
      const response = await fetch(`/api/evaluations/${evaluationId}/run`, {
        method: 'POST',
      });
      
      if (!response.ok) {
        throw new Error('Failed to run evaluation');
      }
      
      const result = await response.json();
      
      updateEvaluation(evaluationId, {
        status: 'completed',
        completedAt: new Date(),
      });
      
      set((state) => ({
        results: [...state.results, ...result.results],
      }));
    } catch (error) {
      console.error('Error running evaluation:', error);
      updateEvaluation(evaluationId, { status: 'failed' });
      throw error;
    }
  },
});

const createResultSlice: (
  set: (partial: Partial<EvaluationStore> | ((state: EvaluationStore) => Partial<EvaluationStore>)) => void,
  get: () => EvaluationStore
) => ResultSlice = (set, get) => ({
  results: [],
  addResult: (result) =>
    set((state) => ({
      results: [...state.results, result],
    })),
  getResult: (evaluationId) => {
    const { results } = get();
    return results.filter((r) => r.evaluationId === evaluationId);
  },
});

export const useEvaluationStore = create<EvaluationStore>((set, get) => ({
  ...createEvaluationSlice(set, get),
  ...createResultSlice(set, get),
}));

export const useEvaluations = () =>
  useEvaluationStore((state) => state.evaluations);

export const useCurrentEvaluation = () =>
  useEvaluationStore((state) => state.currentEvaluation);

export const useEvaluationResults = (evaluationId: string) =>
  useEvaluationStore((state) =>
    state.results.filter((r) => r.evaluationId === evaluationId)
  );

export const useEvaluationActions = () =>
  useEvaluationStore((state) => ({
    addEvaluation: state.addEvaluation,
    setCurrentEvaluation: state.setCurrentEvaluation,
    updateEvaluation: state.updateEvaluation,
    deleteEvaluation: state.deleteEvaluation,
    fetchEvaluations: state.fetchEvaluations,
    runEvaluation: state.runEvaluation,
    addResult: state.addResult,
    getResult: state.getResult,
  }));

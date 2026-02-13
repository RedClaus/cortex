import { useState, useCallback, useRef } from 'react'
import { apiClient, APIErrorImpl } from '../services/api'
import type {
  AnalysisRequest,
  Evaluation,
  EvaluationResult
} from '../types/api'

interface UseAnalysisReturn {
  loading: boolean
  error: string | null
  result: {
    id: string
    valueScore: number
    executiveSummary: string
    technicalFeasibility: string
    gapAnalysis: string
    suggestedCR: string
    providerUsed: string
    similarEvaluations?: any[]
  } | null
  analyze: (request: AnalysisRequest) => Promise<void>
  reset: () => void
}

export function useAnalysis(): UseAnalysisReturn {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [result, setResult] = useState<UseAnalysisReturn['result']>(null)
  const abortControllerRef = useRef<AbortController | null>(null)

  const analyze = useCallback(async (request: AnalysisRequest) => {
    setLoading(true)
    setError(null)
    setResult(null)

    abortControllerRef.current = new AbortController()
    const { signal } = abortControllerRef.current

    try {
      const response = await apiClient.analyzeEvaluation(request)
      setResult({
        id: response.id,
        valueScore: response.valueScore,
        executiveSummary: response.executiveSummary,
        technicalFeasibility: response.technicalFeasibility,
        gapAnalysis: response.gapAnalysis,
        suggestedCR: response.suggestedCR,
        providerUsed: response.providerUsed,
        similarEvaluations: response.similarEvaluations
      })
    } catch (err) {
      if (err instanceof Error && err.name === 'AbortError') {
        return
      }
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to analyze'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [])

  const reset = useCallback(() => {
    setResult(null)
    setError(null)
  }, [])

  const cancel = useCallback(() => {
    abortControllerRef.current?.abort()
    setLoading(false)
  }, [])

  return { loading, error, result, analyze, reset, cancel }
}

export function useEvaluation(evaluationId?: string) {
  const [evaluation, setEvaluation] = useState<Evaluation | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchEvaluation = useCallback(async (id: string) => {
    setLoading(true)
    setError(null)
    try {
      const data = await apiClient.getEvaluation(id)
      setEvaluation(data)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch evaluation'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [])

  const fetchSimilar = useCallback(async (id: string, limit = 10) => {
    try {
      const response = await apiClient.getSimilarEvaluations(id, limit)
      return response.similarEvaluations
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch similar evaluations'
      setError(message)
      return []
    }
  }, [])

  return { evaluation, loading, error, fetchEvaluation, fetchSimilar }
}

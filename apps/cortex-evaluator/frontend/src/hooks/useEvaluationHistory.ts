import { useState, useCallback } from 'react'
import { apiClient, APIErrorImpl } from '../services/api'
import type { Evaluation, SearchResult } from '../types/api'

interface UseEvaluationHistoryReturn {
  evaluations: Evaluation[]
  loading: boolean
  error: string | null
  total: number
  page: number
  pageSize: number
  fetchPage: (page: number, pageSize?: number) => Promise<void>
  nextPage: () => void
  prevPage: () => void
  refetch: () => Promise<void>
}

export function useEvaluationHistory(projectId?: string, initialPageSize = 50): UseEvaluationHistoryReturn {
  const [evaluations, setEvaluations] = useState<Evaluation[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [total, setTotal] = useState(0)
  const [page, setPage] = useState(0)
  const [pageSize, setPageSize] = useState(initialPageSize)

  const fetchPage = useCallback(async (newPage: number, newPageSize?: number) => {
    setLoading(true)
    setError(null)
    const actualPageSize = newPageSize || pageSize
    const offset = newPage * actualPageSize

    try {
      const response = await apiClient.getEvaluationHistory(projectId, actualPageSize, offset)
      setEvaluations(response.evaluations)
      setTotal(response.total)
      setPage(newPage)
      if (newPageSize) setPageSize(newPageSize)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch evaluation history'
      setError(message)
      setEvaluations([])
    } finally {
      setLoading(false)
    }
  }, [projectId, pageSize])

  const nextPage = useCallback(() => {
    const maxPage = Math.ceil(total / pageSize) - 1
    if (page < maxPage) {
      fetchPage(page + 1)
    }
  }, [page, total, pageSize, fetchPage])

  const prevPage = useCallback(() => {
    if (page > 0) {
      fetchPage(page - 1)
    }
  }, [page, fetchPage])

  const refetch = useCallback(async () => {
    await fetchPage(page, pageSize)
  }, [page, pageSize, fetchPage])

  return {
    evaluations,
    loading,
    error,
    total,
    page,
    pageSize,
    fetchPage,
    nextPage,
    prevPage,
    refetch
  }
}

interface UseSearchReturn {
  results: SearchResult[]
  loading: boolean
  error: string | null
  search: (query: string, semantic?: boolean, filters?: Record<string, unknown>, limit?: number) => Promise<void>
  hasMore: boolean
  loadMore: () => void
}

export function useEvaluationSearch(projectId?: string, initialLimit = 10): UseSearchReturn {
  const [results, setResults] = useState<SearchResult[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [currentQuery, setCurrentQuery] = useState('')
  const [limit, setLimit] = useState(initialLimit)

  const search = useCallback(async (query: string, semantic = true, filters?: Record<string, unknown>, limitArg = 10) => {
    setLoading(true)
    setError(null)
    setCurrentQuery(query)
    setLimit(limitArg)

    try {
      const response = await apiClient.searchEvaluations(query, semantic, filters, limitArg)
      setResults(response.results)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to search evaluations'
      setError(message)
      setResults([])
    } finally {
      setLoading(false)
    }
  }, [])

  const loadMore = useCallback(() => {
    if (!currentQuery || loading) return
    search(currentQuery, true, undefined, limit + 10)
  }, [currentQuery, loading, limit, search])

  const hasMore = results.length >= limit

  return { results, loading, error, search, hasMore, loadMore }
}

interface UseStatsReturn {
  stats: {
    totalEvaluations: number
    avgValueScore: number
    medianValueScore: number
    providerUsage: Record<string, number>
    typeDistribution: Record<string, number>
    implementationRate: { total: number; implemented: number; inProgress: number; pending: number; rate: number }
    trend: { last7Days: number; last30Days: number; last90Days: number }
  } | null
  loading: boolean
  error: string | null
  refetch: () => Promise<void>
}

export function useEvaluationStats(projectId?: string, dateFrom?: string, dateTo?: string): UseStatsReturn {
  const [stats, setStats] = useState<UseStatsReturn['stats']>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const refetch = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const data = await apiClient.getEvaluationStats(projectId, dateFrom, dateTo)
      setStats(data)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch stats'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [projectId, dateFrom, dateTo])

  useEffect(() => {
    refetch()
  }, [refetch])

  return { stats, loading, error, refetch }
}

import { useEffect } from 'react'

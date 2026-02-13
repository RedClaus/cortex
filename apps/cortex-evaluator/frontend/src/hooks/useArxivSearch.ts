import { useState, useCallback, useEffect } from 'react'
import { apiClient, APIErrorImpl } from '../services/api'
import type { ArxivPaper } from '../types/api'

interface UseArxivSearchReturn {
  papers: ArxivPaper[]
  loading: boolean
  error: string | null
  search: (query: string, maxResults?: number, categories?: string[]) => Promise<void>
  getPaper: (paperId: string) => Promise<ArxivPaper>
  findSimilar: (query: string, limit?: number) => Promise<any[]>
}

export function useArxivSearch(): UseArxivSearchReturn {
  const [papers, setPapers] = useState<ArxivPaper[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const search = useCallback(async (query: string, maxResults = 10, categories?: string[]) => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiClient.searchArxiv(query, maxResults, categories)
      setPapers(response.papers)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to search arXiv'
      setError(message)
      setPapers([])
    } finally {
      setLoading(false)
    }
  }, [])

  const getPaper = useCallback(async (paperId: string): Promise<ArxivPaper> => {
    setLoading(true)
    setError(null)
    try {
      const paper = await apiClient.getArxivPaper(paperId)
      return paper
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch paper'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const findSimilar = useCallback(async (query: string, limit = 5): Promise<any[]> => {
    try {
      const response = await apiClient.findSimilarPapers(query, limit)
      return response.similarPapers
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to find similar papers'
      setError(message)
      return []
    }
  }, [])

  return { papers, loading, error, search, getPaper, findSimilar }
}

export function useArxivPaper(paperId?: string) {
  const [paper, setPaper] = useState<ArxivPaper | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    if (!paperId) return

    const fetchPaper = async () => {
      setLoading(true)
      setError(null)
      try {
        const data = await apiClient.getArxivPaper(paperId)
        setPaper(data)
      } catch (err) {
        const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch paper'
        setError(message)
      } finally {
        setLoading(false)
      }
    }

    fetchPaper()
  }, [paperId])

  return { paper, loading, error }
}

import { useState, useEffect, useCallback, useRef } from 'react'
import { apiClient, APIErrorImpl } from '../services/api'
import type {
  CodeFile,
  CodebaseInfo,
  CodebaseInitRequest,
  CodebaseInitResponse,
  SystemDocumentation,
  IndexingProgress
} from '../types/api'

interface UseCodebaseReturn {
  codebase: CodebaseInfo | null
  systemDocumentation: SystemDocumentation | null
  loading: boolean
  error: string | null
  indexingProgress: IndexingProgress
  initializeCodebase: (request: CodebaseInitRequest) => Promise<string>
  generateDocs: (codebaseId: string, includeTests?: boolean, maxFiles?: number) => Promise<void>
  deleteCodebase: (codebaseId: string) => Promise<void>
  refetch: () => Promise<void>
}

export function useCodebase(initialCodebaseId?: string): UseCodebaseReturn {
  const [codebase, setCodebase] = useState<CodebaseInfo | null>(null)
  const [systemDocumentation, setSystemDocumentation] = useState<SystemDocumentation | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [indexingProgress, setIndexingProgress] = useState<IndexingProgress>({
    isIndexing: false,
    totalFiles: 0,
    processedFiles: 0,
    currentFile: '',
    phase: 'idle'
  })
  const wsRef = useRef<WebSocket | null>(null)

  const connectWebSocket = useCallback((codebaseId: string) => {
    if (wsRef.current?.readyState === WebSocket.OPEN) {
      return
    }

    const wsUrl = `${import.meta.env.VITE_WS_URL || 'ws://localhost:8000'}/ws/codebase/${codebaseId}`
    wsRef.current = new WebSocket(wsUrl)

    wsRef.current.onopen = () => {
      console.log('WebSocket connected for codebase progress')
    }

    wsRef.current.onmessage = (event) => {
      const data = JSON.parse(event.data)
      setIndexingProgress(data)
    }

    wsRef.current.onerror = (error) => {
      console.error('WebSocket error:', error)
      setError('Connection error')
    }

    wsRef.current.onclose = () => {
      console.log('WebSocket disconnected')
    }
  }, [])

  const disconnectWebSocket = useCallback(() => {
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [])

  const fetchCodebase = useCallback(async (codebaseId: string) => {
    setLoading(true)
    setError(null)
    try {
      const data = await apiClient.getCodebase(codebaseId)
      setCodebase(data)
      connectWebSocket(codebaseId)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch codebase'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [connectWebSocket])

  const initializeCodebase = useCallback(async (request: CodebaseInitRequest): Promise<string> => {
    setLoading(true)
    setError(null)
    setIndexingProgress({ isIndexing: true, totalFiles: 0, processedFiles: 0, currentFile: '', phase: 'scanning' })

    try {
      const response: CodebaseInitResponse = await apiClient.initializeCodebase(request)
      connectWebSocket(response.codebaseId)
      
      if (response.status === 'indexing' || response.status === 'pending') {
        setIndexingProgress({
          isIndexing: true,
          totalFiles: response.fileCount,
          processedFiles: 0,
          currentFile: 'Initializing...',
          phase: 'scanning'
        })
      }

      return response.codebaseId
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to initialize codebase'
      setError(message)
      setIndexingProgress({ isIndexing: false, totalFiles: 0, processedFiles: 0, currentFile: '', phase: 'idle' })
      throw err
    } finally {
      setLoading(false)
    }
  }, [connectWebSocket])

  const generateDocs = useCallback(async (codebaseId: string, includeTests = false, maxFiles = 15) => {
    setLoading(true)
    setError(null)
    setIndexingProgress(prev => ({ ...prev, phase: 'documenting' }))

    try {
      const docs = await apiClient.generateSystemDocumentation(codebaseId, includeTests, maxFiles)
      setSystemDocumentation(docs)
      setIndexingProgress(prev => ({ ...prev, phase: 'vectorizing' }))

      setTimeout(() => {
        setIndexingProgress({ isIndexing: false, totalFiles: codebase?.fileCount || 0, processedFiles: codebase?.fileCount || 0, currentFile: '', phase: 'idle' })
      }, 1200)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to generate documentation'
      setError(message)
      setIndexingProgress({ isIndexing: false, totalFiles: 0, processedFiles: 0, currentFile: '', phase: 'idle' })
      throw err
    } finally {
      setLoading(false)
    }
  }, [codebase])

  const deleteCodebase = useCallback(async (codebaseId: string) => {
    setLoading(true)
    setError(null)
    try {
      await apiClient.deleteCodebase(codebaseId)
      setCodebase(null)
      setSystemDocumentation(null)
      disconnectWebSocket()
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to delete codebase'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [disconnectWebSocket])

  const refetch = useCallback(async () => {
    if (codebase?.id) {
      await fetchCodebase(codebase.id)
    }
  }, [codebase?.id, fetchCodebase])

  useEffect(() => {
    if (initialCodebaseId) {
      fetchCodebase(initialCodebaseId)
    }

    return () => {
      disconnectWebSocket()
    }
  }, [initialCodebaseId, fetchCodebase, disconnectWebSocket])

  return {
    codebase,
    systemDocumentation,
    loading,
    error,
    indexingProgress,
    initializeCodebase,
    generateDocs,
    deleteCodebase,
    refetch
  }
}

export function useCodebases(projectId?: string) {
  const [codebases, setCodebases] = useState<CodebaseInfo[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const fetchCodebases = useCallback(async () => {
    setLoading(true)
    setError(null)
    try {
      const response = await apiClient.listCodebases(projectId)
      setCodebases(response.codebases)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch codebases'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    fetchCodebases()
  }, [fetchCodebases])

  return { codebases, loading, error, refetch: fetchCodebases }
}

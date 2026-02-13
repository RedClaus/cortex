import { useState, useCallback, useEffect } from 'react'
import { apiClient, APIErrorImpl } from '../services/api'
import type { BrainstormSession, Node, Edge } from '../types/api'

interface UseBrainstormReturn {
  session: BrainstormSession | null
  sessions: BrainstormSession[]
  loading: boolean
  error: string | null
  createSession: (projectId: string, title: string) => Promise<string>
  updateSession: (sessionId: string, updates: Partial<BrainstormSession>) => Promise<void>
  deleteSession: (sessionId: string) => Promise<void>
  loadSession: (sessionId: string) => Promise<void>
  generateIdeas: (topic: string, constraints?: string[], providerPreference?: string) => Promise<unknown[]>
  expandIdea: (idea: string, context?: string) => Promise<{
    title: string
    description: string
    considerations: string[]
    nextSteps: string[]
  }>
  evaluateIdeas: (ideas: string[], criteria?: string[]) => Promise<unknown[]>
  connectIdeas: (ideaA: string, ideaB: string, relationship?: string) => Promise<unknown>
  refetchSessions: () => Promise<void>
}

export function useBrainstorm(sessionId?: string): UseBrainstormReturn {
  const [session, setSession] = useState<BrainstormSession | null>(null)
  const [sessions, setSessions] = useState<BrainstormSession[]>([])
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [projectId, setProjectId] = useState<string | undefined>()

  const createSession = useCallback(async (newProjectId: string, title: string): Promise<string> => {
    setLoading(true)
    setError(null)
    setProjectId(newProjectId)

    try {
      const newSession = await apiClient.createBrainstormSession(newProjectId, title)
      setSession(newSession)
      return newSession.id
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to create session'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const updateSession = useCallback(async (id: string, updates: Partial<BrainstormSession>) => {
    setLoading(true)
    setError(null)

    try {
      const updated = await apiClient.updateBrainstormSession(id, updates)
      setSession(updated)
      setSessions(prev => prev.map(s => s.id === id ? updated : s))
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to update session'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const deleteSession = useCallback(async (id: string) => {
    setLoading(true)
    setError(null)

    try {
      await apiClient.deleteBrainstormSession(id)
      setSession(prev => prev?.id === id ? null : prev)
      setSessions(prev => prev.filter(s => s.id !== id))
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to delete session'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const loadSession = useCallback(async (id: string) => {
    setLoading(true)
    setError(null)

    try {
      const data = await apiClient.getBrainstormSession(id)
      setSession(data)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to load session'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const generateIdeas = useCallback(async (topic: string, constraints?: string[], providerPreference?: string): Promise<unknown[]> => {
    setLoading(true)
    setError(null)

    try {
      const response = await apiClient.generateBrainstormIdeas(topic, constraints, providerPreference)
      return response.ideas
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to generate ideas'
      setError(message)
      return []
    } finally {
      setLoading(false)
    }
  }, [])

  const expandIdea = useCallback(async (idea: string, context?: string): Promise<{
    title: string
    description: string
    considerations: string[]
    nextSteps: string[]
  }> => {
    setLoading(true)
    setError(null)

    try {
      return await apiClient.expandIdea(idea, context)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to expand idea'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const evaluateIdeas = useCallback(async (ideas: string[], criteria?: string[]): Promise<unknown[]> => {
    setLoading(true)
    setError(null)

    try {
      const response = await apiClient.evaluateIdeas(ideas, criteria)
      return response.ideas
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to evaluate ideas'
      setError(message)
      return []
    } finally {
      setLoading(false)
    }
  }, [])

  const connectIdeas = useCallback(async (ideaA: string, ideaB: string, relationship = 'related'): Promise<unknown> => {
    setLoading(true)
    setError(null)

    try {
      return await apiClient.connectIdeas(ideaA, ideaB, relationship)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to connect ideas'
      setError(message)
      throw err
    } finally {
      setLoading(false)
    }
  }, [])

  const refetchSessions = useCallback(async () => {
    setLoading(true)
    setError(null)

    try {
      const response = await apiClient.listBrainstormSessions(projectId)
      setSessions(response.sessions)
    } catch (err) {
      const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch sessions'
      setError(message)
    } finally {
      setLoading(false)
    }
  }, [projectId])

  useEffect(() => {
    if (sessionId) {
      loadSession(sessionId)
    }
  }, [sessionId, loadSession])

  useEffect(() => {
    if (projectId) {
      refetchSessions()
    }
  }, [projectId, refetchSessions])

  return {
    session,
    sessions,
    loading,
    error,
    createSession,
    updateSession,
    deleteSession,
    loadSession,
    generateIdeas,
    expandIdea,
    evaluateIdeas,
    connectIdeas,
    refetchSessions
  }
}

export function useBrainstormTemplates() {
  const [templates, setTemplates] = useState<Record<string, { name: string; description: string; initialNodes: unknown[] }>>({})
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const fetchTemplates = async () => {
      setLoading(true)
      setError(null)

      try {
        const data = await apiClient.generateBrainstormIdeas('templates', [], '')
        setTemplates(data as any)
      } catch (err) {
        const message = err instanceof APIErrorImpl ? err.message : 'Failed to fetch templates'
        setError(message)
      } finally {
        setLoading(false)
      }
    }

    fetchTemplates()
  }, [])

  return { templates, loading, error }
}

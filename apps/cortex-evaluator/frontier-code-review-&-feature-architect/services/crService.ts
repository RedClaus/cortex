import { apiClient } from './api'

export interface BrainstormCR {
  codebaseId: string
  ideaId: string
  problem: string
  solutions: string[]
  feasibility: number
  impact: number
  effort: number
  risk: number
  implementationSteps: string[]
  dependencies: string[]
  suggestedCR: string
  estimatedHours: number
  priority: 'high' | 'medium' | 'low'
}

export async function generateCRBreakdown(
  codebaseId: string,
  idea: string,
  context?: string
): Promise<BrainstormCR> {
  try {
    const response = await apiClient.expandIdea(idea, context)

    return {
      codebaseId,
      ideaId: Date.now().toString(),
      problem: idea,
      solutions: [],
      feasibility: 75,
      impact: 80,
      effort: 60,
      risk: 40,
      implementationSteps: response.nextSteps,
      dependencies: [],
      suggestedCR: response.description,
      estimatedHours: 24,
      priority: 'medium'
    }
  } catch (error) {
    console.error('CR breakdown error:', error)
    throw error
  }
}

export async function generateDetailedCR(
  analysisResult: Record<string, unknown>
): Promise<BrainstormCR> {
  try {
    const response = await apiClient.expandIdea(
      analysisResult.suggestedCR?.toString() || 'Analysis Result',
      JSON.stringify(analysisResult, null, 2)
    )

    return {
      codebaseId: analysisResult.codebaseId?.toString() || '',
      ideaId: Date.now().toString(),
      problem: analysisResult.executiveSummary?.toString() || 'Analysis',
      solutions: [],
      feasibility: analysisResult.valueScore as number || 75,
      impact: 80,
      effort: 60,
      risk: 40,
      implementationSteps: response.nextSteps,
      dependencies: [],
      suggestedCR: response.description,
      estimatedHours: 24,
      priority: analysisResult.valueScore as number > 80 ? 'high' : analysisResult.valueScore as number > 50 ? 'medium' : 'low'
    }
  } catch (error) {
    console.error('Detailed CR generation error:', error)
    throw error
  }
}

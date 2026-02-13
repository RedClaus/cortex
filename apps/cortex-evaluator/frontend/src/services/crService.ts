import { DetailedCR } from '../types/cr';

export interface BreakdownRequest {
  executive_summary: string;
  suggested_cr: string;
  context?: string;
}

export interface CreateIssueRequest {
  platform: 'github' | 'jira' | 'linear';
  title: string;
  body: string;
  metadata?: {
    labels?: string[];
    milestone?: number;
    project?: string;
    cycle?: string;
    priority?: string;
    story_points?: number;
  };
}

export interface CreateIssueResponse {
  url: string;
  id: string;
  status: string;
}

export async function generateBreakdown(request: BreakdownRequest): Promise<DetailedCR> {
  const response = await fetch('/api/brainstorm/breakdown', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Failed to generate breakdown: ${error}`);
  }

  return await response.json();
}

export async function createIssue(request: CreateIssueRequest): Promise<CreateIssueResponse> {
  const response = await fetch('/api/integrations/issues', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(request),
  });

  if (!response.ok) {
    const error = await response.text();
    throw new Error(`Failed to create issue: ${error}`);
  }

  return await response.json();
}

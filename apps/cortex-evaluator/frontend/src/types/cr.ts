export interface CRTemplate {
  id: string;
  name: string;
  format: 'markdown' | 'jira' | 'linear' | 'github' | 'custom';
  description: string;
  sections: CRSection[];
}

export interface CRSection {
  id: string;
  title: string;
  required: boolean;
  placeholder?: string;
}

export interface DetailedCR {
  summary: string;
  type: 'feature' | 'refactor' | 'bugfix' | 'research';

  tasks: CRTask[];
  estimation: CREstimation;
  dependencies: CRDependency[];
  riskFactors: CRRisk[];
  testingRequirements: string[];
  documentationNeeds: string[];

  template_id: string;
  formatted_output: string;
}

export interface CRTask {
  id: string;
  title: string;
  description: string;
  acceptance_criteria: string[];
  estimate_hours?: number;
  priority: 'critical' | 'high' | 'medium' | 'low';
  status: 'pending' | 'in-progress' | 'completed';
}

export interface CREstimation {
  optimistic: { value: number; unit: string };
  expected: { value: number; unit: string };
  pessimistic: { value: number; unit: string };
  complexity: 1 | 2 | 3 | 5 | 8 | 13;
}

export interface CRDependency {
  id: string;
  title: string;
  type: 'internal' | 'external' | 'blocking';
  description: string;
}

export interface CRRisk {
  id: string;
  title: string;
  description: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  mitigation: string;
}

import { describe, it, expect, beforeEach } from 'vitest';
import { formatCR } from '../services/crFormatter';
import { CR_TEMPLATES } from '../data/crTemplates';

describe('CR Formatter', () => {
  const sampleCR = {
    summary: 'Add error handling to API endpoints',
    type: 'feature',
    tasks: [
      {
        id: 'task-1',
        title: 'Implement try-catch blocks',
        description: 'Wrap all API calls in try-catch for error handling',
        priority: 'high',
        acceptance_criteria: ['Try-catch implemented', 'Errors are logged']
      },
      {
        id: 'task-2',
        title: 'Add logging',
        description: 'Log all errors with context',
        priority: 'medium',
        acceptance_criteria: ['Logging configured', 'Error messages structured']
      }
    ],
    dependencies: [
      { title: 'Logger setup', description: 'Configure logging framework' }
    ],
    risk_factors: [
      {
        title: 'Breaking changes',
        description: 'Could affect existing clients',
        severity: 'medium',
        mitigation: 'Maintain backwards compatibility'
      }
    ],
    testingRequirements: [
      'Unit tests for error handling',
      'Integration tests for logging'
    ],
    documentationNeeds: [
      'Update API documentation',
      'Add error handling examples'
    ],
    estimation: {
      complexity: 'medium',
      optimistic: { value: 2, unit: 'days' },
      expected: { value: 3, unit: 'days' },
      pessimistic: { value: 5, unit: 'days' }
    }
  };

  beforeEach(() => {
    vi.useFakeTimers();
  });

  it('should format CR for Jira template', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'jira');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('h1. Add error handling to API endpoints');
    expect(result).toContain('*Type:* feature');
    expect(result).toContain('*Priority:* High');
    expect(result).toContain('h2. Description');
    expect(result).toContain('h2. Acceptance Criteria');
    expect(result).toContain('- [ ] Try-catch implemented');
  });

  it('should format CR for GitHub template', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'github');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('# Add error handling to API endpoints');
    expect(result).toContain('**Type:** `feature`');
    expect(result).toContain('**Complexity:** medium');
    expect(result).toContain('## Summary');
    expect(result).toContain('## Tasks');
    expect(result).toContain('### 1. Implement try-catch blocks');
  });

  it('should format CR for Linear template', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'linear');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('# Add error handling to API endpoints');
    expect(result).toContain('**Type:** feature');
    expect(result).toContain('**Priority:** High');
    expect(result).toContain('## Description');
    expect(result).toContain('## Tasks');
    expect(result).toContain('**1. Implement try-catch blocks** (high)');
  });

  it('should format CR for Markdown template', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'markdown');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('# Add error handling to API endpoints');
    expect(result).toContain('**Type:** feature | **Complexity:** medium (3 days)');
    expect(result).toContain('## Summary');
    expect(result).toContain('## Implementation Plan');
    expect(result).toContain('### 1. Implement try-catch blocks');
    expect(result).toContain('**Priority:** high');
  });

  it('should include all tasks in formatted output', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'markdown');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('Implement try-catch blocks');
    expect(result).toContain('Add logging');
  });

  it('should include dependencies when present', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'github');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('## Dependencies');
    expect(result).toContain('- Logger setup: Configure logging framework');
  });

  it('should include risk factors when present', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'github');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('## Risk Factors');
    expect(result).toContain('⚠️ **Breaking changes:** Could affect existing clients (Mitigation: Maintain backwards compatibility)');
  });

  it('should include testing requirements when present', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'markdown');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('## Testing');
    expect(result).toContain('- Unit tests for error handling');
    expect(result).toContain('- Integration tests for logging');
  });

  it('should include documentation needs when present', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'markdown');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('## Documentation');
    expect(result).toContain('- [ ] Update API documentation');
    expect(result).toContain('- [ ] Add error handling examples');
  });

  it('should include estimation details', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'github');

    if (!template) {
      throw new Error('Template not found');
    }

    const result = formatCR(sampleCR, template);

    expect(result).toContain('Story Points: medium');
    expect(result).toContain('Estimated: 3 days');
  });

  it('should handle CR with empty tasks', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'markdown');

    if (!template) {
      throw new Error('Template not found');
    }

    const crWithEmptyTasks = { ...sampleCR, tasks: [] };
    const result = formatCR(crWithEmptyTasks, template);

    expect(result).toContain('# Add error handling to API endpoints');
  });

  it('should handle CR with no dependencies', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'github');

    if (!template) {
      throw new Error('Template not found');
    }

    const crWithNoDeps = { ...sampleCR, dependencies: [] };
    const result = formatCR(crWithNoDeps, template);

    expect(result).toContain('No dependencies');
  });

  it('should handle CR with no risk factors', () => {
    const template = CR_TEMPLATES.find(t => t.format === 'markdown');

    if (!template) {
      throw new Error('Template not found');
    }

    const crWithNoRisks = { ...sampleCR, risk_factors: [] };
    const result = formatCR(crWithNoRisks, template);

    expect(result).toContain('None');
  });
});

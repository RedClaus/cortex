import { DetailedCR, CRTemplate } from '../types/cr';

export function formatCR(cr: DetailedCR, template: CRTemplate): string {
  switch (template.format) {
    case 'jira':
      return formatJira(cr, template);
    case 'github':
      return formatGitHub(cr, template);
    case 'linear':
      return formatLinear(cr, template);
    case 'markdown':
    default:
      return formatMarkdown(cr, template);
  }
}

function formatJira(cr: DetailedCR, template: CRTemplate): string {
  const priority = cr.tasks[0]?.priority || 'medium';
  const priorityValue = {
    'critical': 'Highest',
    'high': 'High',
    'medium': 'Medium',
    'low': 'Low'
  }[priority];

  let output = `h1. ${cr.summary}

*Type:* ${cr.type}
*Priority:* ${priorityValue}
*Story Points:* ${cr.estimation.complexity}

h2. Description
${cr.summary}

h2. Acceptance Criteria
${cr.tasks.map(t => `- [ ] ${t.title}`).join('\n')}

h2. Tasks`;

  cr.tasks.forEach((t, i) => {
    output += `

*Task ${i + 1}: ${t.title}*
${t.description}
*Acceptance:*
${t.acceptance_criteria.map(ac => `  - [ ] ${ac}`).join('\n')}
`;
  });

  if (cr.dependencies.length > 0) {
    output += `

h2. Dependencies
${cr.dependencies.map(d => `- ${d.title}: ${d.description}`).join('\n')}
`;
  }

  if (cr.riskFactors.length > 0) {
    output += `

h2. Risk Factors
${cr.riskFactors.map(r => `- ${r.title}: ${r.description} (Mitigation: ${r.mitigation})`).join('\n')}
`;
  }

  if (cr.testingRequirements.length > 0) {
    output += `

h2. Testing
${cr.testingRequirements.map(t => `- ${t}`).join('\n')}
`;
  }

  return output.trim();
}

function formatGitHub(cr: DetailedCR, template: CRTemplate): string {
  const labels = [cr.type, cr.tasks[0]?.priority].filter(Boolean);
  
  let output = `# ${cr.summary}

**Type:** \`${cr.type}\`
**Complexity:** ${cr.estimation.complexity}
**Estimated:** ${cr.estimation.expected.value} ${cr.estimation.expected.unit}
**Labels:** ${labels.map(l => `\`${l}\``).join(' ')}

## Summary
${cr.summary}

## Tasks
${cr.tasks.map((t, i) => `
### ${i + 1}. ${t.title}
**Priority:** \`${t.priority}\`

${t.description}

**Acceptance Criteria:**
${t.acceptance_criteria.map(ac => `- [ ] ${ac}`).join('\n')}
`).join('\n')}

## Dependencies
${cr.dependencies.length > 0 
  ? cr.dependencies.map(d => `- [ ] ${d.title}: ${d.description}`).join('\n')
  : 'No dependencies'}
`;

  if (cr.riskFactors.length > 0) {
    output += `

## Risk Factors
${cr.riskFactors.map(r => `- ⚠️ **${r.title}:** ${r.description} (Mitigation: ${r.mitigation})`).join('\n')}
`;
  }

  if (cr.testingRequirements.length > 0) {
    output += `

## Testing
${cr.testingRequirements.map(t => `- ${t}`).join('\n')}
`;
  }

  if (cr.documentationNeeds.length > 0) {
    output += `

## Documentation
${cr.documentationNeeds.map(d => `- [ ] ${d}`).join('\n')}
`;
  }

  return output.trim();
}

function formatLinear(cr: DetailedCR, template: CRTemplate): string {
  const priorityValue = {
    'critical': 'Urgent',
    'high': 'High',
    'medium': 'Medium',
    'low': 'Low'
  }[cr.tasks[0]?.priority || 'medium'];

  let output = `# ${cr.summary}

**Type:** ${cr.type}
**Priority:** ${priorityValue}
**Complexity:** ${cr.estimation.complexity}

## Description
${cr.summary}

## Tasks
${cr.tasks.map((t, i) => `
**${i + 1}. ${t.title}** (${t.priority})

${t.description}

Acceptance:
${t.acceptance_criteria.map(ac => `  - [ ] ${ac}`).join('\n')}
`).join('\n')}

## Dependencies
${cr.dependencies.length > 0 
  ? cr.dependencies.map(d => `- ${d.title}: ${d.description}`).join('\n')
  : 'None'}
`;

  if (cr.riskFactors.length > 0) {
    output += `

## Risks
${cr.riskFactors.map(r => `- ${r.title}: ${r.description} (${r.severity})`).join('\n')}
`;
  }

  return output.trim();
}

function formatMarkdown(cr: DetailedCR, template: CRTemplate): string {
  let output = `# ${cr.summary}

**Type:** ${cr.type} | **Complexity:** ${cr.estimation.complexity} (${cr.estimation.expected.value} ${cr.estimation.expected.unit})

## Summary
${cr.summary}

## Implementation Plan

${cr.tasks.map((t, i) => `
### ${i + 1}. ${t.title}

**Priority:** ${t.priority}

${t.description}

**Acceptance Criteria:**
${t.acceptance_criteria.map(ac => `- [ ] ${ac}`).join('\n')}
`).join('\n')}

## Dependencies
${cr.dependencies.map(d => `- [ ] ${d.description}`).join('\n') || 'None'}

## Risk Factors
${cr.riskFactors.map(r => `- ⚠️ **${r.title}:** ${r.description} (${r.mitigation})`).join('\n') || 'None'}

## Testing
${cr.testingRequirements.map(t => `- ${t}`).join('\n') || 'None'}

## Documentation
${cr.documentationNeeds.map(d => `- [ ] ${d}`).join('\n') || 'None'}
`;

  return output.trim();
}

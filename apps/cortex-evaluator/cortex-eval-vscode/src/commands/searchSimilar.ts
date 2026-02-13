import * as vscode from 'vscode';
import { apiClient } from '../api/client';

export async function searchSimilarCommand(): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showWarningMessage('No active editor. Searching without context...');
  }

  let query = '';

  if (editor) {
    const selection = editor.selection;
    const selectedText = editor.document.getText(selection);

    if (selectedText && selectedText.trim().length > 0) {
      query = selectedText;
    } else {
      query = editor.document.getText();
    }
  }

  if (!query || query.trim().length === 0) {
    const input = await vscode.window.showInputBox({
      placeHolder: 'Enter search query',
      prompt: 'Search for similar evaluations'
    });

    if (!input) {
      return;
    }

    query = input;
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Searching similar evaluations...',
      cancellable: false
    },
    async () => {
      const result = await apiClient.searchEvaluations(query, true, {}, 10);

      if (result.results.length === 0) {
        vscode.window.showInformationMessage('No similar evaluations found');
        return;
      }

      const items = result.results.map(r => ({
        label: r.id,
        description: `Score: ${r.score.toFixed(2)}`,
        detail: r.metadata ? JSON.stringify(r.metadata, null, 2).slice(0, 100) : undefined,
        data: r
      }));

      const selected = await vscode.window.showQuickPick(items, {
        placeHolder: `Found ${result.count} similar evaluations`,
        matchOnDetail: true
      });

      if (selected) {
        const evaluation = await apiClient.getEvaluation(selected.data.id);

        const content = `# Evaluation: ${evaluation.id}

**Created:** ${evaluation.created_at}
**Provider:** ${evaluation.provider_id}
**Input Type:** ${evaluation.input_type}

---

## Result

${evaluation.result ? `
**Value Score:** ${evaluation.result.valueScore}/100

### Executive Summary
${evaluation.result.executiveSummary}

### Technical Feasibility
${evaluation.result.technicalFeasibility}

### Gap Analysis
${evaluation.result.gapAnalysis}

### Change Request
${evaluation.result.suggestedCR}
` : 'No result available'}
`;

        const doc = await vscode.workspace.openTextDocument({
          language: 'markdown',
          content
        });
        await vscode.window.showTextDocument(doc);
      }
    }
  );
}

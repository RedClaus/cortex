import * as vscode from 'vscode';
import { SidePanelProvider } from '../webview/SidePanelProvider';
import { apiClient } from '../api/client';

export async function analyzeSelectionCommand(sidePanelProvider: SidePanelProvider): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No active editor found');
    return;
  }

  const selection = editor.selection;
  const selectedText = editor.document.getText(selection);

  if (!selectedText || selectedText.trim().length === 0) {
    vscode.window.showWarningMessage('No text selected');
    return;
  }

  const config = vscode.workspace.getConfiguration('cortexEval');
  const apiUrl = config.get<string>('apiUrl', 'http://localhost:8000');
  apiClient.setBaseURL(apiUrl);

  let codebaseId = config.get<string>('codebaseId');
  if (!codebaseId) {
    const codebases = await apiClient.listCodebases();

    if (codebases.codebases.length === 0) {
      vscode.window.showErrorMessage('No codebases found. Please create one in the web UI first.');
      return;
    }

    const items = codebases.codebases.map(cb => ({
      label: cb.name,
      description: `${cb.file_count} files`,
      detail: `ID: ${cb.id}`,
      id: cb.id
    }));

    const selected = await vscode.window.showQuickPick(items, {
      placeHolder: 'Select a codebase to analyze against'
    });

    if (!selected) {
      return;
    }

    await config.update('codebaseId', selected.id, vscode.ConfigurationTarget.Workspace);
    codebaseId = selected.id;
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Analyzing selection...',
      cancellable: false
    },
    async () => {
      try {
        const result = await apiClient.analyzeEvaluation({
          codebaseId,
          inputType: 'snippet',
          inputContent: selectedText,
        });

        sidePanelProvider.showResult(result);

        const action = await vscode.window.showQuickPick(
          ['Insert Below Selection', 'Open in New File', 'Show in Panel Only'],
          {
            placeHolder: 'What would you like to do with the result?'
          }
        );

        if (action === 'Insert Below Selection') {
          const position = selection.end;
          await editor.edit((editBuilder: vscode.TextEditorEdit) => {
            editBuilder.insert(
              position,
              '\n\n---\n\n## Analysis Result\n\n' +
              `**Value Score:** ${result.valueScore}/100\n\n` +
              `## Executive Summary\n${result.executiveSummary}\n\n` +
              `## Change Request\n${result.suggestedCR}`
            );
          });
        } else if (action === 'Open in New File') {
          const timestamp = new Date().toISOString().slice(0, 10);
          const filename = `analysis-${result.id}-${timestamp}.md`;
          const uri = vscode.Uri.joinPath(vscode.workspace.workspaceFolders![0].uri, filename);

          const content = `# Analysis Result

**Generated:** ${new Date().toISOString()}
**Value Score:** ${result.valueScore}/100
**Provider:** ${result.providerUsed}

---

## Executive Summary

${result.executiveSummary}

## Technical Feasibility

${result.technicalFeasibility}

## Gap Analysis

${result.gapAnalysis}

## Change Request

${result.suggestedCR}
`;

          await vscode.workspace.fs.writeFile(uri, Buffer.from(content, 'utf8'));
          const doc = await vscode.workspace.openTextDocument(uri);
          await vscode.window.showTextDocument(doc);
        }

        vscode.window.showInformationMessage(`Analysis complete! Value Score: ${result.valueScore}/100`);
      } catch (error) {
        throw error;
      }
    }
  );
}

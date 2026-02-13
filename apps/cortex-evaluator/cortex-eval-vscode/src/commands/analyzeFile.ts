import * as vscode from 'vscode';
import { SidePanelProvider } from '../webview/SidePanelProvider';
import { apiClient } from '../api/client';

export async function analyzeFileCommand(sidePanelProvider: SidePanelProvider): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No active editor found');
    return;
  }

  const config = vscode.workspace.getConfiguration('cortexEval');
  const apiUrl = config.get<string>('apiUrl', 'http://localhost:8000');
  apiClient.setBaseURL(apiUrl);

  const codebaseId = await getCodebaseId(config);
  if (!codebaseId) {
    return;
  }

  const document = editor.document;
  const fileName = document.fileName;
  const content = document.getText();
  const languageId = document.languageId;

  if (!content || content.trim().length === 0) {
    vscode.window.showWarningMessage('File is empty');
    return;
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: 'Analyzing file...',
      cancellable: false
    },
    async () => {
      try {
        const result = await apiClient.analyzeEvaluation({
          codebaseId,
          inputType: 'snippet',
          inputContent: `File: ${fileName}\nLanguage: ${languageId}\n\n${content}`,
        });

        sidePanelProvider.showResult(result);
        vscode.window.showInformationMessage(`Analysis complete! Value Score: ${result.valueScore}/100`);
      } catch (error) {
        throw error;
      }
    }
  );
}

async function getCodebaseId(config: vscode.WorkspaceConfiguration): Promise<string | undefined> {
  let codebaseId = config.get<string>('codebaseId');

  if (!codebaseId) {
    const codebases = await apiClient.listCodebases();

    if (codebases.codebases.length === 0) {
      vscode.window.showErrorMessage('No codebases found. Please create one in the web UI first.');
      return undefined;
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

    if (selected) {
      await config.update('codebaseId', selected.id, vscode.ConfigurationTarget.Workspace);
      codebaseId = selected.id;
    }
  }

  return codebaseId;
}

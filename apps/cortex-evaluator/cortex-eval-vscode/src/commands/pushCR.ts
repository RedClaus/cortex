import * as vscode from 'vscode';
import { apiClient } from '../api/client';

export async function pushCRCommand(): Promise<void> {
  const editor = vscode.window.activeTextEditor;
  if (!editor) {
    vscode.window.showErrorMessage('No active editor found');
    return;
  }

  const document = editor.document;
  const content = document.getText();

  const config = vscode.workspace.getConfiguration('cortexEval');
  const apiUrl = config.get<string>('apiUrl', 'http://localhost:8000');
  apiClient.setBaseURL(apiUrl);

  let title = '';
  let body = '';

  const titleMatch = content.match(/^#\s+(.+)$/m);
  if (titleMatch) {
    title = titleMatch[1].trim();
  }

  body = content;

  if (!title) {
    const input = await vscode.window.showInputBox({
      placeHolder: 'Enter issue title',
      prompt: 'What should be the title of the GitHub issue?'
    });

    if (!input) {
      return;
    }

    title = input;
  }

  const platform = await vscode.window.showQuickPick(
    ['GitHub', 'Jira', 'Linear'],
    {
      placeHolder: 'Select platform'
    }
  );

  if (!platform) {
    return;
  }

  let metadata: Record<string, unknown> = {};

  if (platform === 'GitHub') {
    const labelsInput = await vscode.window.showInputBox({
      placeHolder: 'bug, enhancement, documentation',
      prompt: 'Enter labels (comma-separated)'
    });

    if (labelsInput) {
      metadata.labels = labelsInput.split(',').map((l: string) => l.trim());
    }
  } else if (platform === 'Jira') {
    const priority = await vscode.window.showQuickPick(
      ['Highest', 'High', 'Medium', 'Low', 'Lowest'],
      { placeHolder: 'Select priority' }
    );

    if (priority) {
      metadata.priority = priority;
    }
  }

  await vscode.window.withProgress(
    {
      location: vscode.ProgressLocation.Notification,
      title: `Creating ${platform} issue...`,
      cancellable: false
    },
    async () => {
      try {
        const result = await apiClient.createIssue({
          platform: platform.toLowerCase(),
          title,
          body,
          metadata
        });

        const openAction = await vscode.window.showInformationMessage(
          `Issue created successfully!`,
          'Open in Browser'
        );

        if (openAction === 'Open in Browser') {
          await vscode.env.openExternal(vscode.Uri.parse(result.url));
        }
      } catch (error) {
        throw error;
      }
    }
  );
}

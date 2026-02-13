import * as vscode from 'vscode';

export async function openWorkspaceCommand(): Promise<void> {
  const workspaceFolders = vscode.workspace.workspaceFolders;
  if (!workspaceFolders) {
    vscode.window.showErrorMessage('No workspace folder open');
    return;
  }

  const config = vscode.workspace.getConfiguration('cortexEval');
  const apiUrl = config.get<string>('apiUrl', 'http://localhost:8000');
  const projectId = config.get<string>('projectId');

  if (!projectId) {
    const input = await vscode.window.showInputBox({
      placeHolder: 'Enter project ID',
      prompt: 'Enter the project ID from the web UI'
    });

    if (!input) {
      return;
    }

    await config.update('projectId', input, vscode.ConfigurationTarget.Workspace);
  }

  const finalProjectId = projectId || (await config.get<string>('projectId'));

  if (!finalProjectId) {
    return;
  }

  const url = `${apiUrl}/projects/${finalProjectId}`;

  const openAction = await vscode.window.showInformationMessage(
    `Opening workspace in browser...`,
    'Copy URL'
  );

  if (openAction === 'Copy URL') {
    await vscode.env.clipboard.writeText(url);
    vscode.window.showInformationMessage('URL copied to clipboard');
  } else {
    await vscode.env.openExternal(vscode.Uri.parse(url));
  }
}

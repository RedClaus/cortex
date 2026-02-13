import * as vscode from 'vscode';
import { analyzeFileCommand } from './commands/analyzeFile';
import { analyzeSelectionCommand } from './commands/analyzeSelection';
import { searchSimilarCommand } from './commands/searchSimilar';
import { pushCRCommand } from './commands/pushCR';
import { openWorkspaceCommand } from './commands/openWorkspace';
import { SidePanelProvider } from './webview/SidePanelProvider';
import { apiClient } from './api/client';

export function activate(context: vscode.ExtensionContext) {
  console.log('Cortex Evaluator extension is now active');

  const sidePanelProvider = new SidePanelProvider(context.extensionUri);

  context.subscriptions.push(
    vscode.window.registerWebviewViewProvider('cortexEval.sidePanel', sidePanelProvider)
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex-eval.analyzeFile', async () => {
      try {
        await analyzeFileCommand(sidePanelProvider);
      } catch (error) {
        vscode.window.showErrorMessage(`Analysis failed: ${(error as Error).message}`);
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex-eval.analyzeSelection', async () => {
      try {
        await analyzeSelectionCommand(sidePanelProvider);
      } catch (error) {
        vscode.window.showErrorMessage(`Analysis failed: ${(error as Error).message}`);
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex-eval.searchSimilar', async () => {
      try {
        await searchSimilarCommand();
      } catch (error) {
        vscode.window.showErrorMessage(`Search failed: ${(error as Error).message}`);
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex-eval.pushCR', async () => {
      try {
        await pushCRCommand();
      } catch (error) {
        vscode.window.showErrorMessage(`Push failed: ${(error as Error).message}`);
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex-eval.openWorkspace', async () => {
      try {
        await openWorkspaceCommand();
      } catch (error) {
        vscode.window.showErrorMessage(`Failed to open workspace: ${(error as Error).message}`);
      }
    })
  );

  context.subscriptions.push(
    vscode.commands.registerCommand('cortex-eval.showSidePanel', () => {
      vscode.commands.executeCommand('cortexEval.sidePanel.focus');
    })
  );

  const statusBarItem = vscode.window.createStatusBarItem(vscode.StatusBarAlignment.Right, 100);
  statusBarItem.text = '$(pulse) Cortex';
  statusBarItem.command = 'cortex-eval.showSidePanel';
  statusBarItem.show();
  context.subscriptions.push(statusBarItem);

  checkConnection(statusBarItem);
}

async function checkConnection(statusBarItem: vscode.StatusBarItem) {
  try {
    const config = vscode.workspace.getConfiguration('cortexEval');
    const apiUrl = config.get<string>('apiUrl', 'http://localhost:8000');
    apiClient.setBaseURL(apiUrl);

    await apiClient.healthCheck();
    statusBarItem.text = '$(check) Cortex';
    statusBarItem.tooltip = 'Connected to Cortex Evaluator';
  } catch (error) {
    statusBarItem.text = '$(error) Cortex';
    statusBarItem.tooltip = 'Failed to connect to Cortex Evaluator';
  }
}

export function deactivate() {
  console.log('Cortex Evaluator extension is now deactivated');
}

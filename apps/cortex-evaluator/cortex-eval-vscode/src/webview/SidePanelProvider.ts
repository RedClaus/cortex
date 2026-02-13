import * as vscode from 'vscode';
import * as path from 'path';
import { apiClient } from '../api/client';

export interface AnalysisResult {
  id: string;
  valueScore: number;
  executiveSummary: string;
  technicalFeasibility: string;
  gapAnalysis: string;
  suggestedCR: string;
  providerUsed: string;
  similarEvaluations?: Array<{ id: string; score: number }>;
}

export class SidePanelProvider {
  private _view?: vscode.WebviewView;
  private _currentResult: AnalysisResult | null = null;

  constructor(private readonly _extensionUri: vscode.Uri) {}

  public resolveWebviewView(
    webviewView: vscode.WebviewView
  ): void | Promise<void> {
    this._view = webviewView;

    webviewView.webview.options = {
      enableScripts: true,
      localResourceRoots: [this._extensionUri]
    };

    webviewView.webview.html = this._getHtmlForWebview(webviewView.webview);

    webviewView.webview.onDidReceiveMessage((data: any) => {
      switch (data.type) {
        case 'insertCR':
          this._insertCR();
          break;
        case 'copyCR':
          vscode.env.clipboard.writeText(this._currentResult?.suggestedCR || '');
          vscode.window.showInformationMessage('CR copied to clipboard');
          break;
        case 'openFile':
          this._createCRFile();
          break;
        case 'refresh':
          if (this._currentResult) {
            this._updatePanel(this._currentResult);
          }
          break;
      }
    });
  }

  private _getHtmlForWebview(webview: vscode.Webview): string {
    const styleUri = webview.asWebviewUri(
      vscode.Uri.joinPath(this._extensionUri, 'media', 'style.css')
    );

    return `<!DOCTYPE html>
      <html lang="en">
      <head>
        <meta charset="UTF-8">
        <meta name="viewport" content="width=device-width, initial-scale=1.0">
        <meta http-equiv="Content-Security-Policy" content="default-src 'none'; style-src ${webview.cspSource} 'unsafe-inline'; script-src ${webview.cspSource} 'unsafe-inline';">
        <title>Cortex Evaluator</title>
        <link rel="stylesheet" href="${styleUri}">
      </head>
      <body>
        <div class="container">
          <div class="header">
            <h1>Cortex Evaluator</h1>
            <div id="status" class="status disconnected">Disconnected</div>
          </div>
          <div id="content">
            <div class="placeholder">
              <p>Select code and use "Analyze Selection" or "Analyze Current File" to get started.</p>
            </div>
          </div>
        </div>
        <script>
          const vscode = acquireVsCodeApi();

          function insertCR() {
            vscode.postMessage({ type: 'insertCR' });
          }

          function copyCR() {
            vscode.postMessage({ type: 'copyCR' });
          }

          function openFile() {
            vscode.postMessage({ type: 'openFile' });
          }

          function updateResult(result) {
            const content = document.getElementById('content');
            const status = document.getElementById('status');
            
            status.className = 'status connected';
            status.textContent = 'Connected';
            
            content.innerHTML = \`
              <div class="result">
                <div class="score-bar">
                  <div class="score-label">Value Score</div>
                  <div class="score-value">\${result.valueScore}/100</div>
                  <div class="score-bar-fill" style="width: \${result.valueScore}%"></div>
                </div>
                
                <div class="section">
                  <h2>Executive Summary</h2>
                  <div class="markdown">\${result.executiveSummary}</div>
                </div>
                
                <div class="section">
                  <h2>Technical Feasibility</h2>
                  <div class="markdown">\${result.technicalFeasibility}</div>
                </div>
                
                <div class="section">
                  <h2>Gap Analysis</h2>
                  <div class="markdown">\${result.gapAnalysis}</div>
                </div>
                
                <div class="section">
                  <h2>Change Request</h2>
                  <div class="markdown">\${result.suggestedCR}</div>
                </div>
                
                <div class="actions">
                  <button onclick="insertCR()">Insert Below</button>
                  <button onclick="copyCR()">Copy CR</button>
                  <button onclick="openFile()">Open in New File</button>
                </div>
                
                \${result.similarEvaluations && result.similarEvaluations.length > 0 ? \`
                  <div class="section">
                    <h2>Similar Evaluations</h2>
                    <ul class="similar-list">
                      \${result.similarEvaluations.map(e => \`
                        <li>\${e.id} (score: \${e.score.toFixed(2)})</li>
                      \`).join('')}
                    </ul>
                  </div>
                \` : ''}
              </div>
            \`;
          }

          window.addEventListener('message', event => {
            const message = event.data;
            if (message.type === 'updateResult') {
              updateResult(message.result);
            }
          });
        </script>
      </body>
      </html>`;
  }

  public showResult(result: AnalysisResult): void {
    this._currentResult = result;
    this._updatePanel(result);
  }

  private _updatePanel(result: AnalysisResult): void {
    if (this._view) {
      this._view.webview.postMessage({ type: 'updateResult', result });
    }
  }

  private async _insertCR(): Promise<void> {
    const editor = vscode.window.activeTextEditor;
    if (!editor || !this._currentResult) {
      return;
    }

      const position = editor.selection.end;
      await editor.edit((editBuilder: vscode.TextEditorEdit) => {
      editBuilder.insert(position, '\n\n---\n\n## Change Request\n\n' + this._currentResult!.suggestedCR);
    });
  }

  private async _createCRFile(): Promise<void> {
    if (!this._currentResult) {
      return;
    }

    const timestamp = new Date().toISOString().slice(0, 10);
    const filename = `cr-${this._currentResult.id}-${timestamp}.md`;
    const uri = vscode.Uri.joinPath(vscode.workspace.workspaceFolders![0].uri, filename);

    const content = `# Change Request

**Generated:** ${new Date().toISOString()}
**Value Score:** ${this._currentResult.valueScore}/100
**Provider:** ${this._currentResult.providerUsed}

---

## Executive Summary

${this._currentResult.executiveSummary}

## Technical Feasibility

${this._currentResult.technicalFeasibility}

## Gap Analysis

${this._currentResult.gapAnalysis}

## Change Request

${this._currentResult.suggestedCR}
`;

    await vscode.workspace.fs.writeFile(uri, Buffer.from(content, 'utf8'));
    const doc = await vscode.workspace.openTextDocument(uri);
    await vscode.window.showTextDocument(doc);
  }
}

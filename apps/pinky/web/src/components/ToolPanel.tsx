import React from 'react';
import type { ToolCall } from '../types';
import './ToolPanel.css';

interface ToolPanelProps {
  toolCalls: ToolCall[];
}

export function ToolPanel({ toolCalls }: ToolPanelProps) {
  const hasActiveTool = toolCalls.some(tc => tc.status === 'running');

  return (
    <div className="tool-panel">
      <div className="panel-header">
        <span className={`indicator ${hasActiveTool ? 'active' : 'inactive'}`} />
        <span>Tool Execution</span>
        {toolCalls.length > 0 && (
          <span className="tool-count">{toolCalls.length}</span>
        )}
      </div>
      <div className="panel-content">
        {toolCalls.length === 0 ? (
          <div className="tool-empty">
            <div className="empty-terminal">
              <div className="terminal-header">
                <span className="terminal-dot red" />
                <span className="terminal-dot yellow" />
                <span className="terminal-dot green" />
              </div>
              <div className="terminal-body">
                <span className="terminal-prompt">$</span>
                <span className="terminal-cursor" />
              </div>
            </div>
            <p>No active tools</p>
            <span>Command output will appear here</span>
          </div>
        ) : (
          <div className="tool-history">
            {toolCalls.map((toolCall, index) => (
              <div
                key={toolCall.id}
                className={`tool-entry ${toolCall.status}`}
                style={{ animationDelay: `${index * 50}ms` }}
              >
                <div className="tool-entry-header">
                  <span className="tool-entry-icon">
                    {getToolIcon(toolCall.tool)}
                  </span>
                  <span className="tool-entry-name">{toolCall.tool}</span>
                  <span className={`tool-entry-status ${toolCall.status}`}>
                    {getStatusIcon(toolCall.status)}
                    {toolCall.status}
                  </span>
                </div>

                <div className="tool-entry-command">
                  <span className="command-prompt">$</span>
                  <code>{formatCommand(toolCall)}</code>
                </div>

                {toolCall.output && (
                  <div className="tool-entry-output">
                    <pre>{toolCall.output}</pre>
                  </div>
                )}

                {toolCall.error && (
                  <div className="tool-entry-error">
                    <pre>{toolCall.error}</pre>
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function getToolIcon(tool: string): React.ReactElement {
  switch (tool) {
    case 'shell':
      return (
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M4 17l6-6-6-6M12 19h8" />
        </svg>
      );
    case 'git':
      return (
        <svg width="12" height="12" viewBox="0 0 24 24" fill="currentColor">
          <path d="M23.546 10.93L13.067.452c-.604-.603-1.582-.603-2.188 0L8.708 2.627l2.76 2.76c.645-.215 1.379-.07 1.889.441.516.515.658 1.258.438 1.9l2.658 2.66c.645-.223 1.387-.078 1.9.435.721.72.721 1.884 0 2.604-.719.719-1.881.719-2.6 0-.539-.541-.674-1.337-.404-1.996L12.86 8.955v6.525c.176.086.342.203.488.348.713.721.713 1.883 0 2.6-.719.721-1.889.721-2.609 0-.719-.719-.719-1.879 0-2.598.182-.18.387-.316.605-.406V8.835c-.217-.091-.424-.222-.6-.401-.545-.545-.676-1.342-.396-2.009L7.636 3.7.45 10.881c-.6.605-.6 1.584 0 2.189l10.48 10.477c.604.604 1.582.604 2.186 0l10.43-10.43c.605-.603.605-1.582 0-2.187" />
        </svg>
      );
    case 'files':
      return (
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M22 19a2 2 0 01-2 2H4a2 2 0 01-2-2V5a2 2 0 012-2h5l2 3h9a2 2 0 012 2z" />
        </svg>
      );
    case 'web':
      return (
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="10" />
          <path d="M2 12h20M12 2a15.3 15.3 0 014 10 15.3 15.3 0 01-4 10 15.3 15.3 0 01-4-10 15.3 15.3 0 014-10z" />
        </svg>
      );
    default:
      return (
        <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="3" />
        </svg>
      );
  }
}

function getStatusIcon(status: ToolCall['status']): React.ReactElement | null {
  switch (status) {
    case 'completed':
      return (
        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
          <path d="M20 6L9 17l-5-5" />
        </svg>
      );
    case 'failed':
      return (
        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3">
          <path d="M18 6L6 18M6 6l12 12" />
        </svg>
      );
    case 'running':
      return <span className="status-spinner" />;
    default:
      return null;
  }
}

function formatCommand(toolCall: ToolCall): string {
  if (toolCall.tool === 'shell' && toolCall.input.command) {
    return String(toolCall.input.command);
  }
  return JSON.stringify(toolCall.input);
}

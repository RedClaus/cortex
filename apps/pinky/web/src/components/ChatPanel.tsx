import React, { useRef, useEffect } from 'react';
import type { Message, ToolCall } from '../types';
import './ChatPanel.css';

interface ChatPanelProps {
  messages: Message[];
  inputValue: string;
  isStreaming: boolean;
  pendingApproval: ToolCall | null;
  onInputChange: (value: string) => void;
  onSendMessage: (content: string) => void;
  onApprove: (toolCall: ToolCall) => void;
  onDeny: (toolCall: ToolCall) => void;
}

export function ChatPanel({
  messages,
  inputValue,
  isStreaming,
  pendingApproval,
  onInputChange,
  onSendMessage,
  onApprove,
  onDeny,
}: ChatPanelProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLTextAreaElement>(null);
  const wasStreamingRef = useRef(false);

  // Scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // Focus input on mount
  useEffect(() => {
    inputRef.current?.focus();
  }, []);

  // Refocus input after streaming completes
  useEffect(() => {
    if (wasStreamingRef.current && !isStreaming) {
      // Small delay to ensure DOM is ready
      setTimeout(() => {
        inputRef.current?.focus();
      }, 50);
    }
    wasStreamingRef.current = isStreaming;
  }, [isStreaming]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      onSendMessage(inputValue);
    }
  };

  return (
    <div className="chat-panel">
      <div className="messages-container">
        <div className="messages">
          {messages.map((message, index) => (
            <div
              key={message.id}
              className={`message ${message.role}`}
              style={{ animationDelay: `${index * 50}ms` }}
            >
              <div className="message-avatar">
                {message.role === 'user' ? (
                  <span className="avatar-user">You</span>
                ) : (
                  <span className="avatar-assistant">
                    <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                      <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="1.5" />
                      <circle cx="12" cy="12" r="4" fill="currentColor" />
                    </svg>
                  </span>
                )}
              </div>
              <div className="message-content">
                <div className="message-header">
                  <span className="message-author">
                    {message.role === 'user' ? 'You' : 'Pinky'}
                  </span>
                  <span className="message-time">
                    {formatTime(message.timestamp)}
                  </span>
                </div>
                <div className="message-body">
                  {formatContent(message.content)}
                </div>

                {/* Tool Calls */}
                {message.toolCalls?.map(toolCall => (
                  <div key={toolCall.id} className={`tool-call ${toolCall.status}`}>
                    <div className="tool-call-header">
                      <span className="tool-icon">
                        {getToolIcon(toolCall.tool)}
                      </span>
                      <span className="tool-name">{toolCall.tool}</span>
                      <span className={`tool-status ${toolCall.status}`}>
                        {toolCall.status}
                      </span>
                    </div>
                    <div className="tool-call-command">
                      <code>{JSON.stringify(toolCall.input)}</code>
                    </div>
                    {toolCall.output && (
                      <div className="tool-call-output">
                        <pre>{toolCall.output}</pre>
                      </div>
                    )}
                  </div>
                ))}
              </div>
            </div>
          ))}

          {isStreaming && (
            <div className="message assistant streaming">
              <div className="message-avatar">
                <span className="avatar-assistant">
                  <svg width="20" height="20" viewBox="0 0 24 24" fill="none">
                    <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="1.5" />
                    <circle cx="12" cy="12" r="4" fill="currentColor" />
                  </svg>
                </span>
              </div>
              <div className="message-content">
                <div className="message-header">
                  <span className="message-author">Pinky</span>
                </div>
                <div className="message-body">
                  <span className="typing-indicator">
                    <span></span>
                    <span></span>
                    <span></span>
                  </span>
                </div>
              </div>
            </div>
          )}

          <div ref={messagesEndRef} />
        </div>
      </div>

      {/* Approval Dialog */}
      {pendingApproval && (
        <div className="approval-dialog">
          <div className="approval-header">
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
            <span>Approval Required</span>
          </div>
          <div className="approval-content">
            <p>Pinky wants to execute:</p>
            <div className="approval-command">
              <code>{JSON.stringify(pendingApproval.input, null, 2)}</code>
            </div>
            <div className="approval-meta">
              <span className="approval-tool">Tool: {pendingApproval.tool}</span>
              <span className="approval-risk high">High Risk</span>
            </div>
          </div>
          <div className="approval-actions">
            <button className="btn-deny" onClick={() => onDeny(pendingApproval)}>
              Deny
            </button>
            <button className="btn-approve" onClick={() => onApprove(pendingApproval)}>
              Approve
            </button>
          </div>
        </div>
      )}

      {/* Input Area */}
      <div className="input-container">
        <div className="input-wrapper">
          <textarea
            ref={inputRef}
            value={inputValue}
            onChange={e => onInputChange(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Type a message... (Enter to send, Shift+Enter for new line)"
            className="chat-input"
            rows={1}
            disabled={isStreaming}
          />
          <button
            className="send-button"
            onClick={() => onSendMessage(inputValue)}
            disabled={!inputValue.trim() || isStreaming}
            aria-label="Send message"
          >
            <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M22 2L11 13M22 2l-7 20-4-9-9-4 20-7z" />
            </svg>
          </button>
        </div>
        <div className="input-hints">
          <span>Press <kbd>Enter</kbd> to send</span>
          <span><kbd>Shift+Enter</kbd> for new line</span>
        </div>
      </div>
    </div>
  );
}

function formatTime(date: Date): string {
  return date.toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
}

function formatContent(content: string): React.ReactElement[] {
  const parts = content.split(/(\*\*[^*]+\*\*|\n|• [^\n]+)/g);
  return parts.map((part, i) => {
    if (part.startsWith('**') && part.endsWith('**')) {
      return <strong key={i}>{part.slice(2, -2)}</strong>;
    }
    if (part === '\n') {
      return <br key={i} />;
    }
    if (part.startsWith('• ')) {
      return <div key={i} className="list-item">{part}</div>;
    }
    return <span key={i}>{part}</span>;
  });
}

function getToolIcon(tool: string): React.ReactElement {
  switch (tool) {
    case 'shell':
      return (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M4 17l6-6-6-6M12 19h8" />
        </svg>
      );
    case 'git':
      return (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="currentColor">
          <path d="M23.546 10.93L13.067.452c-.604-.603-1.582-.603-2.188 0L8.708 2.627l2.76 2.76c.645-.215 1.379-.07 1.889.441.516.515.658 1.258.438 1.9l2.658 2.66c.645-.223 1.387-.078 1.9.435.721.72.721 1.884 0 2.604-.719.719-1.881.719-2.6 0-.539-.541-.674-1.337-.404-1.996L12.86 8.955v6.525c.176.086.342.203.488.348.713.721.713 1.883 0 2.6-.719.721-1.889.721-2.609 0-.719-.719-.719-1.879 0-2.598.182-.18.387-.316.605-.406V8.835c-.217-.091-.424-.222-.6-.401-.545-.545-.676-1.342-.396-2.009L7.636 3.7.45 10.881c-.6.605-.6 1.584 0 2.189l10.48 10.477c.604.604 1.582.604 2.186 0l10.43-10.43c.605-.603.605-1.582 0-2.187" />
        </svg>
      );
    default:
      return (
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <circle cx="12" cy="12" r="3" />
          <path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83" />
        </svg>
      );
  }
}

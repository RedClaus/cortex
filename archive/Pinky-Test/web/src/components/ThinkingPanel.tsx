import type { ThinkingStep } from '../types';
import './ThinkingPanel.css';

interface ThinkingPanelProps {
  steps: ThinkingStep[];
  visible: boolean;
}

export function ThinkingPanel({ steps, visible }: ThinkingPanelProps) {
  if (!visible) return null;

  return (
    <div className="thinking-panel">
      <div className="panel-header">
        <span className={`indicator ${steps.some(s => s.status === 'active') ? 'active' : 'inactive'}`} />
        <span>Agent Thinking</span>
      </div>
      <div className="panel-content">
        {steps.length === 0 ? (
          <div className="thinking-empty">
            <div className="empty-brain">
              <svg width="48" height="48" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1">
                <circle cx="12" cy="12" r="10" />
                <path d="M8 12h.01M12 12h.01M16 12h.01" strokeWidth="2" strokeLinecap="round" />
              </svg>
            </div>
            <p>Waiting for input...</p>
            <span>Pinky will show reasoning steps here</span>
          </div>
        ) : (
          <div className="thinking-steps">
            {steps.map((step, index) => (
              <div
                key={step.id}
                className={`thinking-step ${step.status}`}
                style={{ animationDelay: `${index * 100}ms` }}
              >
                <div className="step-indicator">
                  {step.status === 'completed' && (
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M20 6L9 17l-5-5" />
                    </svg>
                  )}
                  {step.status === 'active' && (
                    <div className="step-spinner" />
                  )}
                  {step.status === 'pending' && (
                    <div className="step-dot" />
                  )}
                  {step.status === 'failed' && (
                    <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                      <path d="M18 6L6 18M6 6l12 12" />
                    </svg>
                  )}
                </div>
                <span className="step-text">{step.description}</span>
              </div>
            ))}
          </div>
        )}

        {/* Neural activity visualization */}
        <div className="neural-activity">
          <div className="neural-grid">
            {[...Array(16)].map((_, i) => (
              <div
                key={i}
                className={`neural-node ${steps.some(s => s.status === 'active') ? 'active' : ''}`}
                style={{
                  animationDelay: `${i * 100}ms`,
                  opacity: steps.some(s => s.status === 'active') ? 0.3 + Math.random() * 0.7 : 0.1,
                }}
              />
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

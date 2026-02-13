import './Header.css';

interface HeaderProps {
  sidebarCollapsed: boolean;
  onToggleSidebar: () => void;
  showThinking: boolean;
  onToggleThinking: () => void;
}

export function Header({
  sidebarCollapsed,
  onToggleSidebar,
  showThinking,
  onToggleThinking,
}: HeaderProps) {
  return (
    <header className="header">
      <div className="header-left">
        <button
          className="header-btn sidebar-toggle"
          onClick={onToggleSidebar}
          aria-label={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            {sidebarCollapsed ? (
              <path d="M4 6h16M4 12h16M4 18h16" />
            ) : (
              <path d="M4 6h16M4 12h10M4 18h16" />
            )}
          </svg>
        </button>

        <div className="logo">
          <div className="logo-icon">
            <svg width="24" height="24" viewBox="0 0 24 24" fill="none">
              <circle cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="1.5" />
              <circle cx="12" cy="12" r="6" stroke="currentColor" strokeWidth="1" opacity="0.6" />
              <circle cx="12" cy="12" r="2" fill="currentColor" />
              <path d="M12 2v4M12 18v4M2 12h4M18 12h4" stroke="currentColor" strokeWidth="1" opacity="0.4" />
            </svg>
          </div>
          <span className="logo-text">Pinky</span>
          <span className="logo-version">v1.0.0</span>
        </div>
      </div>

      <div className="header-center">
        <div className="status-bar">
          <div className="status-item">
            <span className="status-indicator active" />
            <span className="status-label">Brain</span>
            <span className="status-value">Embedded</span>
          </div>
          <div className="status-divider" />
          <div className="status-item">
            <span className="status-indicator active" />
            <span className="status-label">Memory</span>
            <span className="status-value">847 items</span>
          </div>
          <div className="status-divider" />
          <div className="status-item">
            <span className="status-indicator" />
            <span className="status-label">Channels</span>
            <span className="status-value">0 active</span>
          </div>
        </div>
      </div>

      <div className="header-right">
        <button
          className={`header-btn thinking-toggle ${showThinking ? 'active' : ''}`}
          onClick={onToggleThinking}
          aria-label={showThinking ? 'Hide thinking panel' : 'Show thinking panel'}
          title="Toggle reasoning visibility"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <path d="M12 3c-4.97 0-9 3.58-9 8 0 2.1.87 4.02 2.3 5.46L4 21l4.54-1.3C9.98 20.53 10.96 21 12 21c4.97 0 9-3.58 9-8s-4.03-8-9-8z" />
            <circle cx="8" cy="11" r="1" fill="currentColor" />
            <circle cx="12" cy="11" r="1" fill="currentColor" />
            <circle cx="16" cy="11" r="1" fill="currentColor" />
          </svg>
        </button>

        <button className="header-btn settings-btn" aria-label="Settings">
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="3" />
            <path d="M12 1v4M12 19v4M4.22 4.22l2.83 2.83M16.95 16.95l2.83 2.83M1 12h4M19 12h4M4.22 19.78l2.83-2.83M16.95 7.05l2.83-2.83" />
          </svg>
        </button>

        <div className="user-menu">
          <div className="user-avatar">
            <span>U</span>
          </div>
        </div>
      </div>
    </header>
  );
}

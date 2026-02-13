import { useState, useEffect } from 'react';
import type { Config, Persona, PermissionTier, LaneInfo, APIKeyInfo } from '../types';
import './Sidebar.css';

interface SidebarProps {
  collapsed: boolean;
  config: Config;
  personas: Persona[];
  selectedPersona: string;
  permissionTier: PermissionTier;
  onPersonaChange: (id: string) => void;
  onPermissionChange: (tier: PermissionTier) => void;
  onChannelToggle: (channel: keyof Config['channels']) => void;
}

export function Sidebar({
  collapsed,
  config,
  personas,
  selectedPersona,
  permissionTier,
  onPersonaChange,
  onPermissionChange,
  onChannelToggle,
}: SidebarProps) {
  const [expandedSections, setExpandedSections] = useState<Set<string>>(
    new Set(['lanes', 'channels', 'persona', 'permissions'])
  );
  const [lanes, setLanes] = useState<LaneInfo[]>([]);
  const [currentLane, setCurrentLane] = useState<string>('');
  const [autoLLM, setAutoLLM] = useState<boolean>(false);
  const [apiKeys, setApiKeys] = useState<APIKeyInfo[]>([]);
  const [editingKey, setEditingKey] = useState<string | null>(null);
  const [keyInput, setKeyInput] = useState<string>('');

  // Fetch lanes and API keys on mount
  useEffect(() => {
    fetchLanes();
    fetchApiKeys();
  }, []);

  const fetchLanes = async () => {
    try {
      const res = await fetch('/api/v1/lanes');
      if (res.ok) {
        const data = await res.json();
        setLanes(data.lanes || []);
        setCurrentLane(data.current || '');
        setAutoLLM(data.autoLLM || false);
      }
    } catch (err) {
      console.error('Failed to fetch lanes:', err);
    }
  };

  const handleLaneChange = async (laneName: string) => {
    try {
      const res = await fetch('/api/v1/lane', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lane: laneName }),
      });
      if (res.ok) {
        setCurrentLane(laneName);
        // Refresh lanes to update active status
        fetchLanes();
      }
    } catch (err) {
      console.error('Failed to switch lane:', err);
    }
  };

  const handleAutoLLMToggle = async () => {
    try {
      const res = await fetch('/api/v1/autollm', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ enabled: !autoLLM }),
      });
      if (res.ok) {
        setAutoLLM(!autoLLM);
      }
    } catch (err) {
      console.error('Failed to toggle AutoLLM:', err);
    }
  };

  const fetchApiKeys = async () => {
    try {
      const res = await fetch('/api/v1/apikeys');
      if (res.ok) {
        const data = await res.json();
        setApiKeys(data || []);
      }
    } catch (err) {
      console.error('Failed to fetch API keys:', err);
    }
  };

  const handleSaveApiKey = async (lane: string) => {
    try {
      const res = await fetch('/api/v1/apikeys', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lane, apiKey: keyInput }),
      });
      if (res.ok) {
        setEditingKey(null);
        setKeyInput('');
        fetchApiKeys();
      }
    } catch (err) {
      console.error('Failed to save API key:', err);
    }
  };

  const handleClearApiKey = async (lane: string) => {
    try {
      const res = await fetch('/api/v1/apikeys', {
        method: 'DELETE',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ lane }),
      });
      if (res.ok) {
        fetchApiKeys();
      }
    } catch (err) {
      console.error('Failed to clear API key:', err);
    }
  };

  const toggleSection = (section: string) => {
    setExpandedSections(prev => {
      const next = new Set(prev);
      if (next.has(section)) {
        next.delete(section);
      } else {
        next.add(section);
      }
      return next;
    });
  };

  const channels = Object.entries(config.channels) as [keyof Config['channels'], Config['channels'][keyof Config['channels']]][];

  return (
    <aside className={`sidebar ${collapsed ? 'collapsed' : ''}`}>
      <div className="sidebar-content">
        {/* Lanes Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('lanes')}
            aria-expanded={expandedSections.has('lanes')}
          >
            <svg className={`chevron ${expandedSections.has('lanes') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">Inference</span>
            <span className="section-badge">{autoLLM ? 'Auto' : currentLane}</span>
          </button>

          {expandedSections.has('lanes') && (
            <div className="section-content">
              {/* AutoLLM Toggle */}
              <label className="lane-item autollm-toggle">
                <input
                  type="checkbox"
                  checked={autoLLM}
                  onChange={handleAutoLLMToggle}
                  className="lane-checkbox"
                />
                <span className="lane-icon">🔀</span>
                <div className="lane-info">
                  <span className="lane-name">AutoLLM</span>
                  <span className="lane-desc">Auto-select by complexity</span>
                </div>
              </label>

              <div className="lanes-divider" />

              {/* Lane Options */}
              {lanes.map(lane => (
                <label key={lane.name} className={`lane-item ${lane.active && !autoLLM ? 'active' : ''}`}>
                  <input
                    type="radio"
                    name="lane"
                    value={lane.name}
                    checked={currentLane === lane.name && !autoLLM}
                    onChange={() => handleLaneChange(lane.name)}
                    disabled={autoLLM}
                    className="lane-radio"
                  />
                  <span className="lane-icon">{getLaneIcon(lane.name)}</span>
                  <div className="lane-info">
                    <span className="lane-name">{lane.name}</span>
                    <span className="lane-desc">{lane.engine}/{lane.model}</span>
                  </div>
                </label>
              ))}
            </div>
          )}
        </section>

        {/* API Keys Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('apikeys')}
            aria-expanded={expandedSections.has('apikeys')}
          >
            <svg className={`chevron ${expandedSections.has('apikeys') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">API Keys</span>
            <span className="section-badge">
              {apiKeys.filter(k => k.keySet).length}/{apiKeys.length}
            </span>
          </button>

          {expandedSections.has('apikeys') && (
            <div className="section-content">
              {apiKeys.map(key => (
                <div key={key.lane} className="apikey-item">
                  <div className="apikey-header">
                    <span className="apikey-lane">{key.lane}</span>
                    <span className="apikey-engine">{key.engine}</span>
                  </div>
                  {editingKey === key.lane ? (
                    <div className="apikey-edit">
                      <input
                        type="password"
                        value={keyInput}
                        onChange={(e) => setKeyInput(e.target.value)}
                        placeholder="Enter API key..."
                        className="apikey-input"
                        autoFocus
                      />
                      <div className="apikey-actions">
                        <button
                          className="apikey-btn save"
                          onClick={() => handleSaveApiKey(key.lane)}
                        >
                          Save
                        </button>
                        <button
                          className="apikey-btn cancel"
                          onClick={() => { setEditingKey(null); setKeyInput(''); }}
                        >
                          Cancel
                        </button>
                      </div>
                    </div>
                  ) : (
                    <div className="apikey-display">
                      <span className={`apikey-status ${key.keySet ? 'set' : 'unset'}`}>
                        {key.keySet ? key.keyMasked : 'Not set'}
                      </span>
                      <div className="apikey-actions">
                        <button
                          className="apikey-btn edit"
                          onClick={() => { setEditingKey(key.lane); setKeyInput(''); }}
                        >
                          {key.keySet ? 'Change' : 'Set'}
                        </button>
                        {key.keySet && (
                          <button
                            className="apikey-btn clear"
                            onClick={() => handleClearApiKey(key.lane)}
                          >
                            Clear
                          </button>
                        )}
                      </div>
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Channels Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('channels')}
            aria-expanded={expandedSections.has('channels')}
          >
            <svg className={`chevron ${expandedSections.has('channels') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">Channels</span>
            <span className="section-badge">
              {channels.filter(([, c]) => c.enabled).length}/{channels.length}
            </span>
          </button>

          {expandedSections.has('channels') && (
            <div className="section-content">
              {channels.map(([key, channel]) => (
                <label key={key} className="channel-item">
                  <input
                    type="checkbox"
                    checked={channel.enabled}
                    onChange={() => onChannelToggle(key)}
                    className="channel-checkbox"
                  />
                  <span className="channel-icon">
                    {getChannelIcon(key)}
                  </span>
                  <span className="channel-name">{channel.name}</span>
                  <span className={`channel-status ${channel.connected ? 'connected' : ''}`}>
                    {channel.enabled ? (channel.connected ? 'Connected' : 'Enabled') : 'Off'}
                  </span>
                </label>
              ))}
            </div>
          )}
        </section>

        {/* Persona Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('persona')}
            aria-expanded={expandedSections.has('persona')}
          >
            <svg className={`chevron ${expandedSections.has('persona') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">Persona</span>
          </button>

          {expandedSections.has('persona') && (
            <div className="section-content">
              {personas.map(persona => (
                <label key={persona.id} className="persona-item">
                  <input
                    type="radio"
                    name="persona"
                    value={persona.id}
                    checked={selectedPersona === persona.id}
                    onChange={() => onPersonaChange(persona.id)}
                    className="persona-radio"
                  />
                  <div className="persona-info">
                    <span className="persona-name">{persona.name}</span>
                    <span className="persona-desc">{persona.description}</span>
                  </div>
                </label>
              ))}
            </div>
          )}
        </section>

        {/* Permissions Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('permissions')}
            aria-expanded={expandedSections.has('permissions')}
          >
            <svg className={`chevron ${expandedSections.has('permissions') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">Permissions</span>
          </button>

          {expandedSections.has('permissions') && (
            <div className="section-content">
              {(['unrestricted', 'some', 'restricted'] as PermissionTier[]).map(tier => (
                <label key={tier} className="permission-item">
                  <input
                    type="radio"
                    name="permission"
                    value={tier}
                    checked={permissionTier === tier}
                    onChange={() => onPermissionChange(tier)}
                    className="permission-radio"
                  />
                  <div className="permission-info">
                    <span className={`permission-name ${tier}`}>
                      {tier === 'unrestricted' ? 'Open' : tier === 'some' ? 'Some' : 'Restricted'}
                    </span>
                    <span className="permission-desc">
                      {tier === 'unrestricted' && 'Auto-execute all tools'}
                      {tier === 'some' && 'Ask for high-risk tools'}
                      {tier === 'restricted' && 'Ask before every action'}
                    </span>
                  </div>
                </label>
              ))}
            </div>
          )}
        </section>

        {/* Memory Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('memory')}
            aria-expanded={expandedSections.has('memory')}
          >
            <svg className={`chevron ${expandedSections.has('memory') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">Memory</span>
            <span className="section-badge">847</span>
          </button>

          {expandedSections.has('memory') && (
            <div className="section-content">
              <div className="memory-search">
                <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
                  <circle cx="11" cy="11" r="8" />
                  <path d="M21 21l-4.35-4.35" />
                </svg>
                <input type="text" placeholder="Search memories..." className="memory-input" />
              </div>
              <div className="memory-stats">
                <div className="memory-stat">
                  <span className="stat-value">423</span>
                  <span className="stat-label">Episodic</span>
                </div>
                <div className="memory-stat">
                  <span className="stat-value">312</span>
                  <span className="stat-label">Semantic</span>
                </div>
                <div className="memory-stat">
                  <span className="stat-value">112</span>
                  <span className="stat-label">Procedural</span>
                </div>
              </div>
            </div>
          )}
        </section>

        {/* Sessions Section */}
        <section className="sidebar-section">
          <button
            className="section-header"
            onClick={() => toggleSection('sessions')}
            aria-expanded={expandedSections.has('sessions')}
          >
            <svg className={`chevron ${expandedSections.has('sessions') ? 'expanded' : ''}`} width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <path d="M9 18l6-6-6-6" />
            </svg>
            <span className="section-title">Sessions</span>
          </button>

          {expandedSections.has('sessions') && (
            <div className="section-content">
              <div className="session-item active">
                <span className="session-indicator" />
                <span className="session-name">WebUI</span>
                <span className="session-time">now</span>
              </div>
              <div className="session-item">
                <span className="session-indicator" />
                <span className="session-name">TUI</span>
                <span className="session-time">2m ago</span>
              </div>
            </div>
          )}
        </section>
      </div>

      {/* Footer */}
      <div className="sidebar-footer">
        <div className="brain-status">
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="12" cy="12" r="10" />
            <path d="M12 6v6l4 2" />
          </svg>
          <span>Brain: Embedded</span>
        </div>
      </div>
    </aside>
  );
}

function getLaneIcon(lane: string): string {
  switch (lane) {
    case 'fast':
      return '⚡';
    case 'local':
      return '🏠';
    case 'smart':
      return '🧠';
    case 'openai':
      return '🤖';
    default:
      return '🔧';
  }
}

function getChannelIcon(channel: string) {
  switch (channel) {
    case 'telegram':
      return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2zm4.64 6.8c-.15 1.58-.8 5.42-1.13 7.19-.14.75-.42 1-.68 1.03-.58.05-1.02-.38-1.58-.75-.88-.58-1.38-.94-2.23-1.5-.99-.65-.35-1.01.22-1.59.15-.15 2.71-2.48 2.76-2.69a.2.2 0 00-.05-.18c-.06-.05-.14-.03-.21-.02-.09.02-1.49.95-4.22 2.79-.4.27-.76.41-1.08.4-.36-.01-1.04-.2-1.55-.37-.63-.2-1.12-.31-1.08-.66.02-.18.27-.36.74-.55 2.92-1.27 4.86-2.11 5.83-2.51 2.78-1.16 3.35-1.36 3.73-1.36.08 0 .27.02.39.12.1.08.13.19.14.27-.01.06.01.24 0 .38z"/>
        </svg>
      );
    case 'discord':
      return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
          <path d="M20.317 4.37a19.791 19.791 0 00-4.885-1.515.074.074 0 00-.079.037c-.21.375-.444.864-.608 1.25a18.27 18.27 0 00-5.487 0 12.64 12.64 0 00-.617-1.25.077.077 0 00-.079-.037A19.736 19.736 0 003.677 4.37a.07.07 0 00-.032.027C.533 9.046-.32 13.58.099 18.057a.082.082 0 00.031.057 19.9 19.9 0 005.993 3.03.078.078 0 00.084-.028c.462-.63.874-1.295 1.226-1.994a.076.076 0 00-.041-.106 13.107 13.107 0 01-1.872-.892.077.077 0 01-.008-.128 10.2 10.2 0 00.372-.292.074.074 0 01.077-.01c3.928 1.793 8.18 1.793 12.062 0a.074.074 0 01.078.01c.12.098.246.198.373.292a.077.077 0 01-.006.127 12.299 12.299 0 01-1.873.892.077.077 0 00-.041.107c.36.698.772 1.362 1.225 1.993a.076.076 0 00.084.028 19.839 19.839 0 006.002-3.03.077.077 0 00.032-.054c.5-5.177-.838-9.674-3.549-13.66a.061.061 0 00-.031-.03zM8.02 15.33c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.956-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.956 2.418-2.157 2.418zm7.975 0c-1.183 0-2.157-1.085-2.157-2.419 0-1.333.955-2.419 2.157-2.419 1.21 0 2.176 1.096 2.157 2.42 0 1.333-.946 2.418-2.157 2.418z"/>
        </svg>
      );
    case 'slack':
      return (
        <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
          <path d="M5.042 15.165a2.528 2.528 0 01-2.52 2.523A2.528 2.528 0 010 15.165a2.527 2.527 0 012.522-2.52h2.52v2.52zm1.271 0a2.527 2.527 0 012.521-2.52 2.527 2.527 0 012.521 2.52v6.313A2.528 2.528 0 018.834 24a2.528 2.528 0 01-2.521-2.522v-6.313zM8.834 5.042a2.528 2.528 0 01-2.521-2.52A2.528 2.528 0 018.834 0a2.528 2.528 0 012.521 2.522v2.52H8.834zm0 1.271a2.528 2.528 0 012.521 2.521 2.528 2.528 0 01-2.521 2.521H2.522A2.528 2.528 0 010 8.834a2.528 2.528 0 012.522-2.521h6.312zm10.124 2.521a2.528 2.528 0 012.522-2.521A2.528 2.528 0 0124 8.834a2.528 2.528 0 01-2.52 2.521h-2.522V8.834zm-1.268 0a2.528 2.528 0 01-2.523 2.521 2.527 2.527 0 01-2.52-2.521V2.522A2.527 2.527 0 0115.165 0a2.528 2.528 0 012.523 2.522v6.312zm-2.523 10.124a2.528 2.528 0 012.523 2.52A2.528 2.528 0 0115.165 24a2.527 2.527 0 01-2.52-2.522v-2.522h2.52zm0-1.268a2.527 2.527 0 01-2.52-2.523 2.526 2.526 0 012.52-2.52h6.313A2.527 2.527 0 0124 15.165a2.528 2.528 0 01-2.522 2.523h-6.313z"/>
        </svg>
      );
    default:
      return null;
  }
}

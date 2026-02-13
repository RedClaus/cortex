import { useState, useCallback, useEffect } from 'react';
import { Sidebar } from './components/Sidebar';
import { ChatPanel } from './components/ChatPanel';
import { ThinkingPanel } from './components/ThinkingPanel';
import { ToolPanel } from './components/ToolPanel';
import { Header } from './components/Header';
import type { Message, ThinkingStep, ToolCall, Config, PermissionTier, Persona } from './types';
import './App.css';

// Default configuration
const defaultConfig: Config = {
  brain: { mode: 'embedded' },
  server: { host: '127.0.0.1', port: 18800, webuiPort: 18801 },
  channels: {
    telegram: { name: 'Telegram', enabled: false, connected: false },
    discord: { name: 'Discord', enabled: false, connected: false },
    slack: { name: 'Slack', enabled: false, connected: false },
  },
  permissions: { defaultTier: 'some' },
  persona: { default: 'professional' },
};

const personas: Persona[] = [
  { id: 'professional', name: 'Professional', description: 'Clear, concise, formal' },
  { id: 'casual', name: 'Casual', description: 'Friendly, conversational' },
  { id: 'mentor', name: 'Mentor', description: 'Patient, educational' },
  { id: 'minimalist', name: 'Minimalist', description: 'Terse, just the facts' },
];

function App() {
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [showThinking, setShowThinking] = useState(true);
  const [config, setConfig] = useState<Config>(defaultConfig);
  const [selectedPersona, setSelectedPersona] = useState<string>('professional');
  const [permissionTier, setPermissionTier] = useState<PermissionTier>('some');

  // Theme state - default to dark, persist in localStorage
  const [darkMode, setDarkMode] = useState(() => {
    const saved = localStorage.getItem('pinky-theme');
    return saved ? saved === 'dark' : true; // Default to dark mode
  });

  // Apply theme to document
  useEffect(() => {
    document.documentElement.setAttribute('data-theme', darkMode ? 'dark' : 'light');
    localStorage.setItem('pinky-theme', darkMode ? 'dark' : 'light');
  }, [darkMode]);

  const handleToggleDarkMode = useCallback(() => {
    setDarkMode(prev => !prev);
  }, []);

  // Chat state
  const [messages, setMessages] = useState<Message[]>([
    {
      id: '1',
      role: 'assistant',
      content: "Narf! Hello! I'm Pinky, your AI assistant. What shall we do tonight?",
      timestamp: new Date(),
    },
  ]);
  const [inputValue, setInputValue] = useState('');
  const [isStreaming, setIsStreaming] = useState(false);

  // Thinking state
  const [thinkingSteps, setThinkingSteps] = useState<ThinkingStep[]>([]);

  // Tool state
  const [toolCalls, setToolCalls] = useState<ToolCall[]>([]);
  const [pendingApproval, setPendingApproval] = useState<ToolCall | null>(null);

  const handleSendMessage = useCallback(async (content: string) => {
    if (!content.trim() || isStreaming) return;

    const userMessage: Message = {
      id: Date.now().toString(),
      role: 'user',
      content: content.trim(),
      timestamp: new Date(),
    };

    setMessages(prev => [...prev, userMessage]);
    setInputValue('');
    setIsStreaming(true);

    // Show thinking steps
    setThinkingSteps([
      { id: '1', description: 'Analyzing request...', status: 'active' },
      { id: '2', description: 'Processing with Brain', status: 'pending' },
      { id: '3', description: 'Generating output', status: 'pending' },
    ]);

    try {
      // Call the backend API
      const response = await fetch('/api/v1/chat', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ content: content.trim() }),
      });

      // Update thinking steps
      setThinkingSteps(prev => prev.map((s, i) =>
        i === 0 ? { ...s, status: 'completed' } :
        i === 1 ? { ...s, status: 'active' } : s
      ));

      if (!response.ok) {
        throw new Error(`API error: ${response.status}`);
      }

      const data = await response.json();

      // Update thinking steps from response
      if (data.thinking) {
        setThinkingSteps(data.thinking);
      } else {
        setThinkingSteps(prev => prev.map(s => ({ ...s, status: 'completed' })));
      }

      // Add assistant response
      const assistantMessage: Message = {
        id: data.message?.id || (Date.now() + 1).toString(),
        role: 'assistant',
        content: data.message?.content || 'No response received',
        timestamp: new Date(data.message?.timestamp || Date.now()),
        toolCalls: data.tools,
      };

      setMessages(prev => [...prev, assistantMessage]);
    } catch (error) {
      console.error('Chat error:', error);
      // Add error message
      const errorMessage: Message = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: `Error: ${error instanceof Error ? error.message : 'Unknown error'}`,
        timestamp: new Date(),
      };
      setMessages(prev => [...prev, errorMessage]);
      setThinkingSteps(prev => prev.map(s => ({ ...s, status: 'failed' })));
    } finally {
      setIsStreaming(false);
      // Clear thinking steps after a moment
      setTimeout(() => setThinkingSteps([]), 1000);
    }
  }, [isStreaming]);

  const handleApprove = useCallback((toolCall: ToolCall) => {
    setToolCalls(prev => prev.map(tc =>
      tc.id === toolCall.id ? { ...tc, status: 'running' } : tc
    ));
    setPendingApproval(null);

    // Simulate tool execution
    setTimeout(() => {
      setToolCalls(prev => prev.map(tc =>
        tc.id === toolCall.id ? {
          ...tc,
          status: 'completed',
          output: 'Command executed successfully.\n\n$ Output:\nOperation completed in 0.45s'
        } : tc
      ));
    }, 1500);
  }, []);

  const handleDeny = useCallback((toolCall: ToolCall) => {
    setToolCalls(prev => prev.map(tc =>
      tc.id === toolCall.id ? { ...tc, status: 'denied' } : tc
    ));
    setPendingApproval(null);
  }, []);

  const handleChannelToggle = useCallback((channel: keyof Config['channels']) => {
    setConfig(prev => ({
      ...prev,
      channels: {
        ...prev.channels,
        [channel]: {
          ...prev.channels[channel],
          enabled: !prev.channels[channel].enabled,
        },
      },
    }));
  }, []);

  return (
    <div className="app">
      <Header
        sidebarCollapsed={sidebarCollapsed}
        onToggleSidebar={() => setSidebarCollapsed(!sidebarCollapsed)}
        showThinking={showThinking}
        onToggleThinking={() => setShowThinking(!showThinking)}
        darkMode={darkMode}
        onToggleDarkMode={handleToggleDarkMode}
      />

      <div className="app-content">
        <Sidebar
          collapsed={sidebarCollapsed}
          config={config}
          personas={personas}
          selectedPersona={selectedPersona}
          permissionTier={permissionTier}
          onPersonaChange={setSelectedPersona}
          onPermissionChange={setPermissionTier}
          onChannelToggle={handleChannelToggle}
        />

        <main className="main-content">
          <ChatPanel
            messages={messages}
            inputValue={inputValue}
            isStreaming={isStreaming}
            pendingApproval={pendingApproval}
            onInputChange={setInputValue}
            onSendMessage={handleSendMessage}
            onApprove={handleApprove}
            onDeny={handleDeny}
          />
        </main>

        <aside className={`right-panels ${showThinking ? '' : 'collapsed'}`}>
          <ThinkingPanel steps={thinkingSteps} visible={showThinking} />
          <ToolPanel toolCalls={toolCalls} />
        </aside>
      </div>
    </div>
  );
}

export default App;

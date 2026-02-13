import { useState, useCallback } from 'react';
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

    // Simulate AI response with thinking steps
    setThinkingSteps([
      { id: '1', description: 'Analyzing request...', status: 'active' },
      { id: '2', description: 'Planning response', status: 'pending' },
      { id: '3', description: 'Generating output', status: 'pending' },
    ]);

    // Simulate processing
    await new Promise(resolve => setTimeout(resolve, 800));
    setThinkingSteps(prev => prev.map((s, i) =>
      i === 0 ? { ...s, status: 'completed' } :
      i === 1 ? { ...s, status: 'active' } : s
    ));

    await new Promise(resolve => setTimeout(resolve, 600));
    setThinkingSteps(prev => prev.map((s, i) =>
      i <= 1 ? { ...s, status: 'completed' } :
      i === 2 ? { ...s, status: 'active' } : s
    ));

    await new Promise(resolve => setTimeout(resolve, 400));
    setThinkingSteps(prev => prev.map(s => ({ ...s, status: 'completed' })));

    // Add assistant response
    const assistantMessage: Message = {
      id: (Date.now() + 1).toString(),
      role: 'assistant',
      content: getResponse(content),
      timestamp: new Date(),
    };

    setMessages(prev => [...prev, assistantMessage]);
    setIsStreaming(false);

    // Clear thinking steps after a moment
    setTimeout(() => setThinkingSteps([]), 1000);
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

// Simple response generator for demo
function getResponse(input: string): string {
  const lower = input.toLowerCase();
  if (lower.includes('deploy')) {
    return "Poit! I'll help you deploy. Let me check the git status and run the tests first. Are you ready to deploy to staging or production?";
  }
  if (lower.includes('hello') || lower.includes('hi')) {
    return "Zort! Hello there, friend! I'm ready to help you take over... er, I mean, assist with your tasks! What would you like to do?";
  }
  if (lower.includes('help')) {
    return "Narf! Here's what I can do:\n\n• **Shell commands** - Run bash, scripts, git\n• **File operations** - Read, write, search files\n• **Code execution** - Run Python, Node.js\n• **Web requests** - Fetch URLs, call APIs\n• **Git operations** - Status, commit, push, PR\n\nJust tell me what you need!";
  }
  return "Egad! That sounds interesting. Let me think about how I can help with that. Would you like me to explore the codebase or execute some commands?";
}

export default App;

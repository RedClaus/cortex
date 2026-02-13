// Pinky WebUI Type Definitions

export interface Message {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  timestamp: Date;
  toolCalls?: ToolCall[];
  isStreaming?: boolean;
}

export interface ToolCall {
  id: string;
  tool: string;
  input: Record<string, unknown>;
  status: 'pending' | 'approved' | 'denied' | 'running' | 'completed' | 'failed';
  output?: string;
  error?: string;
  reason?: string;
}

export interface ApprovalRequest {
  id: string;
  tool: string;
  command: string;
  riskLevel: 'low' | 'medium' | 'high';
  workingDir?: string;
  reason?: string;
}

export interface ThinkingStep {
  id: string;
  description: string;
  status: 'pending' | 'active' | 'completed' | 'failed';
}

export interface Channel {
  name: string;
  enabled: boolean;
  connected: boolean;
}

export interface Persona {
  id: string;
  name: string;
  description: string;
}

export type PermissionTier = 'unrestricted' | 'some' | 'restricted';

export interface Config {
  brain: {
    mode: 'embedded' | 'remote';
    remoteUrl?: string;
  };
  server: {
    host: string;
    port: number;
    webuiPort: number;
  };
  channels: {
    telegram: Channel;
    discord: Channel;
    slack: Channel;
  };
  permissions: {
    defaultTier: PermissionTier;
  };
  persona: {
    default: string;
  };
}

export interface Session {
  id: string;
  channel: string;
  lastActivity: Date;
  messageCount: number;
}

export interface Memory {
  id: string;
  content: string;
  type: 'episodic' | 'semantic' | 'procedural';
  importance: number;
  createdAt: Date;
}

// Lane types for inference routing
export interface LaneInfo {
  name: string;
  engine: string;
  model: string;
  active: boolean;
}

export interface LanesResponse {
  lanes: LaneInfo[];
  autoLLM: boolean;
  current: string;
}

// API Key management
export interface APIKeyInfo {
  lane: string;
  engine: string;
  model: string;
  keySet: boolean;
  keyMasked?: string;
}

// WebSocket message types
export type WSMessageType =
  | 'chat'
  | 'chat_stream'
  | 'tool_call'
  | 'approval_request'
  | 'approval_response'
  | 'thinking_update'
  | 'config_update'
  | 'error';

export interface WSMessage {
  type: WSMessageType;
  payload: unknown;
}

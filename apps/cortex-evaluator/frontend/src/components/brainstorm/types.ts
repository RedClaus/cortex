import { Node as ReactFlowNode, Edge as ReactFlowEdge } from '@xyflow/react';

export enum NodeType {
  PROBLEM = 'problem',
  SOLUTION = 'solution',
  QUESTION = 'question',
  REFERENCE = 'reference',
  CONSTRAINT = 'constraint'
}

export interface IdeaNodeData {
  title: string;
  content: string;
  aiGenerated: boolean;
  confidence?: number;
  source?: string;
  editable?: boolean;
}

export interface IdeaNode extends ReactFlowNode {
  type: NodeType;
  position: { x: number; y: number };
  data: IdeaNodeData;
}

export interface IdeaEdge extends ReactFlowEdge {
  id: string;
  source: string;
  target: string;
  type?: 'default' | 'floating';
  label?: string;
  animated?: boolean;
  style?: React.CSSProperties;
}

export interface BrainstormSession {
  id: string;
  name: string;
  nodes: IdeaNode[];
  edges: IdeaEdge[];
  createdAt: Date;
  updatedAt: Date;
}

export interface BrainstormViewport {
  x: number;
  y: number;
  zoom: number;
}

export interface NodePaletteItem {
  type: NodeType;
  label: string;
  icon: string;
  color: string;
  bgColor: string;
}

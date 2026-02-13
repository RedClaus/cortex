import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { shallow } from 'zustand/shallow';
import { BrainstormStore, Node, Edge, BrainstormSession, Viewport } from './types';

interface NodeSlice {
  nodes: Node[];
  setNodes: (nodes: Node[]) => void;
  addNode: (node: Node) => void;
  updateNode: (nodeId: string, updates: Partial<Node>) => void;
  deleteNode: (nodeId: string) => void;
}

interface EdgeSlice {
  edges: Edge[];
  setEdges: (edges: Edge[]) => void;
  addEdge: (edge: Edge) => void;
  deleteEdge: (edgeId: string) => void;
}

interface CanvasSlice {
  viewport: Viewport;
  currentSession: BrainstormSession | null;
  setViewport: (viewport: Partial<Viewport>) => void;
  createSession: (name: string) => BrainstormSession;
  loadSession: (sessionId: string) => void;
  saveSession: () => void;
}

const createNodeSlice: (
  set: (partial: Partial<BrainstormStore> | ((state: BrainstormStore) => Partial<BrainstormStore>)) => void,
  get: () => BrainstormStore
) => NodeSlice = (set, get) => ({
  nodes: [],
  setNodes: (nodes) => set({ nodes }),
  addNode: (node) => set((state) => ({ nodes: [...state.nodes, node] })),
  updateNode: (nodeId, updates) =>
    set((state) => ({
      nodes: state.nodes.map((n) =>
        n.id === nodeId ? { ...n, ...updates } : n
      ),
    })),
  deleteNode: (nodeId) =>
    set((state) => ({
      nodes: state.nodes.filter((n) => n.id !== nodeId),
      edges: state.edges.filter((e) => e.source !== nodeId && e.target !== nodeId),
    })),
});

const createEdgeSlice: (
  set: (partial: Partial<BrainstormStore> | ((state: BrainstormStore) => Partial<BrainstormStore>)) => void,
  get: () => BrainstormStore
) => EdgeSlice = (set) => ({
  edges: [],
  setEdges: (edges) => set({ edges }),
  addEdge: (edge) => set((state) => ({ edges: [...state.edges, edge] })),
  deleteEdge: (edgeId) =>
    set((state) => ({
      edges: state.edges.filter((e) => e.id !== edgeId),
    })),
});

const createCanvasSlice: (
  set: (partial: Partial<BrainstormStore> | ((state: BrainstormStore) => Partial<BrainstormStore>)) => void,
  get: () => BrainstormStore
) => CanvasSlice = (set, get) => ({
  viewport: { x: 0, y: 0, zoom: 1 },
  currentSession: null,
  setViewport: (viewport) =>
    set((state) => ({
      viewport: { ...state.viewport, ...viewport },
    })),
  createSession: (name) => {
    const session: BrainstormSession = {
      id: crypto.randomUUID(),
      name,
      nodes: [],
      edges: [],
      createdAt: new Date(),
      updatedAt: new Date(),
    };
    set({ currentSession: session });
    return session;
  },
  loadSession: (sessionId) => {
    const { nodes, edges } = get();
    const session: BrainstormSession = {
      id: sessionId,
      name: `Session ${sessionId}`,
      nodes,
      edges,
      createdAt: new Date(),
      updatedAt: new Date(),
    };
    set({ currentSession: session });
  },
  saveSession: () => {
    const { currentSession, nodes, edges } = get();
    if (currentSession) {
      set((state) => ({
        currentSession: {
          ...state.currentSession!,
          nodes,
          edges,
          updatedAt: new Date(),
        } as BrainstormSession,
      }));
    }
  },
});

export const useBrainstormStore = create<BrainstormStore>()(
  persist(
    (set, get) => ({
      ...createNodeSlice(set, get),
      ...createEdgeSlice(set, get),
      ...createCanvasSlice(set, get),
    }),
    {
      name: 'cortex-evaluator-brainstorm',
      partialize: (state) => ({
        nodes: state.nodes,
        edges: state.edges,
        viewport: state.viewport,
        currentSession: state.currentSession,
      }),
    }
  )
);

export const useBrainstormNodes = () =>
  useBrainstormStore(
    (state) => state.nodes,
    shallow
  );

export const useBrainstormEdges = () =>
  useBrainstormStore(
    (state) => state.edges,
    shallow
  );

export const useBrainstormViewport = () =>
  useBrainstormStore(
    (state) => state.viewport,
    shallow
  );

export const useBrainstormActions = () =>
  useBrainstormStore((state) => ({
    setNodes: state.setNodes,
    addNode: state.addNode,
    updateNode: state.updateNode,
    deleteNode: state.deleteNode,
    setEdges: state.setEdges,
    addEdge: state.addEdge,
    deleteEdge: state.deleteEdge,
    setViewport: state.setViewport,
    createSession: state.createSession,
    loadSession: state.loadSession,
    saveSession: state.saveSession,
  }));

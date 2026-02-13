import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia } from 'pinia';

describe('Brainstorm Store', () => {
  let useBrainstormStore: any;
  let pinia: any;

  beforeEach(async () => {
    pinia = (await import('pinia')).createPinia();
    setActivePinia(pinia);
    vi.useFakeTimers();
  });

  it('should initialize with empty nodes and edges', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { nodes, edges, currentSession, viewport } = useBrainstormStore();

    expect(nodes).toEqual([]);
    expect(edges).toEqual([]);
    expect(currentSession).toBe(null);
    expect(viewport).toEqual({ x: 0, y: 0, zoom: 1 });
  });

  it('should set nodes', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { setNodes, nodes } = useBrainstormStore();

    const newNodes = [
      { id: 'node-1', type: 'problem', content: 'Test problem', position: { x: 100, y: 100 }, color: '#EF4444', connections: [], metadata: {} }
    ];

    setNodes(newNodes);

    expect(nodes).toEqual(newNodes);
  });

  it('should add node to existing nodes', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { addNode, nodes } = useBrainstormStore();

    const initialCount = nodes.length;

    addNode({ id: 'new-node', type: 'solution', content: 'Test solution', position: { x: 200, y: 200 }, color: '#22C55E', connections: [], metadata: {} });

    expect(nodes.length).toBe(initialCount + 1);
    expect(nodes[nodes.length - 1].id).toBe('new-node');
  });

  it('should update node', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { addNode, updateNode, nodes } = useBrainstormStore();

    addNode({ id: 'node-1', type: 'problem', content: 'Old content', position: { x: 100, y: 100 }, color: '#EF4444', connections: [], metadata: {} });

    updateNode('node-1', { content: 'New content' });

    expect(nodes.find((n: any) => n.id === 'node-1').content).toBe('New content');
  });

  it('should delete node and connected edges', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { addNode, addEdge, deleteNode, nodes, edges } = useBrainstormStore();

    addNode({ id: 'node-1', type: 'problem', content: 'Problem', position: { x: 100, y: 100 }, color: '#EF4444', connections: [], metadata: {} });
    addNode({ id: 'node-2', type: 'solution', content: 'Solution', position: { x: 200, y: 100 }, color: '#22C55E', connections: [], metadata: {} });
    addEdge({ id: 'edge-1', source: 'node-1', target: 'node-2', label: '', type: 'directed' });

    expect(nodes.length).toBe(2);
    expect(edges.length).toBe(1);

    deleteNode('node-1');

    expect(nodes.length).toBe(1);
    expect(nodes.find((n: any) => n.id === 'node-1')).toBeUndefined();
    expect(edges.find((e: any) => e.source === 'node-1' || e.target === 'node-1')).toBeUndefined();
  });

  it('should set edges', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { setEdges, edges } = useBrainstormStore();

    const newEdges = [
      { id: 'edge-1', source: 'node-1', target: 'node-2', label: 'related', type: 'directed' }
    ];

    setEdges(newEdges);

    expect(edges).toEqual(newEdges);
  });

  it('should add edge', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { addEdge, edges } = useBrainstormStore();

    const initialCount = edges.length;

    addEdge({ id: 'new-edge', source: 'node-1', target: 'node-2', label: 'related', type: 'directed' });

    expect(edges.length).toBe(initialCount + 1);
  });

  it('should delete edge', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { addEdge, deleteEdge, edges } = useBrainstormStore();

    addEdge({ id: 'edge-1', source: 'node-1', target: 'node-2', label: '', type: 'directed' });
    addEdge({ id: 'edge-2', source: 'node-2', target: 'node-3', label: '', type: 'directed' });

    expect(edges.length).toBe(2);

    deleteEdge('edge-1');

    expect(edges.length).toBe(1);
    expect(edges.find((e: any) => e.id === 'edge-1')).toBeUndefined();
  });

  it('should set viewport', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { setViewport, viewport } = useBrainstormStore();

    setViewport({ x: 100, y: 200, zoom: 1.5 });

    expect(viewport).toEqual({ x: 100, y: 200, zoom: 1.5 });
  });

  it('should partially update viewport', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { setViewport, viewport } = useBrainstormStore();

    const initialViewport = { ...viewport };

    setViewport({ x: 100, zoom: 1.5 });

    expect(viewport.x).toBe(100);
    expect(viewport.y).toBe(initialViewport.y);
    expect(viewport.zoom).toBe(1.5);
  });

  it('should create new session', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { createSession, currentSession } = useBrainstormStore();

    const session = createSession('Test Brainstorm');

    expect(session).toHaveProperty('id');
    expect(session.name).toBe('Test Brainstorm');
    expect(session.nodes).toEqual([]);
    expect(session.edges).toEqual([]);
    expect(currentSession).toEqual(session);
  });

  it('should load session', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { loadSession, nodes, edges, currentSession } = useBrainstormStore();

    const testNodes = [
      { id: 'node-1', type: 'problem', content: 'Test', position: { x: 0, y: 0 }, color: '#EF4444', connections: [], metadata: {} }
    ];
    const testEdges = [
      { id: 'edge-1', source: 'node-1', target: 'node-2', label: '', type: 'directed' }
    ];

    loadSession('session-123');

    expect(nodes).toEqual(testNodes);
    expect(edges).toEqual(testEdges);
    expect(currentSession.id).toBe('session-123');
  });

  it('should save session with current nodes and edges', () => {
    const { useBrainstormStore } = await import('../stores/useBrainstormStore');
    useBrainstormStore = useBrainstormStore();
    const { createSession, addNode, saveSession, currentSession } = useBrainstormStore();

    const session = createSession('Test Session');
    addNode({ id: 'node-1', type: 'problem', content: 'Problem', position: { x: 0, y: 0 }, color: '#EF4444', connections: [], metadata: {} });

    const initialUpdatedAt = session.updatedAt;

    saveSession();

    expect(currentSession.nodes).toHaveLength(1);
    expect(currentSession.updatedAt).not.toBe(initialUpdatedAt);
  });
});

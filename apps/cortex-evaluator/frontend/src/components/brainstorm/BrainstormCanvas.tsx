import { MouseEvent, useCallback, useEffect, useState } from 'react';
import {
  ReactFlow,
  Background,
  Controls,
  MiniMap,
  useNodesState,
  useEdgesState,
  useReactFlow,
  addEdge,
  Connection,
  Edge,
  Node,
  EdgeChange,
  NodeChange,
  BackgroundVariant,
  ReactFlowProvider,
  MarkerType,
} from '@xyflow/react';
import '@xyflow/react/dist/style.css';

import ProblemNode from './ProblemNode';
import SolutionNode from './SolutionNode';
import QuestionNode from './QuestionNode';
import ReferenceNode from './ReferenceNode';
import { NodeType, IdeaNode, IdeaEdge, NodePaletteItem } from './types';
import NodePalette from './NodePalette';
import { useBrainstorm } from '../../hooks/useBrainstorm';

const nodeTypes = {
  [NodeType.PROBLEM]: ProblemNode,
  [NodeType.SOLUTION]: SolutionNode,
  [NodeType.QUESTION]: QuestionNode,
  [NodeType.REFERENCE]: ReferenceNode,
};

const paletteItems: NodePaletteItem[] = [
  {
    type: NodeType.PROBLEM,
    label: 'Problem',
    icon: '⚠️',
    color: '#EF4444',
    bgColor: 'bg-red-50 hover:bg-red-100 border-red-400',
  },
  {
    type: NodeType.SOLUTION,
    label: 'Solution',
    icon: '✅',
    color: '#22C55E',
    bgColor: 'bg-green-50 hover:bg-green-100 border-green-400',
  },
  {
    type: NodeType.QUESTION,
    label: 'Question',
    icon: '❓',
    color: '#3B82F6',
    bgColor: 'bg-blue-50 hover:bg-blue-100 border-blue-400',
  },
  {
    type: NodeType.REFERENCE,
    label: 'Reference',
    icon: '🔗',
    color: '#9333EA',
    bgColor: 'bg-purple-50 hover:bg-purple-100 border-purple-400',
  },
];

function BrainstormCanvasContent() {
  const { fitView, screenToFlowPosition } = useReactFlow();
  const [nodes, setNodes, onNodesChange] = useNodesState<Node>([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState<Edge>([]);
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; nodeId: string } | null>(null);
  const [isExpanding, setIsExpanding] = useState(false);
  const [isConnecting, setIsConnecting] = useState(false);
  const [connectionStatus, setConnectionStatus] = useState<{ valid: boolean; message: string } | null>(null);
  const [detailNode, setDetailNode] = useState<Node | null>(null); // For full-page node view
  const [copied, setCopied] = useState(false);
  const { expandIdea, connectIdeas } = useBrainstorm();

  // Define saveCanvasState first since other callbacks depend on it
  const saveCanvasState = useCallback((savedNodes: Node[], savedEdges: Edge[]) => {
    try {
      const state = {
        nodes: savedNodes,
        edges: savedEdges,
        savedAt: new Date().toISOString(),
      };
      localStorage.setItem('brainstorm-canvas-state', JSON.stringify(state));
    } catch (error) {
      console.error('Failed to save canvas state:', error);
    }
  }, []);

  const loadCanvasState = useCallback(() => {
    try {
      const savedState = localStorage.getItem('brainstorm-canvas-state');
      if (savedState) {
        const state = JSON.parse(savedState);
        setNodes(state.nodes || []);
        setEdges(state.edges || []);
      }
    } catch (error) {
      console.error('Failed to load canvas state:', error);
    }
  }, [setNodes, setEdges]);

  useEffect(() => {
    loadCanvasState();
  }, [loadCanvasState]);

  useEffect(() => {
    if (nodes.length === 0) {
      const timeout = setTimeout(() => {
        fitView({ padding: 0.2, duration: 800 });
      }, 100);
      return () => clearTimeout(timeout);
    }
  }, [nodes.length, fitView]);

  // Simple onConnect - just creates the edge without AI analysis
  const onConnect = useCallback(
    (params: Connection) => {
      if (!params.source || !params.target) return;

      const newEdge: Edge = {
        id: `e${params.source}-${params.target}`,
        source: params.source,
        target: params.target,
        type: 'floating',
        animated: false,
        style: { stroke: '#64748B', strokeWidth: 2 },
        markerEnd: {
          type: MarkerType.ArrowClosed,
          color: '#64748B',
        },
        data: { analyzed: false },
      };
      setEdges((eds) => addEdge(newEdge, eds));
      saveCanvasState(nodes, [...edges, newEdge]);
    },
    [nodes, edges, setEdges, saveCanvasState]
  );

  // AI Analysis - analyzes all unanalyzed connections
  const runAIAnalysis = useCallback(async () => {
    if (edges.length === 0) {
      setConnectionStatus({ valid: false, message: 'No connections to analyze. Connect some nodes first!' });
      setTimeout(() => setConnectionStatus(null), 3000);
      return;
    }

    setIsConnecting(true);
    setConnectionStatus({ valid: true, message: 'Running AI analysis on connections...' });

    // Find edges that haven't been analyzed yet
    const unanalyzedEdges = edges.filter(e => !e.data?.analyzed);

    if (unanalyzedEdges.length === 0) {
      setConnectionStatus({ valid: true, message: 'All connections already analyzed!' });
      setIsConnecting(false);
      setTimeout(() => setConnectionStatus(null), 3000);
      return;
    }

    let analyzedCount = 0;
    let bridgeNodeCreated = false; // Only create one bridge node per analysis run

    for (const edge of unanalyzedEdges) {
      const sourceNode = nodes.find(n => n.id === edge.source);
      const targetNode = nodes.find(n => n.id === edge.target);

      if (!sourceNode || !targetNode) continue;

      const sourceData = sourceNode.data as { content?: string; title?: string };
      const targetData = targetNode.data as { content?: string; title?: string };
      const sourceContent = sourceData?.content || sourceData?.title || sourceNode.type || '';
      const targetContent = targetData?.content || targetData?.title || targetNode.type || '';

      try {
        console.log(`[AI Analysis] Analyzing: "${sourceContent}" <-> "${targetContent}"`);

        const analysis = await connectIdeas(sourceContent, targetContent, 'connected') as {
          ideaA: string;
          ideaB: string;
          relationship: string;
          analysis: {
            synergy: string;
            conflicts: string;
            complementary_aspects: string[];
            integration_approach: string;
            isValid: boolean;
            bridgeConcept?: string;
          };
        };

        console.log('[AI Analysis] Response:', analysis);

        const isValid = analysis.analysis?.isValid !== false;

        // Update edge with analysis result
        setEdges((eds) => eds.map(e => {
          if (e.id === edge.id) {
            return {
              ...e,
              animated: false,
              style: {
                stroke: isValid ? '#22C55E' : '#EF4444',
                strokeWidth: 2,
              },
              markerEnd: {
                type: MarkerType.ArrowClosed,
                color: isValid ? '#22C55E' : '#EF4444',
              },
              label: isValid ? '✓' : '⚠',
              labelStyle: { fill: isValid ? '#22C55E' : '#EF4444', fontWeight: 700 },
              data: { analyzed: true, analysis: analysis.analysis },
            };
          }
          return e;
        }));

        // Create bridge node only once per analysis run (for first connection with a bridge concept)
        if (analysis.analysis?.bridgeConcept && !bridgeNodeCreated) {
          bridgeNodeCreated = true;
          const midX = (sourceNode.position.x + targetNode.position.x) / 2;
          const midY = (sourceNode.position.y + targetNode.position.y) / 2 - 80;

          const bridgeNode: Node = {
            id: `node-${Date.now()}-bridge`,
            type: NodeType.SOLUTION,
            position: { x: midX, y: midY },
            data: {
              title: 'Bridge Concept',
              content: analysis.analysis.bridgeConcept,
              aiGenerated: true,
            },
          };

          // Add bridge node and its edges immediately
          setNodes((nds) => [...nds, bridgeNode]);
          setEdges((eds) => [...eds,
            {
              id: `e${bridgeNode.id}-${edge.source}`,
              source: bridgeNode.id,
              target: edge.source,
              type: 'floating',
              animated: false,
              style: { stroke: '#8B5CF6', strokeWidth: 2, strokeDasharray: '5,5' },
              markerEnd: { type: MarkerType.ArrowClosed, color: '#8B5CF6' },
              data: { analyzed: true },
            },
            {
              id: `e${bridgeNode.id}-${edge.target}`,
              source: bridgeNode.id,
              target: edge.target,
              type: 'floating',
              animated: false,
              style: { stroke: '#8B5CF6', strokeWidth: 2, strokeDasharray: '5,5' },
              markerEnd: { type: MarkerType.ArrowClosed, color: '#8B5CF6' },
              data: { analyzed: true },
            }
          ]);
        }

        analyzedCount++;
      } catch (error) {
        console.error('Analysis failed for edge:', edge.id, error);
        // Mark edge as analyzed but failed
        setEdges((eds) => eds.map(e => {
          if (e.id === edge.id) {
            return {
              ...e,
              style: { stroke: '#F59E0B', strokeWidth: 2 },
              markerEnd: { type: MarkerType.ArrowClosed, color: '#F59E0B' },
              label: '?',
              labelStyle: { fill: '#F59E0B', fontWeight: 700 },
              data: { analyzed: true, error: true },
            };
          }
          return e;
        }));
      }
    }

    setIsConnecting(false);
    setConnectionStatus({
      valid: true,
      message: `Analysis complete! ${analyzedCount} connection(s) analyzed.`,
    });
    setTimeout(() => setConnectionStatus(null), 5000);

    // Save state
    saveCanvasState(nodes, edges);
  }, [nodes, edges, setNodes, setEdges, connectIdeas, saveCanvasState]);


  const onDrop = useCallback(
    (event: React.DragEvent) => {
      event.preventDefault();

      const type = event.dataTransfer.getData('application/reactflow');
      if (!type) return;

      const position = screenToFlowPosition({
        x: event.clientX,
        y: event.clientY,
      });

      const newNode: Node = {
        id: `node-${Date.now()}`,
        type,
        position,
        data: {
          title: type.charAt(0).toUpperCase() + type.slice(1),
          content: '',
          aiGenerated: false,
        },
      };

      setNodes((nds) => {
        const updated = [...nds, newNode];
        saveCanvasState(updated, edges);
        return updated;
      });
    },
    [screenToFlowPosition, setNodes, edges, saveCanvasState]
  );

  const onDragOver = useCallback((event: React.DragEvent) => {
    event.preventDefault();
    event.dataTransfer.dropEffect = 'move';
  }, []);

  const onNodeContextMenu = useCallback(
    (event: MouseEvent, node: Node) => {
      event.preventDefault();
      setContextMenu({
        x: event.clientX,
        y: event.clientY,
        nodeId: node.id,
      });
    },
    []
  );

  const closeContextMenu = useCallback(() => {
    setContextMenu(null);
  }, []);

  const handleAIExpansion = useCallback(
    async (nodeId: string) => {
      closeContextMenu();

      // Find the node to expand
      const nodeToExpand = nodes.find(n => n.id === nodeId);
      if (!nodeToExpand) {
        console.error('Node not found:', nodeId);
        return;
      }

      // Get the content from the node
      const nodeData = nodeToExpand.data as { content?: string; title?: string };
      const nodeContent = nodeData?.content || nodeData?.title || '';
      if (!nodeContent) {
        console.warn('Node has no content to expand');
        return;
      }

      setIsExpanding(true);

      try {
        // Call the AI expand API
        const expansion = await expandIdea(nodeContent, `Node type: ${nodeToExpand.type}`);

        // Create new nodes from the expansion result
        const baseX = nodeToExpand.position.x;
        const baseY = nodeToExpand.position.y;
        const newNodes: Node[] = [];
        const newEdges: Edge[] = [];

        // Create a solution node with the main description
        if (expansion.description) {
          const solutionNode: Node = {
            id: `node-${Date.now()}-desc`,
            type: NodeType.SOLUTION,
            position: { x: baseX + 250, y: baseY - 50 },
            data: {
              title: expansion.title || 'AI Analysis',
              content: expansion.description,
              aiGenerated: true,
            },
          };
          newNodes.push(solutionNode);
          newEdges.push({
            id: `e${nodeId}-${solutionNode.id}`,
            source: nodeId,
            target: solutionNode.id,
            type: 'floating',
            animated: true,
            style: { stroke: '#22C55E', strokeWidth: 2 },
            markerEnd: { type: MarkerType.ArrowClosed, color: '#22C55E' },
          });
        }

        // Create question nodes for considerations
        if (expansion.considerations && expansion.considerations.length > 0) {
          expansion.considerations.slice(0, 3).forEach((consideration, index) => {
            const questionNode: Node = {
              id: `node-${Date.now()}-cons-${index}`,
              type: NodeType.QUESTION,
              position: { x: baseX + 250, y: baseY + 100 + (index * 120) },
              data: {
                title: 'Consideration',
                content: consideration,
                aiGenerated: true,
              },
            };
            newNodes.push(questionNode);
            newEdges.push({
              id: `e${nodeId}-${questionNode.id}`,
              source: nodeId,
              target: questionNode.id,
              type: 'floating',
              animated: true,
              style: { stroke: '#3B82F6', strokeWidth: 2 },
              markerEnd: { type: MarkerType.ArrowClosed, color: '#3B82F6' },
            });
          });
        }

        // Create reference nodes for next steps
        if (expansion.nextSteps && expansion.nextSteps.length > 0) {
          expansion.nextSteps.slice(0, 2).forEach((step, index) => {
            const refNode: Node = {
              id: `node-${Date.now()}-step-${index}`,
              type: NodeType.REFERENCE,
              position: { x: baseX - 200, y: baseY + 100 + (index * 120) },
              data: {
                title: `Next Step ${index + 1}`,
                content: step,
                aiGenerated: true,
              },
            };
            newNodes.push(refNode);
            newEdges.push({
              id: `e${nodeId}-${refNode.id}`,
              source: nodeId,
              target: refNode.id,
              type: 'floating',
              animated: true,
              style: { stroke: '#9333EA', strokeWidth: 2 },
              markerEnd: { type: MarkerType.ArrowClosed, color: '#9333EA' },
            });
          });
        }

        // Update state with new nodes and edges
        setNodes((nds) => {
          const updated = [...nds, ...newNodes];
          saveCanvasState(updated, [...edges, ...newEdges]);
          return updated;
        });
        setEdges((eds) => [...eds, ...newEdges]);

        console.log(`AI expansion complete: created ${newNodes.length} new nodes`);

      } catch (error) {
        console.error('AI expansion failed:', error);
      } finally {
        setIsExpanding(false);
      }
    },
    [closeContextMenu, nodes, edges, expandIdea, setNodes, setEdges, saveCanvasState]
  );

  const onNodesChangeHandler = useCallback(
    (changes: NodeChange[]) => {
      onNodesChange(changes);
    },
    [onNodesChange]
  );

  const onEdgesChangeHandler = useCallback(
    (changes: EdgeChange[]) => {
      onEdgesChange(changes);
    },
    [onEdgesChange]
  );

  return (
    <div className="w-full h-screen flex">
      <div className="w-64 bg-gray-50 border-r border-gray-200 p-4 overflow-y-auto">
        <div className="mb-4">
          <h2 className="text-lg font-semibold text-gray-900 mb-2">Node Palette</h2>
          <p className="text-sm text-gray-600">Drag nodes to canvas</p>
        </div>
        <NodePalette items={paletteItems} />

        {/* AI Analysis Section */}
        <div className="mt-6 pt-4 border-t border-gray-200">
          <h3 className="text-md font-semibold text-gray-900 mb-2">AI Analysis</h3>
          <p className="text-xs text-gray-500 mb-3">
            Connect nodes, then click to analyze relationships
          </p>
          <button
            onClick={runAIAnalysis}
            disabled={isConnecting || edges.length === 0}
            className={`w-full px-4 py-3 rounded-lg font-medium text-white flex items-center justify-center gap-2 transition-all
              ${isConnecting
                ? 'bg-blue-400 cursor-wait'
                : edges.length === 0
                  ? 'bg-gray-300 cursor-not-allowed'
                  : 'bg-blue-600 hover:bg-blue-700 active:bg-blue-800'}`}
          >
            {isConnecting ? (
              <>
                <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Analyzing...
              </>
            ) : (
              <>
                <span>🤖</span>
                Run AI Analysis
              </>
            )}
          </button>
          {edges.length > 0 && (
            <p className="text-xs text-gray-400 mt-2 text-center">
              {edges.filter(e => e.data?.analyzed).length}/{edges.length} connections analyzed
            </p>
          )}
        </div>
      </div>

      <div className="flex-1 h-full">
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChangeHandler}
          onEdgesChange={onEdgesChangeHandler}
          onConnect={onConnect}
          onDrop={onDrop}
          onDragOver={onDragOver}
          onNodeContextMenu={onNodeContextMenu}
          onNodeDoubleClick={(_, node) => setDetailNode(node)}
          onPaneClick={closeContextMenu}
          nodeTypes={nodeTypes}
          nodeOrigin={[0.5, 0.5]}
          fitView
          defaultEdgeOptions={{
            type: 'floating',
            animated: false,
            style: { stroke: '#64748B', strokeWidth: 2 },
            markerEnd: {
              type: MarkerType.ArrowClosed,
              color: '#64748B',
            },
          }}
        >
          <Background color="#E5E7EB" gap={16} variant={BackgroundVariant.Dots} />
          <Controls />
          <MiniMap
            nodeColor={(node) => {
              switch (node.type) {
                case NodeType.PROBLEM:
                  return '#EF4444';
                case NodeType.SOLUTION:
                  return '#22C55E';
                case NodeType.QUESTION:
                  return '#3B82F6';
                case NodeType.REFERENCE:
                  return '#9333EA';
                default:
                  return '#64748B';
              }
            }}
            position="bottom-right"
          />
        </ReactFlow>

        {contextMenu && (
          <div
            className="fixed bg-white rounded-lg shadow-xl border border-gray-200 py-2 min-w-[200px] z-50"
            style={{ left: contextMenu.x, top: contextMenu.y }}
          >
            <button
              onClick={() => {
                const node = nodes.find(n => n.id === contextMenu.nodeId);
                if (node) setDetailNode(node);
                closeContextMenu();
              }}
              className="w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 flex items-center gap-2"
            >
              <span>🔍</span>
              View Full Details
            </button>
            <button
              onClick={() => handleAIExpansion(contextMenu.nodeId)}
              disabled={isExpanding}
              className="w-full px-4 py-2 text-left text-sm text-gray-700 hover:bg-gray-100 flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <span>{isExpanding ? '⏳' : '🤖'}</span>
              {isExpanding ? 'Expanding...' : 'AI Expand Node'}
            </button>
            <div className="border-t border-gray-100 my-1"></div>
            <button
              onClick={() => {
                setNodes((nds) => nds.filter((n) => n.id !== contextMenu.nodeId));
                closeContextMenu();
              }}
              className="w-full px-4 py-2 text-left text-sm text-red-600 hover:bg-red-50 flex items-center gap-2"
            >
              <span>🗑️</span>
              Delete Node
            </button>
          </div>
        )}

        {isExpanding && (
          <div className="fixed bottom-4 right-4 bg-blue-500 text-white px-4 py-2 rounded-lg shadow-lg flex items-center gap-2 z-50">
            <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            AI is analyzing your idea...
          </div>
        )}

        {isConnecting && (
          <div className="fixed bottom-4 right-4 bg-amber-500 text-white px-4 py-2 rounded-lg shadow-lg flex items-center gap-2 z-50">
            <svg className="animate-spin h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
            </svg>
            AI is analyzing connection...
          </div>
        )}

        {connectionStatus && !isConnecting && (
          <div className={`fixed bottom-4 right-4 px-4 py-3 rounded-lg shadow-lg z-50 max-w-md ${
            connectionStatus.valid ? 'bg-green-500 text-white' : 'bg-red-500 text-white'
          }`}>
            <div className="flex items-center gap-2">
              <span className="text-lg">{connectionStatus.valid ? '✓' : '⚠'}</span>
              <span className="text-sm">{connectionStatus.message}</span>
            </div>
          </div>
        )}

        {/* Node Detail Modal - Full page view when double-clicking a node */}
        {detailNode && (
          <div
            className="fixed inset-0 bg-black/50 z-50 flex items-center justify-center p-4"
            onClick={() => setDetailNode(null)}
          >
            <div
              className="bg-white rounded-2xl shadow-2xl w-full max-w-2xl max-h-[80vh] overflow-hidden flex flex-col"
              onClick={(e) => e.stopPropagation()}
            >
              {/* Header */}
              <div className={`px-6 py-4 border-b flex items-center justify-between ${
                detailNode.type === NodeType.PROBLEM ? 'bg-red-50 border-red-200' :
                detailNode.type === NodeType.SOLUTION ? 'bg-green-50 border-green-200' :
                detailNode.type === NodeType.QUESTION ? 'bg-blue-50 border-blue-200' :
                'bg-purple-50 border-purple-200'
              }`}>
                <div className="flex items-center gap-3">
                  <span className="text-2xl">
                    {detailNode.type === NodeType.PROBLEM ? '⚠️' :
                     detailNode.type === NodeType.SOLUTION ? '✅' :
                     detailNode.type === NodeType.QUESTION ? '❓' : '🔗'}
                  </span>
                  <div>
                    <h2 className="text-xl font-bold text-gray-900">
                      {(detailNode.data as { title?: string })?.title || detailNode.type}
                    </h2>
                    {(detailNode.data as { aiGenerated?: boolean })?.aiGenerated && (
                      <span className="text-xs text-gray-500">AI Generated</span>
                    )}
                  </div>
                </div>
                <button
                  onClick={() => setDetailNode(null)}
                  className="p-2 hover:bg-gray-200 rounded-full transition-colors"
                >
                  <svg className="w-6 h-6 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </button>
              </div>

              {/* Content */}
              <div className="flex-1 overflow-y-auto p-6">
                <div className="prose prose-sm max-w-none">
                  <div className="flex items-center justify-between mb-2">
                    <h3 className="text-lg font-semibold text-gray-700">Content</h3>
                    <button
                      onClick={() => {
                        const content = (detailNode.data as { content?: string })?.content || '';
                        navigator.clipboard.writeText(content);
                        setCopied(true);
                        setTimeout(() => setCopied(false), 2000);
                      }}
                      className={`px-3 py-1 text-xs rounded-md flex items-center gap-1 transition-colors ${
                        copied ? 'bg-green-500 text-white' : 'bg-gray-200 hover:bg-gray-300 text-gray-700'
                      }`}
                    >
                      <span>{copied ? '✓' : '📋'}</span> {copied ? 'Copied!' : 'Copy'}
                    </button>
                  </div>
                  <div className="bg-gray-50 rounded-lg p-4 whitespace-pre-wrap text-gray-800 min-h-[200px] max-h-[400px] overflow-y-auto select-text cursor-text">
                    {(detailNode.data as { content?: string })?.content || 'No content yet...'}
                  </div>
                </div>

                {/* Show analysis data if available (from connected edges) */}
                {edges.filter(e => e.source === detailNode.id || e.target === detailNode.id)
                  .filter(e => e.data?.analysis).length > 0 && (
                  <div className="mt-6">
                    <h3 className="text-lg font-semibold text-gray-700 mb-2">Connection Analysis</h3>
                    {edges.filter(e => e.source === detailNode.id || e.target === detailNode.id)
                      .filter(e => e.data?.analysis)
                      .map((edge, idx) => {
                        const analysis = edge.data?.analysis as {
                          synergy?: string;
                          conflicts?: string;
                          complementary_aspects?: string[];
                          integration_approach?: string;
                        };
                        const otherNodeId = edge.source === detailNode.id ? edge.target : edge.source;
                        const otherNode = nodes.find(n => n.id === otherNodeId);
                        return (
                          <div key={idx} className="bg-blue-50 rounded-lg p-4 mb-3">
                            <p className="text-sm font-medium text-blue-800 mb-2">
                              Connection to: {(otherNode?.data as { title?: string })?.title || 'Node'}
                            </p>
                            {analysis?.synergy && (
                              <p className="text-sm text-gray-700"><strong>Synergy:</strong> {analysis.synergy}</p>
                            )}
                            {analysis?.integration_approach && (
                              <p className="text-sm text-gray-700 mt-1"><strong>Integration:</strong> {analysis.integration_approach}</p>
                            )}
                            {analysis?.complementary_aspects && analysis.complementary_aspects.length > 0 && (
                              <div className="mt-2">
                                <strong className="text-sm text-gray-700">Complementary Aspects:</strong>
                                <ul className="list-disc list-inside text-sm text-gray-600 mt-1">
                                  {analysis.complementary_aspects.map((aspect, i) => (
                                    <li key={i}>{aspect}</li>
                                  ))}
                                </ul>
                              </div>
                            )}
                          </div>
                        );
                      })}
                  </div>
                )}
              </div>

              {/* Footer */}
              <div className="px-6 py-4 border-t bg-gray-50 flex justify-end gap-3">
                <button
                  onClick={async () => {
                    await handleAIExpansion(detailNode.id);
                    setDetailNode(null);
                  }}
                  disabled={isExpanding}
                  className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:opacity-50 flex items-center gap-2"
                >
                  {isExpanding ? (
                    <>
                      <svg className="animate-spin h-4 w-4" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                      </svg>
                      Expanding...
                    </>
                  ) : (
                    <>
                      <span>🤖</span>
                      AI Expand
                    </>
                  )}
                </button>
                <button
                  onClick={() => setDetailNode(null)}
                  disabled={isExpanding}
                  className="px-4 py-2 bg-gray-200 text-gray-700 rounded-lg hover:bg-gray-300 disabled:opacity-50"
                >
                  Close
                </button>
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function BrainstormCanvas() {
  return (
    <ReactFlowProvider>
      <BrainstormCanvasContent />
    </ReactFlowProvider>
  );
}

function applyNodeChanges(changes: NodeChange[], nodes: Node[]): Node[] {
  return nodes;
}

function applyEdgeChanges(changes: EdgeChange[], edges: Edge[]): Edge[] {
  return edges;
}

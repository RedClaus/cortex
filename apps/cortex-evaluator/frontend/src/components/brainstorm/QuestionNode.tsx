import { memo, useCallback } from 'react';
import { Handle, Position, NodeProps, useReactFlow } from '@xyflow/react';

const QuestionNode = memo(({ id, data, selected }: NodeProps) => {
  const { setNodes } = useReactFlow();

  const updateNodeData = useCallback((field: string, value: string) => {
    setNodes((nds) =>
      nds.map((node) => {
        if (node.id === id) {
          return {
            ...node,
            data: { ...node.data, [field]: value },
          };
        }
        return node;
      })
    );
  }, [id, setNodes]);

  return (
    <div
      className={`px-4 py-3 shadow-lg rounded-lg border-2 min-w-[280px] max-w-[320px] bg-white
        ${selected ? 'border-blue-500 ring-2 ring-blue-500 ring-offset-2' : 'border-blue-400'}`}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!w-3 !h-3 !bg-blue-500"
      />

      <div className="flex items-center gap-2 mb-2">
        <span className="text-lg">❓</span>
        <input
          type="text"
          value={data.title || 'Question'}
          onChange={(e) => updateNodeData('title', e.target.value)}
          className="font-bold text-gray-900 bg-transparent border-none outline-none w-full"
        />
      </div>

      <textarea
        value={data.content || ''}
        onChange={(e) => updateNodeData('content', e.target.value)}
        placeholder="What do you want to explore?"
        className="w-full min-h-[80px] p-2 text-sm text-gray-700 bg-gray-50 rounded border border-gray-200 resize-y focus:outline-none focus:ring-2 focus:ring-blue-400"
        rows={3}
      />

      {data.aiGenerated && (
        <div className="mt-2 flex items-center gap-1 text-xs text-gray-500">
          <span className="px-2 py-1 bg-blue-100 text-blue-700 rounded-full">
            AI Generated
          </span>
          {data.confidence !== undefined && (
            <span className="ml-auto">
              Confidence: {Math.round(data.confidence * 100)}%
            </span>
          )}
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        className="!w-3 !h-3 !bg-blue-500"
      />
    </div>
  );
});

QuestionNode.displayName = 'QuestionNode';

export default QuestionNode;

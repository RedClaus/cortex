import { memo, useCallback } from 'react';
import { Handle, Position, NodeProps, useReactFlow } from '@xyflow/react';

const ReferenceNode = memo(({ id, data, selected }: NodeProps) => {
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
      className={`px-3 py-2 shadow-md rounded-lg border-2 min-w-[200px] max-w-[280px] bg-white
        ${selected ? 'border-purple-500 ring-2 ring-purple-500 ring-offset-2' : 'border-purple-400'}`}
    >
      <Handle
        type="target"
        position={Position.Left}
        className="!w-3 !h-3 !bg-purple-500"
      />

      <div className="flex items-center gap-1.5 mb-1.5">
        <span className="text-base">🔗</span>
        <input
          type="text"
          value={data.title || 'Reference'}
          onChange={(e) => updateNodeData('title', e.target.value)}
          className="font-semibold text-sm text-gray-900 bg-transparent border-none outline-none w-full"
        />
      </div>

      {data.source && (
        <div className="text-xs text-purple-600 mb-1.5 truncate">
          {data.source}
        </div>
      )}

      <textarea
        value={data.content || ''}
        onChange={(e) => updateNodeData('content', e.target.value)}
        placeholder="Add reference details..."
        className="w-full min-h-[60px] p-1.5 text-xs text-gray-700 bg-gray-50 rounded border border-gray-200 resize-y focus:outline-none focus:ring-2 focus:ring-purple-400"
        rows={2}
      />

      {data.aiGenerated && (
        <div className="mt-1.5 flex items-center gap-1 text-xs text-gray-500">
          <span className="px-1.5 py-0.5 bg-purple-100 text-purple-700 rounded-full">
            AI Generated
          </span>
        </div>
      )}

      <Handle
        type="source"
        position={Position.Right}
        className="!w-3 !h-3 !bg-purple-500"
      />
    </div>
  );
});

ReferenceNode.displayName = 'ReferenceNode';

export default ReferenceNode;

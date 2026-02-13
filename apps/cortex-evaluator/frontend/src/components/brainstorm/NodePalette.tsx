import { memo } from 'react';
import { NodePaletteItem } from './types';

interface NodePaletteProps {
  items: NodePaletteItem[];
}

const NodePalette = memo(({ items }: NodePaletteProps) => {
  const onDragStart = (event: React.DragEvent, type: string) => {
    event.dataTransfer.setData('application/reactflow', type);
    event.dataTransfer.effectAllowed = 'move';
  };

  return (
    <div className="space-y-3">
      {items.map((item) => (
        <div
          key={item.type}
          draggable
          onDragStart={(e) => onDragStart(e, item.type)}
          className={`
            p-3 rounded-lg border-2 cursor-move transition-all duration-200
            ${item.bgColor}
            shadow-sm hover:shadow-md hover:scale-105 active:scale-95
          `}
        >
          <div className="flex items-center gap-2">
            <span className="text-2xl">{item.icon}</span>
            <span className="font-semibold text-gray-900">{item.label}</span>
          </div>
          <div className="text-xs text-gray-600 mt-1">
            {item.label} Node
          </div>
        </div>
      ))}
    </div>
  );
});

NodePalette.displayName = 'NodePalette';

export default NodePalette;

import { useRef, useEffect, useState } from 'react';
import { useVirtualizer } from '@tanstack/react-virtual';
import { clsx } from 'clsx';
import { Edit2, Check, X, User } from 'lucide-react';
import { useMeetingStore } from '@/store';
import { formatTime } from '@/utils/format';
import type { TranscriptSegment } from '@/models';
import { Button } from '@/components/ui';

interface TranscriptViewProps {
  className?: string;
}

export function TranscriptView({ className }: TranscriptViewProps) {
  const {
    currentMeeting,
    autoScrollEnabled,
    selectedSpeakerId,
    interimText,
    updateSegment,
    speakers,
  } = useMeetingStore((state) => ({
    currentMeeting: state.currentMeeting,
    autoScrollEnabled: state.autoScrollEnabled,
    selectedSpeakerId: state.selectedSpeakerId,
    interimText: state.interimText,
    updateSegment: state.updateSegment,
    speakers: state.currentMeeting?.speakers || [],
  }));

  const parentRef = useRef<HTMLDivElement>(null);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [editText, setEditText] = useState('');

  const segments = currentMeeting?.segments || [];
  const filteredSegments = selectedSpeakerId
    ? segments.filter((s) => s.speakerId === selectedSpeakerId)
    : segments;

  const virtualizer = useVirtualizer({
    count: filteredSegments.length + (interimText ? 1 : 0),
    getScrollElement: () => parentRef.current,
    estimateSize: () => 80,
    overscan: 5,
  });

  useEffect(() => {
    if (autoScrollEnabled && parentRef.current) {
      parentRef.current.scrollTop = parentRef.current.scrollHeight;
    }
  }, [segments.length, interimText, autoScrollEnabled]);

  const handleEdit = (segment: TranscriptSegment) => {
    setEditingId(segment.id);
    setEditText(segment.text);
  };

  const handleSaveEdit = () => {
    if (editingId) {
      updateSegment(editingId, { text: editText });
      setEditingId(null);
      setEditText('');
    }
  };

  const handleCancelEdit = () => {
    setEditingId(null);
    setEditText('');
  };

  const getSpeakerColor = (speakerId: string) => {
    const speaker = speakers.find((s) => s.id === speakerId);
    return speaker?.color || '#6B7280';
  };

  if (segments.length === 0 && !interimText) {
    return (
      <div
        className={clsx(
          'flex flex-col items-center justify-center h-full text-gray-400',
          className
        )}
      >
        <User className="w-12 h-12 mb-4" />
        <p className="text-lg font-medium">No transcript yet</p>
        <p className="text-sm">Start recording to see the transcript</p>
      </div>
    );
  }

  return (
    <div ref={parentRef} className={clsx('overflow-auto scrollbar-thin', className)}>
      <div
        style={{
          height: `${virtualizer.getTotalSize()}px`,
          width: '100%',
          position: 'relative',
        }}
      >
        {virtualizer.getVirtualItems().map((virtualItem) => {
          const isInterim = virtualItem.index === filteredSegments.length;

          if (isInterim) {
            return (
              <div
                key="interim"
                style={{
                  position: 'absolute',
                  top: 0,
                  left: 0,
                  width: '100%',
                  transform: `translateY(${virtualItem.start}px)`,
                }}
                className="p-3"
              >
                <div className="flex gap-3">
                  <div
                    className="w-1 rounded-full opacity-50"
                    style={{ backgroundColor: getSpeakerColor(speakers[0]?.id || '') }}
                  />
                  <p className="text-gray-400 italic">{interimText}</p>
                </div>
              </div>
            );
          }

          const segment = filteredSegments[virtualItem.index];
          const isEditing = editingId === segment.id;

          return (
            <div
              key={segment.id}
              style={{
                position: 'absolute',
                top: 0,
                left: 0,
                width: '100%',
                transform: `translateY(${virtualItem.start}px)`,
              }}
              className="p-3 group"
            >
              <div className="flex gap-3">
                <div
                  className="w-1 rounded-full shrink-0"
                  style={{ backgroundColor: getSpeakerColor(segment.speakerId) }}
                />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <span
                      className="text-sm font-medium"
                      style={{ color: getSpeakerColor(segment.speakerId) }}
                    >
                      {segment.speakerLabel}
                    </span>
                    <span className="text-xs text-gray-400">
                      {formatTime(segment.startTime)}
                    </span>
                    {segment.isEdited && (
                      <span className="text-xs text-gray-400">(edited)</span>
                    )}
                  </div>

                  {isEditing ? (
                    <div className="flex items-start gap-2">
                      <textarea
                        value={editText}
                        onChange={(e) => setEditText(e.target.value)}
                        className="flex-1 p-2 text-sm bg-white dark:bg-surface-800 border border-gray-300 dark:border-surface-600 rounded resize-none"
                        rows={3}
                        autoFocus
                      />
                      <div className="flex flex-col gap-1">
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={handleSaveEdit}
                          icon={<Check className="w-4 h-4 text-green-500" />}
                        />
                        <Button
                          variant="ghost"
                          size="sm"
                          onClick={handleCancelEdit}
                          icon={<X className="w-4 h-4 text-red-500" />}
                        />
                      </div>
                    </div>
                  ) : (
                    <div className="flex items-start gap-2">
                      <p className="text-gray-700 dark:text-gray-300 text-sm leading-relaxed">
                        {segment.text}
                      </p>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleEdit(segment)}
                        className="opacity-0 group-hover:opacity-100 transition-opacity shrink-0"
                        icon={<Edit2 className="w-3 h-3" />}
                      />
                    </div>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

import { useState } from 'react';
import { ArrowLeft, Edit2, Check, Tag, X } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useMeetingStore, useCortexStore } from '@/store';
import { formatDateTime } from '@/utils/format';
import { Button, Badge, Input } from '@/components/ui';

export function MeetingHeader() {
  const navigate = useNavigate();
  const {
    currentMeeting,
    updateMeetingTitle,
    addTag,
    removeTag,
    closeMeeting,
  } = useMeetingStore();
  const { status } = useCortexStore();

  const [isEditingTitle, setIsEditingTitle] = useState(false);
  const [titleInput, setTitleInput] = useState('');
  const [tagInput, setTagInput] = useState('');
  const [showTagInput, setShowTagInput] = useState(false);

  const handleEditTitle = () => {
    setTitleInput(currentMeeting?.title || '');
    setIsEditingTitle(true);
  };

  const handleSaveTitle = () => {
    if (titleInput.trim()) {
      updateMeetingTitle(titleInput.trim());
    }
    setIsEditingTitle(false);
  };

  const handleAddTag = () => {
    if (tagInput.trim()) {
      addTag(tagInput.trim());
      setTagInput('');
    }
    setShowTagInput(false);
  };

  const handleBack = () => {
    closeMeeting();
    navigate('/');
  };

  if (!currentMeeting) return null;

  return (
    <header className="flex items-center justify-between px-6 py-4 bg-white dark:bg-surface-900 border-b border-gray-200 dark:border-surface-700">
      <div className="flex items-center gap-4">
        <Button variant="ghost" onClick={handleBack} icon={<ArrowLeft className="w-4 h-4" />} />

        <div>
          {isEditingTitle ? (
            <div className="flex items-center gap-2">
              <Input
                value={titleInput}
                onChange={(e) => setTitleInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleSaveTitle()}
                className="w-64"
                autoFocus
              />
              <Button
                variant="ghost"
                size="sm"
                onClick={handleSaveTitle}
                icon={<Check className="w-4 h-4" />}
              />
            </div>
          ) : (
            <div className="flex items-center gap-2 group">
              <h1 className="text-xl font-semibold text-gray-900 dark:text-white">
                {currentMeeting.title}
              </h1>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleEditTitle}
                className="opacity-0 group-hover:opacity-100 transition-opacity"
                icon={<Edit2 className="w-3 h-3" />}
              />
            </div>
          )}

          <div className="flex items-center gap-2 mt-1 text-sm text-gray-500">
            <span>{formatDateTime(currentMeeting.createdAt)}</span>
            {currentMeeting.participants.length > 0 && (
              <>
                <span>•</span>
                <span>{currentMeeting.participants.length} participants</span>
              </>
            )}
          </div>
        </div>
      </div>

      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          {currentMeeting.tags.map((tag) => (
            <Badge key={tag} variant="default" className="group">
              {tag}
              <button
                onClick={() => removeTag(tag)}
                className="ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
              >
                <X className="w-3 h-3" />
              </button>
            </Badge>
          ))}

          {showTagInput ? (
            <div className="flex items-center gap-1">
              <Input
                value={tagInput}
                onChange={(e) => setTagInput(e.target.value)}
                onKeyDown={(e) => e.key === 'Enter' && handleAddTag()}
                placeholder="Add tag..."
                className="w-24 h-6 text-xs"
                autoFocus
              />
              <Button
                variant="ghost"
                size="sm"
                onClick={handleAddTag}
                icon={<Check className="w-3 h-3" />}
              />
            </div>
          ) : (
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowTagInput(true)}
              icon={<Tag className="w-3 h-3" />}
            >
              Add Tag
            </Button>
          )}
        </div>

        <div className="flex items-center gap-2 text-sm">
          <div
            className={`w-2 h-2 rounded-full ${
              status === 'connected'
                ? 'bg-green-500'
                : status === 'connecting'
                ? 'bg-yellow-500 animate-pulse'
                : 'bg-red-500'
            }`}
          />
          <span className="text-gray-500 dark:text-gray-400">
            Cortex: {status}
          </span>
        </div>
      </div>
    </header>
  );
}

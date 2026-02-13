import React, { useEffect, useState, useMemo } from 'react';
import { useNavigate } from 'react-router-dom';
import {
  Search,
  Clock,
  FileText,
  Trash2,
  Download,
  Copy,
  Sparkles,
  SortAsc,
  SortDesc,
} from 'lucide-react';
import { useMeetingStore } from '@/store';
import {
  getAllMeetings,
  deleteMeeting,
  duplicateMeeting,
  searchMeetings,
} from '@/services/meeting';
import { exportMeetingAsMarkdown } from '@/utils/export';
import { formatRelativeTime, formatDuration, formatDate } from '@/utils/format';
import { Button, Input, Card, Badge, Select } from '@/components/ui';
import type { MeetingSession } from '@/models';

type SortField = 'date' | 'title' | 'duration';
type SortOrder = 'asc' | 'desc';

export function HistoryView() {
  const navigate = useNavigate();
  const { loadMeeting } = useMeetingStore();
  const [meetings, setMeetings] = useState<MeetingSession[]>([]);
  const [searchQuery, setSearchQuery] = useState('');
  const [sortField, setSortField] = useState<SortField>('date');
  const [sortOrder, setSortOrder] = useState<SortOrder>('desc');
  const [filterAnalyzed, setFilterAnalyzed] = useState<'all' | 'yes' | 'no'>('all');
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadMeetings();
  }, []);

  const loadMeetings = async () => {
    setLoading(true);
    const data = await getAllMeetings();
    setMeetings(data);
    setLoading(false);
  };

  const handleSearch = async (query: string) => {
    setSearchQuery(query);
    if (query.trim()) {
      const results = await searchMeetings(query);
      setMeetings(results);
    } else {
      loadMeetings();
    }
  };

  const handleOpen = (meeting: MeetingSession) => {
    loadMeeting(meeting);
    navigate('/meeting');
  };

  const handleDelete = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    if (window.confirm('Are you sure you want to delete this meeting?')) {
      await deleteMeeting(id);
      setMeetings((prev) => prev.filter((m) => m.id !== id));
    }
  };

  const handleDuplicate = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    const duplicate = await duplicateMeeting(id);
    if (duplicate) {
      setMeetings((prev) => [duplicate, ...prev]);
    }
  };

  const handleExport = (meeting: MeetingSession, e: React.MouseEvent) => {
    e.stopPropagation();
    exportMeetingAsMarkdown(meeting);
  };

  const sortedAndFilteredMeetings = useMemo(() => {
    let result = [...meetings];

    if (filterAnalyzed !== 'all') {
      result = result.filter((m) =>
        filterAnalyzed === 'yes' ? m.isAnalyzed : !m.isAnalyzed
      );
    }

    result.sort((a, b) => {
      let comparison = 0;
      switch (sortField) {
        case 'date':
          comparison = new Date(a.createdAt).getTime() - new Date(b.createdAt).getTime();
          break;
        case 'title':
          comparison = a.title.localeCompare(b.title);
          break;
        case 'duration':
          comparison = a.duration - b.duration;
          break;
      }
      return sortOrder === 'asc' ? comparison : -comparison;
    });

    return result;
  }, [meetings, sortField, sortOrder, filterAnalyzed]);

  const toggleSortOrder = () => {
    setSortOrder((prev) => (prev === 'asc' ? 'desc' : 'asc'));
  };

  return (
    <div className="p-6 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h1 className="text-2xl font-bold text-gray-900 dark:text-white">
            Meeting History
          </h1>
          <p className="text-gray-500 dark:text-gray-400 mt-1">
            {meetings.length} meetings total
          </p>
        </div>
      </div>

      <Card padding="md" className="mb-6">
        <div className="flex flex-wrap items-center gap-4">
          <div className="flex-1 min-w-[200px]">
            <Input
              placeholder="Search meetings..."
              value={searchQuery}
              onChange={(e) => handleSearch(e.target.value)}
              icon={<Search className="w-4 h-4" />}
            />
          </div>

          <Select
            value={sortField}
            onChange={(value) => setSortField(value as SortField)}
            options={[
              { value: 'date', label: 'Sort by Date' },
              { value: 'title', label: 'Sort by Title' },
              { value: 'duration', label: 'Sort by Duration' },
            ]}
          />

          <Button variant="ghost" onClick={toggleSortOrder}>
            {sortOrder === 'asc' ? (
              <SortAsc className="w-4 h-4" />
            ) : (
              <SortDesc className="w-4 h-4" />
            )}
          </Button>

          <Select
            value={filterAnalyzed}
            onChange={(value) => setFilterAnalyzed(value as 'all' | 'yes' | 'no')}
            options={[
              { value: 'all', label: 'All Meetings' },
              { value: 'yes', label: 'Analyzed' },
              { value: 'no', label: 'Not Analyzed' },
            ]}
          />
        </div>
      </Card>

      {loading ? (
        <div className="text-center py-12 text-gray-500">Loading...</div>
      ) : sortedAndFilteredMeetings.length === 0 ? (
        <div className="text-center py-12">
          <FileText className="w-12 h-12 mx-auto mb-4 text-gray-300" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
            No meetings found
          </h3>
          <p className="text-gray-500">
            {searchQuery
              ? 'Try a different search term'
              : 'Start a new meeting to see it here'}
          </p>
        </div>
      ) : (
        <div className="space-y-4">
          {sortedAndFilteredMeetings.map((meeting) => (
            <Card
              key={meeting.id}
              padding="md"
              hover
              onClick={() => handleOpen(meeting)}
            >
              <div className="flex items-center justify-between">
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-3 mb-2">
                    <h3 className="font-semibold text-gray-900 dark:text-white truncate">
                      {meeting.title}
                    </h3>
                    {meeting.isAnalyzed && (
                      <Badge variant="success" size="sm">
                        <Sparkles className="w-3 h-3 mr-1" />
                        Analyzed
                      </Badge>
                    )}
                  </div>

                  <div className="flex items-center gap-4 text-sm text-gray-500">
                    <span className="flex items-center gap-1">
                      <Clock className="w-3 h-3" />
                      {formatRelativeTime(meeting.createdAt)}
                    </span>
                    <span>{formatDate(meeting.createdAt)}</span>
                    <span>{formatDuration(meeting.duration)}</span>
                    <span>{meeting.segments.length} segments</span>
                  </div>

                  {meeting.tags.length > 0 && (
                    <div className="flex items-center gap-2 mt-2">
                      {meeting.tags.slice(0, 3).map((tag) => (
                        <Badge key={tag} variant="default" size="sm">
                          {tag}
                        </Badge>
                      ))}
                      {meeting.tags.length > 3 && (
                        <span className="text-xs text-gray-400">
                          +{meeting.tags.length - 3} more
                        </span>
                      )}
                    </div>
                  )}
                </div>

                <div className="flex items-center gap-2 ml-4">
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(e) => handleExport(meeting, e)}
                    title="Export"
                  >
                    <Download className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(e) => handleDuplicate(meeting.id, e)}
                    title="Duplicate"
                  >
                    <Copy className="w-4 h-4" />
                  </Button>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={(e) => handleDelete(meeting.id, e)}
                    title="Delete"
                    className="text-red-500 hover:text-red-600"
                  >
                    <Trash2 className="w-4 h-4" />
                  </Button>
                </div>
              </div>
            </Card>
          ))}
        </div>
      )}
    </div>
  );
}

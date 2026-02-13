import React, { useState, useCallback } from 'react';
import { Search, Brain, FileText, Database, Clock, ExternalLink } from 'lucide-react';
import { useCortexStore, useMeetingStore } from '@/store';
import { getCortexClient } from '@/services/cortex';
import { searchMeetings } from '@/services/meeting';
import { formatRelativeTime } from '@/utils/format';
import { Button, Input, Card, Badge } from '@/components/ui';
import type { SearchResult, SearchResults } from '@/models';
import { useNavigate } from 'react-router-dom';

export function MemoryView() {
  const navigate = useNavigate();
  const { loadMeeting } = useMeetingStore();
  const { isSearching, setSearching, status } = useCortexStore();
  const [query, setQuery] = useState('');
  const [results, setResults] = useState<SearchResults | null>(null);
  const [localResults, setLocalResults] = useState<SearchResult[]>([]);

  const handleSearch = useCallback(async () => {
    if (!query.trim()) return;

    setSearching(true);

    try {
      const [localMeetings, cortexResults] = await Promise.all([
        searchMeetings(query),
        status === 'connected'
          ? getCortexClient().searchMemory(query)
          : Promise.resolve(null),
      ]);

      const local: SearchResult[] = localMeetings.map((m) => ({
        id: m.id,
        type: 'meeting' as const,
        title: m.title,
        snippet: m.segments
          .slice(0, 3)
          .map((s) => s.text)
          .join(' ')
          .slice(0, 200),
        relevanceScore: 1,
        metadata: {
          duration: m.duration,
          isAnalyzed: m.isAnalyzed,
          segmentsCount: m.segments.length,
        },
        source: 'local',
        timestamp: m.createdAt,
      }));

      setLocalResults(local);

      if (cortexResults) {
        setResults({
          ...cortexResults,
          sources: {
            ...cortexResults.sources,
            local: local.length,
          },
        });
      } else {
        setResults({
          query,
          results: [],
          totalCount: local.length,
          sources: {
            local: local.length,
            cortexMemory: 0,
            cortexKnowledge: 0,
          },
          searchDuration: 0,
        });
      }
    } catch (err) {
      console.error('Search failed:', err);
    } finally {
      setSearching(false);
    }
  }, [query, status, setSearching]);

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSearch();
    }
  };

  const handleOpenMeeting = async (meetingId: string) => {
    const { getMeeting } = await import('@/services/meeting');
    const meeting = await getMeeting(meetingId);
    if (meeting) {
      loadMeeting(meeting);
      navigate('/meeting');
    }
  };

  const allResults = [
    ...localResults,
    ...(results?.results || []),
  ].sort((a, b) => b.relevanceScore - a.relevanceScore);

  const getSourceIcon = (source: string) => {
    switch (source) {
      case 'local':
        return <FileText className="w-4 h-4" />;
      case 'cortex-memory':
        return <Brain className="w-4 h-4" />;
      case 'cortex-knowledge':
        return <Database className="w-4 h-4" />;
      default:
        return <FileText className="w-4 h-4" />;
    }
  };

  const getSourceLabel = (source: string) => {
    switch (source) {
      case 'local':
        return 'Local Meeting';
      case 'cortex-memory':
        return 'Cortex Memory';
      case 'cortex-knowledge':
        return 'Knowledge Base';
      default:
        return source;
    }
  };

  return (
    <div className="p-6 max-w-4xl mx-auto">
      <div className="mb-8">
        <h1 className="text-2xl font-bold text-gray-900 dark:text-white mb-2">
          Memory Search
        </h1>
        <p className="text-gray-500 dark:text-gray-400">
          Search across your meetings, Cortex memory, and knowledge base
        </p>
      </div>

      <Card padding="lg" className="mb-8">
        <div className="flex gap-4">
          <div className="flex-1">
            <Input
              value={query}
              onChange={(e) => setQuery(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder="Ask anything about your meetings..."
              icon={<Search className="w-4 h-4" />}
              className="text-lg"
            />
          </div>
          <Button
            variant="primary"
            onClick={handleSearch}
            loading={isSearching}
            disabled={!query.trim()}
          >
            Search
          </Button>
        </div>

        <div className="flex items-center gap-4 mt-4 text-sm text-gray-500">
          <div className="flex items-center gap-2">
            <FileText className="w-4 h-4" />
            <span>Local Meetings</span>
          </div>
          {status === 'connected' ? (
            <>
              <div className="flex items-center gap-2">
                <Brain className="w-4 h-4 text-green-500" />
                <span>Cortex Memory</span>
              </div>
              <div className="flex items-center gap-2">
                <Database className="w-4 h-4 text-green-500" />
                <span>Knowledge Base</span>
              </div>
            </>
          ) : (
            <span className="text-amber-500">
              Cortex offline - searching local only
            </span>
          )}
        </div>
      </Card>

      {results && (
        <div className="mb-6 flex items-center justify-between">
          <div className="text-sm text-gray-500">
            Found {allResults.length} results
            {results.searchDuration > 0 && ` in ${results.searchDuration}ms`}
          </div>
          <div className="flex items-center gap-4 text-sm">
            <Badge variant="default">
              <FileText className="w-3 h-3 mr-1" />
              {results.sources.local} local
            </Badge>
            <Badge variant="primary">
              <Brain className="w-3 h-3 mr-1" />
              {results.sources.cortexMemory} memory
            </Badge>
            <Badge variant="success">
              <Database className="w-3 h-3 mr-1" />
              {results.sources.cortexKnowledge} knowledge
            </Badge>
          </div>
        </div>
      )}

      {allResults.length > 0 ? (
        <div className="space-y-4">
          {allResults.map((result) => (
            <Card
              key={result.id}
              padding="md"
              hover
              onClick={() => {
                if (result.source === 'local') {
                  handleOpenMeeting(result.id);
                }
              }}
            >
              <div className="flex items-start gap-4">
                <div className="p-2 rounded-lg bg-gray-100 dark:bg-surface-800 text-gray-500">
                  {getSourceIcon(result.source)}
                </div>

                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2 mb-1">
                    <h3 className="font-medium text-gray-900 dark:text-white truncate">
                      {result.title}
                    </h3>
                    <Badge variant="default" size="sm">
                      {getSourceLabel(result.source)}
                    </Badge>
                  </div>

                  <p className="text-sm text-gray-600 dark:text-gray-400 line-clamp-2">
                    {result.snippet}
                  </p>

                  <div className="flex items-center gap-4 mt-2 text-xs text-gray-500">
                    <span className="flex items-center gap-1">
                      <Clock className="w-3 h-3" />
                      {formatRelativeTime(result.timestamp)}
                    </span>
                    <span>
                      Relevance: {Math.round(result.relevanceScore * 100)}%
                    </span>
                  </div>
                </div>

                {result.source === 'local' && (
                  <Button variant="ghost" size="sm">
                    <ExternalLink className="w-4 h-4" />
                  </Button>
                )}
              </div>
            </Card>
          ))}
        </div>
      ) : results ? (
        <div className="text-center py-12">
          <Search className="w-12 h-12 mx-auto mb-4 text-gray-300" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
            No results found
          </h3>
          <p className="text-gray-500">Try different keywords or a broader search</p>
        </div>
      ) : (
        <div className="text-center py-12">
          <Brain className="w-12 h-12 mx-auto mb-4 text-gray-300" />
          <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
            Search Your Memory
          </h3>
          <p className="text-gray-500 max-w-md mx-auto">
            Ask questions about your meetings, find action items, or search through
            your knowledge base. Try queries like "What did we decide about the
            project timeline?" or "Find all tasks assigned to John"
          </p>
        </div>
      )}
    </div>
  );
}

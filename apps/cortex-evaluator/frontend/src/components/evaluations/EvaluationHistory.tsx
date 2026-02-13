import { useEffect, useState } from 'react';
import { useEvaluationHistory, useEvaluationSearch } from '../../hooks/useEvaluationHistory';
import EvaluationCard from './EvaluationCard';
import SearchBar from '../shared/SearchBar';

interface EvaluationHistoryProps {
  projectId?: string;
  onSelectEvaluation?: (evaluationId: string) => void;
}

export default function EvaluationHistory({ projectId, onSelectEvaluation }: EvaluationHistoryProps) {
  const { evaluations, loading, error, total, page, pageSize, fetchPage, nextPage, prevPage } = useEvaluationHistory(projectId, 30);
  const { results: searchResults, loading: searchLoading, search } = useEvaluationSearch(projectId, 10);
  const [searchMode, setSearchMode] = useState(false);
  const [filters, setFilters] = useState({
    dateFrom: '',
    dateTo: '',
    provider: '',
    minScore: 0,
    maxScore: 100,
  });

  useEffect(() => {
    fetchPage(0, 30);
  }, [projectId, fetchPage]);

  const handleSearch = (query: string, semantic: boolean) => {
    if (query.trim()) {
      setSearchMode(true);
      search(query, semantic, filters, 30);
    } else {
      setSearchMode(false);
    }
  };

  const handleFilterChange = (newFilters: typeof filters) => {
    setFilters(newFilters);
    if (searchMode) {
      const searchQuery = (document.querySelector('input[placeholder="Search evaluations..."]') as HTMLInputElement)?.value || '';
      if (searchQuery.trim()) {
        search(searchQuery, true, newFilters, 30);
      }
    }
  };

  const handlePageChange = (newPage: number) => {
    fetchPage(newPage, 30);
  };

  const displayEvaluations = searchMode ? searchResults as any[] : evaluations;
  const displayLoading = loading || searchLoading;
  const displayTotal = searchMode ? searchResults.length : total;
  const displayPage = searchMode ? 0 : page;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-4">
        <SearchBar
          onSearch={handleSearch}
          placeholder="Search evaluations..."
          showSemanticToggle
          showFilters
          filters={filters}
          onFilterChange={handleFilterChange}
        />

        {searchMode && (
          <div className="flex items-center gap-2 text-sm text-gray-600">
            <span>Found {displayTotal} results</span>
            <button
              onClick={() => setSearchMode(false)}
              className="text-blue-600 hover:text-blue-700"
            >
              Clear search
            </button>
          </div>
        )}
      </div>

      {displayLoading && evaluations.length === 0 ? (
        <div className="text-center py-12">
          <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
          <p className="mt-2 text-sm text-gray-600">Loading evaluations...</p>
        </div>
      ) : error ? (
        <div className="bg-red-50 border border-red-200 rounded-lg p-4">
          <p className="text-red-600 text-sm">{error}</p>
        </div>
      ) : displayEvaluations.length === 0 ? (
        <div className="text-center py-12 text-gray-500">
          <svg className="mx-auto h-12 w-12 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
          </svg>
          <p className="mt-2 text-sm font-medium">No evaluations found</p>
          <p className="text-xs text-gray-400 mt-1">
            {searchMode ? 'Try different search terms' : 'Run your first evaluation to see it here'}
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {displayEvaluations.map((evaluation) => (
            <EvaluationCard
              key={evaluation.id}
              evaluation={evaluation}
              onClick={() => onSelectEvaluation?.(evaluation.id)}
            />
          ))}
        </div>
      )}

      {!searchMode && displayTotal > pageSize && (
        <div className="flex items-center justify-between pt-4 border-t border-gray-200">
          <button
            onClick={() => prevPage()}
            disabled={displayPage === 0}
            className="px-3 py-1.5 text-sm border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            Previous
          </button>
          <span className="text-sm text-gray-600">
            Page {displayPage + 1} of {Math.ceil(displayTotal / pageSize)}
          </span>
          <button
            onClick={() => nextPage()}
            disabled={displayPage >= Math.ceil(displayTotal / pageSize) - 1}
            className="px-3 py-1.5 text-sm border border-gray-300 rounded-lg hover:bg-gray-50 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}

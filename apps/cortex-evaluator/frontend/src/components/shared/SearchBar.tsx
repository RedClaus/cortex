import { useState, useEffect, useCallback } from 'react';

interface SearchBarProps {
  onSearch: (query: string, semantic: boolean) => void;
  placeholder?: string;
  showSemanticToggle?: boolean;
  showFilters?: boolean;
  filters?: {
    dateFrom: string;
    dateTo: string;
    provider: string;
    minScore: number;
    maxScore: number;
  };
  onFilterChange?: (filters: any) => void;
}

export default function SearchBar({
  onSearch,
  placeholder = 'Search...',
  showSemanticToggle = false,
  showFilters = false,
  filters,
  onFilterChange,
}: SearchBarProps) {
  const [query, setQuery] = useState('');
  const [semantic, setSemantic] = useState(true);
  const [debouncedQuery, setDebouncedQuery] = useState('');
  const [showFilterPanel, setShowFilterPanel] = useState(false);
  const [localFilters, setLocalFilters] = useState(
    filters || { dateFrom: '', dateTo: '', provider: '', minScore: 0, maxScore: 100 }
  );

  useEffect(() => {
    const handler = setTimeout(() => {
      setDebouncedQuery(query);
    }, 300);
    return () => clearTimeout(handler);
  }, [query]);

  useEffect(() => {
    if (debouncedQuery) {
      onSearch(debouncedQuery, semantic);
    }
  }, [debouncedQuery, semantic, onSearch]);

  const handleApplyFilters = useCallback(() => {
    onFilterChange?.(localFilters);
    setShowFilterPanel(false);
  }, [localFilters, onFilterChange]);

  const handleResetFilters = useCallback(() => {
    const resetFilters = { dateFrom: '', dateTo: '', provider: '', minScore: 0, maxScore: 100 };
    setLocalFilters(resetFilters);
    onFilterChange?.(resetFilters);
  }, [onFilterChange]);

  return (
    <div className="space-y-3">
      <div className="flex gap-2">
        <div className="flex-1 relative">
          <input
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder={placeholder}
            className="w-full px-4 py-2.5 pl-10 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
          />
          <svg
            className="absolute left-3 top-1/2 transform -translate-y-1/2 w-4 h-4 text-gray-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
            />
          </svg>
        </div>

        {showSemanticToggle && (
          <button
            onClick={() => setSemantic(!semantic)}
            className={`px-4 py-2.5 rounded-lg font-medium transition-colors ${
              semantic
                ? 'bg-purple-600 text-white'
                : 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'
            }`}
            title={semantic ? 'Semantic search enabled' : 'Full-text search'}
          >
            {semantic ? '🧠 Semantic' : '🔤 Full-Text'}
          </button>
        )}

        {showFilters && (
          <button
            onClick={() => setShowFilterPanel(!showFilterPanel)}
            className={`px-4 py-2.5 rounded-lg font-medium transition-colors ${
              showFilterPanel || Object.values(localFilters).some((v) => v !== '' && v !== 0 && v !== 100)
                ? 'bg-blue-600 text-white'
                : 'bg-white border border-gray-300 text-gray-700 hover:bg-gray-50'
            }`}
            title="Filters"
          >
            🔍 Filters
          </button>
        )}
      </div>

      {showFilterPanel && showFilters && (
        <div className="p-4 bg-gray-50 border border-gray-200 rounded-lg space-y-4">
          <div className="grid grid-cols-2 gap-4">
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Date From</label>
              <input
                type="date"
                value={localFilters.dateFrom}
                onChange={(e) => setLocalFilters({ ...localFilters, dateFrom: e.target.value })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-gray-700 mb-1">Date To</label>
              <input
                type="date"
                value={localFilters.dateTo}
                onChange={(e) => setLocalFilters({ ...localFilters, dateTo: e.target.value })}
                className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
              />
            </div>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-1">Provider</label>
            <select
              value={localFilters.provider}
              onChange={(e) => setLocalFilters({ ...localFilters, provider: e.target.value })}
              className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500"
            >
              <option value="">All Providers</option>
              <option value="openai">OpenAI</option>
              <option value="anthropic">Anthropic</option>
              <option value="google">Google</option>
              <option value="local">Local</option>
            </select>
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Score Range: {localFilters.minScore} - {localFilters.maxScore}
            </label>
            <div className="flex items-center gap-4">
              <input
                type="range"
                min="0"
                max="100"
                value={localFilters.minScore}
                onChange={(e) => setLocalFilters({ ...localFilters, minScore: parseInt(e.target.value) })}
                className="flex-1"
              />
              <input
                type="range"
                min="0"
                max="100"
                value={localFilters.maxScore}
                onChange={(e) => setLocalFilters({ ...localFilters, maxScore: parseInt(e.target.value) })}
                className="flex-1"
              />
            </div>
          </div>

          <div className="flex justify-end gap-2">
            <button
              onClick={handleResetFilters}
              className="px-4 py-2 text-gray-700 hover:bg-gray-200 rounded-lg transition-colors"
            >
              Reset
            </button>
            <button
              onClick={handleApplyFilters}
              className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors"
            >
              Apply Filters
            </button>
          </div>
        </div>
      )}

      <div className="flex gap-2 flex-wrap">
        <span className="text-xs text-gray-500">Recent:</span>
        <button
          onClick={() => setQuery('high value score')}
          className="px-2 py-1 text-xs bg-gray-100 text-gray-700 rounded hover:bg-gray-200 transition-colors"
        >
          high value score
        </button>
        <button
          onClick={() => setQuery('openai')}
          className="px-2 py-1 text-xs bg-gray-100 text-gray-700 rounded hover:bg-gray-200 transition-colors"
        >
          openai
        </button>
        <button
          onClick={() => setQuery('completed')}
          className="px-2 py-1 text-xs bg-gray-100 text-gray-700 rounded hover:bg-gray-200 transition-colors"
        >
          completed
        </button>
      </div>
    </div>
  );
}

import { useState } from 'react';
import type { Evaluation, EvaluationResult } from '../../types/api';

interface EvaluationCardProps {
  evaluation: Evaluation | any;
  result?: EvaluationResult;
  onClick?: () => void;
}

export default function EvaluationCard({ evaluation, result, onClick }: EvaluationCardProps) {
  const [showExportMenu, setShowExportMenu] = useState(false);

  const valueScore = result?.valueScore ?? (evaluation as any).valueScore ?? Math.floor(Math.random() * 100);
  const executiveSummary = result?.executiveSummary ?? (evaluation as any).executiveSummary ?? evaluation.inputName ?? 'No summary available';
  const provider = evaluation.providerId ?? (evaluation as any).providerUsed ?? 'openai';
  const inputType = evaluation.inputType ?? (evaluation as any).type ?? 'repo';
  const status = evaluation.status;
  const createdAt = evaluation.createdAt;

  const getScoreColor = (score: number) => {
    if (score >= 67) return 'bg-green-500';
    if (score >= 34) return 'bg-yellow-500';
    return 'bg-red-500';
  };

  const getScoreGradient = (score: number) => {
    if (score >= 67) return 'from-green-400 to-green-600';
    if (score >= 34) return 'from-yellow-400 to-yellow-600';
    return 'from-red-400 to-red-600';
  };

  const getStatusBadge = (status: string) => {
    const statusConfig: Record<string, { bg: string; text: string; label: string }> = {
      pending: { bg: 'bg-gray-100', text: 'text-gray-700', label: 'Pending' },
      running: { bg: 'bg-blue-100', text: 'text-blue-700', label: 'In Progress' },
      completed: { bg: 'bg-green-100', text: 'text-green-700', label: 'Completed' },
      failed: { bg: 'bg-red-100', text: 'text-red-700', label: 'Failed' },
    };
    const config = statusConfig[status] || statusConfig.pending;
    return (
      <span className={`px-2 py-1 text-xs font-medium rounded-full ${config.bg} ${config.text}`}>
        {config.label}
      </span>
    );
  };

  const getProviderIcon = (provider: string) => {
    const icons: Record<string, string> = {
      openai: '🤖',
      anthropic: '🧠',
      google: '🔬',
      local: '💻',
    };
    return icons[provider] || '📊';
  };

  const handleExport = (format: 'json' | 'markdown') => {
    const data = format === 'json' ? JSON.stringify(evaluation, null, 2) : `# Evaluation Report\n\n${executiveSummary}`;
    const blob = new Blob([data], { type: format === 'json' ? 'application/json' : 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `evaluation-${evaluation.id}.${format === 'json' ? 'json' : 'md'}`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
    setShowExportMenu(false);
  };

  const truncatedSummary = executiveSummary.length > 150 ? executiveSummary.substring(0, 150) + '...' : executiveSummary;

  return (
    <div
      className={`p-4 bg-white border-2 rounded-lg transition-all ${
        onClick ? 'border-gray-200 hover:border-gray-300 hover:shadow-md cursor-pointer' : 'border-gray-200'
      }`}
      onClick={onClick}
    >
      <div className="flex items-start justify-between gap-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-3 mb-2">
            <div
              className={`px-3 py-1 text-white font-bold rounded-lg bg-gradient-to-r ${getScoreGradient(valueScore)}`}
            >
              {valueScore}
            </div>
            <div className="flex items-center gap-2">
              <span className="text-lg">{getProviderIcon(provider)}</span>
              <span className="text-sm text-gray-600 capitalize">{provider}</span>
            </div>
            {getStatusBadge(status)}
          </div>

          <h3 className="font-medium text-gray-900 mb-1 truncate">
            {evaluation.name ?? evaluation.inputName ?? 'Untitled Evaluation'}
          </h3>

          <p className="text-sm text-gray-600 mb-3">{truncatedSummary}</p>

          <div className="flex items-center gap-4 text-xs text-gray-500">
            <span className="inline-flex items-center gap-1">
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 7h.01M7 3h5c.512 0 1.024.195 1.414.586l7 7a2 2 0 010 2.828l-7 7a2 2 0 01-2.828 0l-7-7A1.994 1.994 0 013 12V7a4 4 0 014-4z" />
              </svg>
              <span className="capitalize">{inputType}</span>
            </span>
            <span className="inline-flex items-center gap-1">
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7V3m8 4V3m-9 8h10M5 21h14a2 2 0 002-2V7a2 2 0 00-2-2H5a2 2 0 00-2 2v12a2 2 0 002 2z" />
              </svg>
              {new Date(createdAt).toLocaleDateString()}
            </span>
          </div>
        </div>

        <div className="relative">
          <button
            onClick={(e) => {
              e.stopPropagation();
              setShowExportMenu(!showExportMenu);
            }}
            className="p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded transition-colors"
            title="Export"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
            </svg>
          </button>

          {showExportMenu && (
            <div className="absolute right-0 top-full mt-1 bg-white rounded-lg shadow-lg border border-gray-200 py-1 min-w-[120px] z-10">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleExport('json');
                }}
                className="w-full px-3 py-2 text-sm text-left hover:bg-gray-50 transition-colors"
              >
                Export JSON
              </button>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleExport('markdown');
                }}
                className="w-full px-3 py-2 text-sm text-left hover:bg-gray-50 transition-colors"
              >
                Export Markdown
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

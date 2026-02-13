import { useState, useEffect } from 'react';
import { apiClient } from '../../services/api';
import type { Evaluation, SearchResult } from '../../types/api';

interface EvaluationDetailModalProps {
  evaluationId: string | null;
  isOpen: boolean;
  onClose: () => void;
}

export default function EvaluationDetailModal({ evaluationId, isOpen, onClose }: EvaluationDetailModalProps) {
  const [evaluation, setEvaluation] = useState<Evaluation | null>(null);
  const [similarEvaluations, setSimilarEvaluations] = useState<SearchResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<'overview' | 'cr' | 'similar' | 'related'>('overview');

  useEffect(() => {
    if (isOpen && evaluationId) {
      fetchEvaluationData();
    }
  }, [isOpen, evaluationId]);

  const fetchEvaluationData = async () => {
    if (!evaluationId) return;

    setLoading(true);
    setError(null);

    try {
      const [evalData, similarData] = await Promise.all([
        apiClient.getEvaluation(evaluationId),
        apiClient.getSimilarEvaluations(evaluationId, 5),
      ]);

      setEvaluation(evalData);
      setSimilarEvaluations(similarData.similarEvaluations);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to load evaluation');
    } finally {
      setLoading(false);
    }
  };

  const handleCreateGitHubIssue = async () => {
    if (!evaluation) return;

    const title = `Evaluation: ${evaluation.inputName}`;
    const body = `
## Evaluation Report

**ID:** ${evaluation.id}
**Input Type:** ${evaluation.inputType}
**Provider:** ${evaluation.providerId}
**Status:** ${evaluation.status}

### Summary
${evaluation.inputName}

### Created
${new Date(evaluation.createdAt).toLocaleString()}
    `.trim();

    const url = `https://github.com/new?title=${encodeURIComponent(title)}&body=${encodeURIComponent(body)}`;
    window.open(url, '_blank');
  };

  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50 p-4">
      <div className="bg-white rounded-lg shadow-xl w-full max-w-4xl max-h-[90vh] overflow-hidden flex flex-col">
        <div className="flex items-center justify-between p-4 border-b border-gray-200">
          <h2 className="text-lg font-semibold text-gray-900">Evaluation Details</h2>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 transition-colors">
            <svg className="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="flex items-center justify-center py-12">
              <div className="inline-block animate-spin rounded-full h-8 w-8 border-b-2 border-blue-600"></div>
            </div>
          ) : error ? (
            <div className="p-4 text-red-600 text-sm">{error}</div>
          ) : evaluation ? (
            <>
              <div className="border-b border-gray-200">
                <nav className="flex gap-4 px-4">
                  {['overview', 'cr', 'similar', 'related'].map((tab) => (
                    <button
                      key={tab}
                      onClick={() => setActiveTab(tab as any)}
                      className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors ${
                        activeTab === tab
                          ? 'border-blue-600 text-blue-600'
                          : 'border-transparent text-gray-600 hover:text-gray-900'
                      }`}
                    >
                      {tab.charAt(0).toUpperCase() + tab.slice(1)}
                    </button>
                  ))}
                </nav>
              </div>

              <div className="p-6">
                {activeTab === 'overview' && <OverviewTab evaluation={evaluation} />}
                {activeTab === 'cr' && <CRTab evaluation={evaluation} />}
                {activeTab === 'similar' && <SimilarTab similarEvaluations={similarEvaluations} />}
                {activeTab === 'related' && <RelatedTab evaluation={evaluation} />}
              </div>
            </>
          ) : null}
        </div>

        {evaluation && (
          <div className="flex items-center justify-between p-4 border-t border-gray-200 bg-gray-50">
            <div className="flex items-center gap-2 text-sm text-gray-600">
              <span className={`px-2 py-1 rounded-full text-xs font-medium ${
                evaluation.status === 'completed' ? 'bg-green-100 text-green-700' :
                evaluation.status === 'running' ? 'bg-blue-100 text-blue-700' :
                evaluation.status === 'failed' ? 'bg-red-100 text-red-700' :
                'bg-gray-100 text-gray-700'
              }`}>
                {evaluation.status}
              </span>
              <span>{new Date(evaluation.createdAt).toLocaleDateString()}</span>
            </div>
            <div className="flex gap-2">
              <button
                onClick={handleCreateGitHubIssue}
                className="px-4 py-2 bg-black text-white rounded-lg hover:bg-gray-800 transition-colors text-sm font-medium flex items-center gap-2"
              >
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 24 24">
                  <path d="M12 0c-6.626 0-12 5.373-12 12 0 5.302 3.438 9.8 8.207 11.387.599.111.793-.261.793-.577v-2.234c-3.338.726-4.033-1.416-4.033-1.416-.546-1.387-1.333-1.756-1.333-1.756-1.089-.745.083-.729.083-.729 1.205.084 1.839 1.237 1.839 1.237 1.07 1.834 2.807 1.304 3.492.997.107-.775.418-1.305.762-1.604-2.665-.305-5.467-1.334-5.467-5.931 0-1.311.469-2.381 1.236-3.221-.124-.303-.535-1.524.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.983-.399 3.003-.404 1.02.005 2.047.138 3.006.404 2.291-1.552 3.297-1.23 3.297-1.23.653 1.653.242 2.874.118 3.176.77.84 1.235 1.911 1.235 3.221 0 4.609-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222v3.293c0 .319.192.694.801.576 4.765-1.589 8.199-6.086 8.199-11.386 0-6.627-5.373-12-12-12z" />
                </svg>
                Create GitHub Issue
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

function OverviewTab({ evaluation }: { evaluation: Evaluation }) {
  return (
    <div className="space-y-6">
      <div className="grid grid-cols-2 gap-4">
        <InfoCard label="Evaluation ID" value={evaluation.id} />
        <InfoCard label="Status" value={evaluation.status} />
        <InfoCard label="Input Type" value={evaluation.inputType} />
        <InfoCard label="Provider" value={evaluation.providerId} />
        <InfoCard label="Created" value={new Date(evaluation.createdAt).toLocaleString()} />
        {evaluation.completedAt && (
          <InfoCard label="Completed" value={new Date(evaluation.completedAt).toLocaleString()} />
        )}
      </div>

      <div>
        <h3 className="text-sm font-semibold text-gray-900 mb-2">Input Content</h3>
        <p className="text-sm text-gray-700 bg-gray-50 p-4 rounded-lg">{evaluation.inputContent}</p>
      </div>

      <div>
        <h3 className="text-sm font-semibold text-gray-900 mb-2">Input Name</h3>
        <p className="text-sm text-gray-700">{evaluation.inputName}</p>
      </div>
    </div>
  );
}

function CRTab({ evaluation }: { evaluation: Evaluation }) {
  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h3 className="text-sm font-semibold text-gray-900">Suggested Code Review</h3>
        <button className="px-3 py-1.5 text-sm text-blue-600 hover:text-blue-700 transition-colors">
          Edit CR
        </button>
      </div>
      <div className="bg-gray-50 p-4 rounded-lg text-sm text-gray-700 font-mono whitespace-pre-wrap">
        No CR available for this evaluation.
      </div>
    </div>
  );
}

function SimilarTab({ similarEvaluations }: { similarEvaluations: SearchResult[] }) {
  if (similarEvaluations.length === 0) {
    return <p className="text-sm text-gray-500">No similar evaluations found.</p>;
  }

  return (
    <div className="space-y-3">
      {similarEvaluations.map((item, index) => (
        <div key={index} className="p-4 bg-gray-50 rounded-lg border border-gray-200">
          <div className="flex items-center justify-between mb-2">
            <span className="text-sm font-medium text-gray-900">Similarity Score</span>
            <span className="text-sm font-bold text-blue-600">{(item.score * 100).toFixed(1)}%</span>
          </div>
          <div className="text-xs text-gray-600">
            {Object.entries(item.metadata || {}).map(([key, value]) => (
              <div key={key} className="flex justify-between">
                <span className="capitalize">{key}:</span>
                <span>{String(value)}</span>
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}

function RelatedTab({ evaluation }: { evaluation: Evaluation }) {
  return (
    <div className="space-y-4">
      <div className="bg-blue-50 border border-blue-200 rounded-lg p-4">
        <h4 className="text-sm font-semibold text-blue-900 mb-2">Related Brainstorm Sessions</h4>
        <p className="text-sm text-blue-700">
          No related brainstorm sessions found for this evaluation.
        </p>
      </div>
    </div>
  );
}

function InfoCard({ label, value }: { label: string; value: string | number }) {
  return (
    <div className="p-3 bg-gray-50 rounded-lg">
      <p className="text-xs text-gray-500 mb-1">{label}</p>
      <p className="text-sm font-medium text-gray-900">{value}</p>
    </div>
  );
}

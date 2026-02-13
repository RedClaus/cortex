import { useState, useEffect } from 'react';
import { DetailedCR, CRTemplate } from '../../types/cr';
import { CR_TEMPLATES, getTemplateById } from '../../data/crTemplates';
import { formatCR } from '../../services/crFormatter';
import { generateBreakdown, createIssue, BreakdownRequest, CreateIssueRequest } from '../../services/crService';

export function CREditor() {
  const [selectedTemplate, setSelectedTemplate] = useState<string>('claude-code');
  const [analysisResult, setAnalysisResult] = useState<string>('');
  const [cr, setCR] = useState<DetailedCR | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const template: CRTemplate | undefined = getTemplateById(selectedTemplate);

  useEffect(() => {
    if (cr && template) {
      const formatted = formatCR(cr, template);
      setCR(prev => prev ? { ...prev, formatted_output: formatted } : null);
    }
  }, [selectedTemplate, cr, template]);

  const handleGenerateBreakdown = async () => {
    if (!analysisResult.trim()) {
      setError('Please provide analysis result');
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const request: BreakdownRequest = {
        executive_summary: analysisResult,
        suggested_cr: analysisResult,
      };

      const result = await generateBreakdown(request);
      setCR(result);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to generate breakdown');
    } finally {
      setIsLoading(false);
    }
  };

  const handleCreateIssue = async (platform: 'github' | 'jira' | 'linear') => {
    if (!cr) return;

    setIsLoading(true);
    setError(null);

    try {
      const request: CreateIssueRequest = {
        platform,
        title: cr.summary,
        body: cr.formatted_output,
        metadata: {
          labels: [cr.type, cr.tasks[0]?.priority].filter(Boolean),
          priority: cr.tasks[0]?.priority,
          story_points: cr.estimation.complexity,
        },
      };

      const result = await createIssue(request);
      alert(`Issue created: ${result.url}`);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Failed to create issue');
    } finally {
      setIsLoading(false);
    }
  };

  const handleExportToFile = () => {
    if (!cr) return;

    const blob = new Blob([cr.formatted_output], { type: 'text/markdown' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${cr.summary.replace(/\s+/g, '-').toLowerCase()}.md`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  const handleCopyToClipboard = () => {
    if (!cr) return;
    navigator.clipboard.writeText(cr.formatted_output);
  };

  return (
    <div className="flex h-full">
      <div className="w-1/2 p-6 border-r border-gray-200 overflow-y-auto">
        <h1 className="text-2xl font-bold mb-6">CR Editor</h1>

        <div className="space-y-6">
          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Template
            </label>
            <select
              value={selectedTemplate}
              onChange={(e) => setSelectedTemplate(e.target.value)}
              className="w-full p-2 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            >
              {CR_TEMPLATES.map((t) => (
                <option key={t.id} value={t.id}>
                  {t.name}
                </option>
              ))}
            </select>
            {template && (
              <p className="text-sm text-gray-500 mt-1">{template.description}</p>
            )}
          </div>

          <div>
            <label className="block text-sm font-medium text-gray-700 mb-2">
              Analysis Result / Topic
            </label>
            <textarea
              value={analysisResult}
              onChange={(e) => setAnalysisResult(e.target.value)}
              placeholder="Paste analysis result or describe the change request topic..."
              rows={8}
              className="w-full p-3 border border-gray-300 rounded-md focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
            />
          </div>

          <button
            onClick={handleGenerateBreakdown}
            disabled={isLoading || !analysisResult.trim()}
            className="w-full bg-blue-600 text-white py-2 px-4 rounded-md hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
          >
            {isLoading ? 'Generating...' : 'Generate CR Breakdown'}
          </button>

          {error && (
            <div className="p-3 bg-red-100 border border-red-400 text-red-700 rounded">
              {error}
            </div>
          )}
        </div>
      </div>

      <div className="w-1/2 p-6 overflow-y-auto bg-gray-50">
        {cr ? (
          <div className="space-y-6">
            <div className="flex items-center justify-between">
              <h2 className="text-xl font-bold">Preview</h2>
              <div className="flex gap-2">
                <button
                  onClick={handleCopyToClipboard}
                  className="px-3 py-1 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 transition-colors text-sm"
                >
                  Copy
                </button>
                <button
                  onClick={handleExportToFile}
                  className="px-3 py-1 bg-gray-200 text-gray-700 rounded hover:bg-gray-300 transition-colors text-sm"
                >
                  Export
                </button>
              </div>
            </div>

            <div className="space-y-3">
              <button
                onClick={() => handleCreateIssue('github')}
                disabled={isLoading}
                className="w-full bg-gray-900 text-white py-2 px-4 rounded-md hover:bg-gray-800 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
              >
                Create GitHub Issue
              </button>
              <div className="flex gap-2">
                <button
                  onClick={() => handleCreateIssue('jira')}
                  disabled={isLoading}
                  className="flex-1 bg-blue-600 text-white py-2 px-4 rounded-md hover:bg-blue-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
                >
                  Create Jira Epic
                </button>
                <button
                  onClick={() => handleCreateIssue('linear')}
                  disabled={isLoading}
                  className="flex-1 bg-purple-600 text-white py-2 px-4 rounded-md hover:bg-purple-700 disabled:bg-gray-400 disabled:cursor-not-allowed transition-colors"
                >
                  Create Linear Ticket
                </button>
              </div>
            </div>

            <div className="bg-white p-6 rounded-lg border border-gray-200 shadow-sm">
              <pre className="whitespace-pre-wrap text-sm font-mono text-gray-800">
                {cr.formatted_output}
              </pre>
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-center h-full text-gray-400">
            Generate a CR breakdown to see preview
          </div>
        )}
      </div>
    </div>
  );
}

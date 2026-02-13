import { useState } from 'react';
import { clsx } from 'clsx';
import {
  Sparkles,
  ListChecks,
  MessageSquare,
  AlertTriangle,
  TrendingUp,
  ChevronRight,
  Upload,
  Database,
} from 'lucide-react';
import { useMeetingStore, useCortexStore, useTasksStore } from '@/store';
import { getCortexClient } from '@/services/cortex';
import type { MemoryCommitPayload, KnowledgeIngestPayload } from '@/models';
import { Button, Badge, Card } from '@/components/ui';

type Tab = 'summary' | 'actions' | 'decisions' | 'risks';

export function AnalysisPanel() {
  const { currentMeeting } = useMeetingStore();
  const { isCommitting, isIngesting, setCommitting, setIngesting } = useCortexStore();
  const { importTasks } = useTasksStore();
  const [activeTab, setActiveTab] = useState<Tab>('summary');
  const [commitSuccess, setCommitSuccess] = useState(false);
  const [ingestSuccess, setIngestSuccess] = useState(false);

  const analysis = currentMeeting?.analysis;

  const handleCommitMemory = async () => {
    if (!currentMeeting || !analysis) return;

    setCommitting(true);
    try {
      const client = getCortexClient();
      const payload: MemoryCommitPayload = {
        meetingId: currentMeeting.id,
        title: currentMeeting.title,
        summary: analysis.summary,
        decisions: analysis.decisions,
        actionItems: analysis.actionItems,
        keyPoints: analysis.keyPoints,
        tags: currentMeeting.tags,
        redactionsApplied: false,
        timestamp: new Date().toISOString(),
        participants: currentMeeting.participants.map((p) => p.name),
      };
      await client.commitMemory(payload);
      setCommitSuccess(true);
      setTimeout(() => setCommitSuccess(false), 3000);
    } catch (err) {
      console.error('Memory commit failed:', err);
    } finally {
      setCommitting(false);
    }
  };

  const handleIngestKnowledge = async () => {
    if (!currentMeeting) return;

    setIngesting(true);
    try {
      const client = getCortexClient();
      const fullText = currentMeeting.segments.map((s) => s.text).join(' ');
      const payload: KnowledgeIngestPayload = {
        meetingId: currentMeeting.id,
        title: currentMeeting.title,
        participants: currentMeeting.participants,
        fullText,
        segments: currentMeeting.segments,
        metadata: {
          duration: currentMeeting.duration,
          isAnalyzed: currentMeeting.isAnalyzed,
        },
        tags: currentMeeting.tags,
        timestamp: new Date().toISOString(),
      };
      await client.ingestKnowledge(payload);
      setIngestSuccess(true);
      setTimeout(() => setIngestSuccess(false), 3000);
    } catch (err) {
      console.error('Knowledge ingest failed:', err);
    } finally {
      setIngesting(false);
    }
  };

  const handleImportTasks = () => {
    if (analysis?.actionItems) {
      importTasks(analysis.actionItems);
    }
  };

  if (!analysis) {
    return (
      <div className="flex flex-col items-center justify-center h-full p-6 text-center">
        <Sparkles className="w-12 h-12 text-gray-300 dark:text-gray-600 mb-4" />
        <h3 className="text-lg font-medium text-gray-900 dark:text-white mb-2">
          No Analysis Yet
        </h3>
        <p className="text-sm text-gray-500 dark:text-gray-400 max-w-xs">
          Click "Analyze" after recording to get AI-powered insights about your meeting
        </p>
      </div>
    );
  }

  const tabs = [
    { id: 'summary' as Tab, label: 'Summary', icon: MessageSquare, count: null },
    { id: 'actions' as Tab, label: 'Actions', icon: ListChecks, count: analysis.actionItems.length },
    { id: 'decisions' as Tab, label: 'Decisions', icon: TrendingUp, count: analysis.decisions.length },
    { id: 'risks' as Tab, label: 'Risks', icon: AlertTriangle, count: analysis.risks.length },
  ];

  const sentimentColor = {
    positive: 'text-green-500',
    neutral: 'text-gray-500',
    negative: 'text-red-500',
    mixed: 'text-yellow-500',
  };

  return (
    <div className="flex flex-col h-full">
      <div className="flex items-center gap-2 p-3 border-b border-gray-200 dark:border-surface-700 overflow-x-auto">
        {tabs.map((tab) => (
          <button
            key={tab.id}
            onClick={() => setActiveTab(tab.id)}
            className={clsx(
              'flex items-center gap-2 px-3 py-1.5 text-sm font-medium rounded-lg whitespace-nowrap transition-colors',
              activeTab === tab.id
                ? 'bg-primary-50 dark:bg-primary-900/20 text-primary-600 dark:text-primary-400'
                : 'text-gray-600 dark:text-gray-400 hover:bg-gray-100 dark:hover:bg-surface-800'
            )}
          >
            <tab.icon className="w-4 h-4" />
            {tab.label}
            {tab.count !== null && tab.count > 0 && (
              <Badge variant="primary" size="sm">
                {tab.count}
              </Badge>
            )}
          </button>
        ))}
      </div>

      <div className="flex-1 overflow-auto p-4 scrollbar-thin">
        {activeTab === 'summary' && (
          <div className="space-y-4">
            <div>
              <div className="flex items-center gap-2 mb-2">
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  Sentiment:
                </span>
                <span className={clsx('text-sm font-medium capitalize', sentimentColor[analysis.sentiment])}>
                  {analysis.sentiment}
                </span>
              </div>
              <p className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                {analysis.summary}
              </p>
            </div>

            {analysis.keyPoints.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">
                  Key Points
                </h4>
                <ul className="space-y-2">
                  {analysis.keyPoints.map((point) => (
                    <li key={point.id} className="flex items-start gap-2">
                      <ChevronRight className="w-4 h-4 text-primary-500 mt-0.5 shrink-0" />
                      <span className="text-sm text-gray-700 dark:text-gray-300">
                        {point.text}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            )}

            {analysis.topics.length > 0 && (
              <div>
                <h4 className="text-sm font-medium text-gray-900 dark:text-white mb-2">
                  Topics Discussed
                </h4>
                <div className="flex flex-wrap gap-2">
                  {analysis.topics.map((topic) => (
                    <Badge key={topic.id} variant="default">
                      {topic.name}
                    </Badge>
                  ))}
                </div>
              </div>
            )}
          </div>
        )}

        {activeTab === 'actions' && (
          <div className="space-y-3">
            {analysis.actionItems.length === 0 ? (
              <p className="text-sm text-gray-500">No action items identified</p>
            ) : (
              <>
                {analysis.actionItems.map((item) => (
                  <Card key={item.id} padding="sm">
                    <p className="text-sm text-gray-900 dark:text-white mb-2">
                      {item.text}
                    </p>
                    <div className="flex items-center gap-2 text-xs text-gray-500">
                      {item.assignee && <span>Assigned: {item.assignee}</span>}
                      <Badge
                        variant={
                          item.priority === 'high' || item.priority === 'critical'
                            ? 'danger'
                            : item.priority === 'medium'
                            ? 'warning'
                            : 'default'
                        }
                        size="sm"
                      >
                        {item.priority}
                      </Badge>
                    </div>
                  </Card>
                ))}
                <Button
                  variant="secondary"
                  className="w-full"
                  onClick={handleImportTasks}
                  icon={<ListChecks className="w-4 h-4" />}
                >
                  Import to Tasks
                </Button>
              </>
            )}
          </div>
        )}

        {activeTab === 'decisions' && (
          <div className="space-y-3">
            {analysis.decisions.length === 0 ? (
              <p className="text-sm text-gray-500">No decisions identified</p>
            ) : (
              analysis.decisions.map((decision) => (
                <Card key={decision.id} padding="sm">
                  <p className="text-sm text-gray-900 dark:text-white">
                    {decision.text}
                  </p>
                  {decision.context && (
                    <p className="text-xs text-gray-500 mt-1">{decision.context}</p>
                  )}
                </Card>
              ))
            )}
          </div>
        )}

        {activeTab === 'risks' && (
          <div className="space-y-3">
            {analysis.risks.length === 0 ? (
              <p className="text-sm text-gray-500">No risks identified</p>
            ) : (
              analysis.risks.map((risk) => (
                <Card key={risk.id} padding="sm">
                  <div className="flex items-start gap-2">
                    <AlertTriangle
                      className={clsx(
                        'w-4 h-4 shrink-0 mt-0.5',
                        risk.severity === 'high' || risk.severity === 'critical'
                          ? 'text-red-500'
                          : risk.severity === 'medium'
                          ? 'text-yellow-500'
                          : 'text-gray-400'
                      )}
                    />
                    <div>
                      <p className="text-sm text-gray-900 dark:text-white">
                        {risk.text}
                      </p>
                      {risk.mitigation && (
                        <p className="text-xs text-gray-500 mt-1">
                          Mitigation: {risk.mitigation}
                        </p>
                      )}
                    </div>
                  </div>
                </Card>
              ))
            )}
          </div>
        )}
      </div>

      <div className="p-4 border-t border-gray-200 dark:border-surface-700 space-y-2">
        <Button
          variant="primary"
          className="w-full"
          onClick={handleCommitMemory}
          loading={isCommitting}
          disabled={commitSuccess}
          icon={<Upload className="w-4 h-4" />}
        >
          {commitSuccess ? 'Committed!' : 'Commit to Memory'}
        </Button>
        <Button
          variant="secondary"
          className="w-full"
          onClick={handleIngestKnowledge}
          loading={isIngesting}
          disabled={ingestSuccess}
          icon={<Database className="w-4 h-4" />}
        >
          {ingestSuccess ? 'Ingested!' : 'Ingest to Knowledge Base'}
        </Button>
      </div>
    </div>
  );
}

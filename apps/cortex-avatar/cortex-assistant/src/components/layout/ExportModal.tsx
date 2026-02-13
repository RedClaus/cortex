import { useState } from 'react';
import { FileText, FileJson, Download } from 'lucide-react';
import { useUIStore, useMeetingStore, useSettingsStore } from '@/store';
import { Modal, Button, Card } from '@/components/ui';
import {
  exportMeetingAsMarkdown,
  exportMeetingAsText,
  exportMeetingAsJSON,
} from '@/utils/export';
import { applyRedactions } from '@/utils/redaction';

type ExportFormat = 'markdown' | 'text' | 'json';

export function ExportModal() {
  const { exportModalOpen, closeExportModal } = useUIStore();
  const { currentMeeting } = useMeetingStore();
  const { settings } = useSettingsStore();
  const [format, setFormat] = useState<ExportFormat>('markdown');
  const [applyRedaction, setApplyRedaction] = useState(false);

  const handleExport = () => {
    if (!currentMeeting) return;

    let meeting = currentMeeting;

    if (applyRedaction) {
      const redactedSegments = meeting.segments.map((segment) => {
        const { text } = applyRedactions(segment.text, settings.redactionPatterns);
        return { ...segment, text };
      });
      meeting = { ...meeting, segments: redactedSegments };

      if (meeting.analysis) {
        const { text: redactedSummary } = applyRedactions(
          meeting.analysis.summary,
          settings.redactionPatterns
        );
        meeting = {
          ...meeting,
          analysis: { ...meeting.analysis, summary: redactedSummary },
        };
      }
    }

    switch (format) {
      case 'markdown':
        exportMeetingAsMarkdown(meeting);
        break;
      case 'text':
        exportMeetingAsText(meeting);
        break;
      case 'json':
        exportMeetingAsJSON(meeting);
        break;
    }

    closeExportModal();
  };

  const formats = [
    {
      id: 'markdown' as ExportFormat,
      icon: FileText,
      label: 'Markdown',
      description: 'Formatted document with headers and lists',
    },
    {
      id: 'text' as ExportFormat,
      icon: FileText,
      label: 'Plain Text',
      description: 'Simple text format',
    },
    {
      id: 'json' as ExportFormat,
      icon: FileJson,
      label: 'JSON',
      description: 'Raw data for importing',
    },
  ];

  return (
    <Modal
      isOpen={exportModalOpen}
      onClose={closeExportModal}
      title="Export Meeting"
      size="md"
    >
      <div className="space-y-4">
        <div className="grid grid-cols-3 gap-3">
          {formats.map((f) => (
            <Card
              key={f.id}
              padding="sm"
              hover
              onClick={() => setFormat(f.id)}
              className={
                format === f.id
                  ? 'border-primary-500 bg-primary-50 dark:bg-primary-900/20'
                  : ''
              }
            >
              <div className="text-center">
                <f.icon
                  className={`w-6 h-6 mx-auto mb-2 ${
                    format === f.id
                      ? 'text-primary-500'
                      : 'text-gray-400'
                  }`}
                />
                <div className="text-sm font-medium text-gray-900 dark:text-white">
                  {f.label}
                </div>
              </div>
            </Card>
          ))}
        </div>

        <label className="flex items-center gap-2">
          <input
            type="checkbox"
            checked={applyRedaction}
            onChange={(e) => setApplyRedaction(e.target.checked)}
            className="w-4 h-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
          />
          <span className="text-sm text-gray-700 dark:text-gray-300">
            Apply redactions (emails, phone numbers, etc.)
          </span>
        </label>

        <div className="flex justify-end gap-3 pt-4 border-t border-gray-200 dark:border-surface-700">
          <Button variant="secondary" onClick={closeExportModal}>
            Cancel
          </Button>
          <Button
            variant="primary"
            onClick={handleExport}
            icon={<Download className="w-4 h-4" />}
          >
            Export
          </Button>
        </div>
      </div>
    </Modal>
  );
}

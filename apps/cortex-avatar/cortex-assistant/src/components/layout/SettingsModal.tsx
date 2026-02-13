
import { useUIStore, useSettingsStore } from '@/store';
import { Modal, Button, Input, Select } from '@/components/ui';
import { useTheme } from '@/hooks';

export function SettingsModal() {
  const { settingsOpen, closeSettings } = useUIStore();
  const { settings, setCortexUrl, setTranscriptionMode, setAutoSave, setDefaultLanguage } =
    useSettingsStore();
  const { theme, setTheme } = useTheme();

  return (
    <Modal
      isOpen={settingsOpen}
      onClose={closeSettings}
      title="Settings"
      size="lg"
    >
      <div className="space-y-6">
        <div>
          <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-3">
            Appearance
          </h3>
          <Select
            label="Theme"
            value={theme}
            onChange={(value) => setTheme(value as 'light' | 'dark' | 'system')}
            options={[
              { value: 'system', label: 'System' },
              { value: 'light', label: 'Light' },
              { value: 'dark', label: 'Dark' },
            ]}
          />
        </div>

        <div>
          <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-3">
            Cortex Connection
          </h3>
          <Input
            label="Cortex-02 URL"
            value={settings.cortexUrl}
            onChange={(e) => setCortexUrl(e.target.value)}
            placeholder="http://localhost:8080"
          />
        </div>

        <div>
          <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-3">
            Transcription
          </h3>
          <div className="space-y-3">
            <Select
              label="Transcription Mode"
              value={settings.transcriptionMode}
              onChange={(value) =>
                setTranscriptionMode(value as 'web_speech' | 'cortex_stt')
              }
              options={[
                { value: 'web_speech', label: 'Web Speech API (Browser)' },
                { value: 'cortex_stt', label: 'Cortex STT (Whisper)' },
              ]}
            />
            <Select
              label="Language"
              value={settings.defaultLanguage}
              onChange={setDefaultLanguage}
              options={[
                { value: 'en-US', label: 'English (US)' },
                { value: 'en-GB', label: 'English (UK)' },
                { value: 'es-ES', label: 'Spanish' },
                { value: 'fr-FR', label: 'French' },
                { value: 'de-DE', label: 'German' },
                { value: 'ja-JP', label: 'Japanese' },
                { value: 'zh-CN', label: 'Chinese (Simplified)' },
              ]}
            />
          </div>
        </div>

        <div>
          <h3 className="text-sm font-medium text-gray-900 dark:text-white mb-3">
            Auto-Save
          </h3>
          <div className="flex items-center gap-4">
            <label className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={settings.autoSaveEnabled}
                onChange={(e) => setAutoSave(e.target.checked)}
                className="w-4 h-4 rounded border-gray-300 text-primary-600 focus:ring-primary-500"
              />
              <span className="text-sm text-gray-700 dark:text-gray-300">
                Enable auto-save
              </span>
            </label>
            <Select
              value={String(settings.autoSaveInterval)}
              onChange={(value) => setAutoSave(settings.autoSaveEnabled, Number(value))}
              options={[
                { value: '10000', label: '10 seconds' },
                { value: '30000', label: '30 seconds' },
                { value: '60000', label: '1 minute' },
                { value: '300000', label: '5 minutes' },
              ]}
              disabled={!settings.autoSaveEnabled}
            />
          </div>
        </div>

        <div className="flex justify-end gap-3 pt-4 border-t border-gray-200 dark:border-surface-700">
          <Button variant="secondary" onClick={closeSettings}>
            Close
          </Button>
        </div>
      </div>
    </Modal>
  );
}

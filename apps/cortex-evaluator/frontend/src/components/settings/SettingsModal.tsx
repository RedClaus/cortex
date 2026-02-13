import { useState, useEffect } from 'react';
import { useAppStore } from '@/stores';
import { ApiSettings } from '@/stores/types';

interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export default function SettingsModal({ isOpen, onClose }: SettingsModalProps) {
  const { isDarkMode, apiSettings, setApiSettings } = useAppStore();
  const [localSettings, setLocalSettings] = useState<ApiSettings>(apiSettings);
  const [isSaving, setIsSaving] = useState(false);
  const [saveStatus, setSaveStatus] = useState<'idle' | 'success' | 'error'>('idle');

  useEffect(() => {
    setLocalSettings(apiSettings);
  }, [apiSettings, isOpen]);

  if (!isOpen) return null;

  const handleChange = (key: keyof ApiSettings, value: string) => {
    setLocalSettings(prev => ({ ...prev, [key]: value }));
  };

  const handleSave = async () => {
    setIsSaving(true);
    setSaveStatus('idle');

    try {
      // Save to local store
      setApiSettings(localSettings);

      // Also save to backend
      const response = await fetch('/api/settings', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(localSettings),
      });

      if (response.ok) {
        setSaveStatus('success');
        setTimeout(() => {
          onClose();
          setSaveStatus('idle');
        }, 1000);
      } else {
        // Still saved locally even if backend fails
        setSaveStatus('success');
        setTimeout(() => setSaveStatus('idle'), 2000);
      }
    } catch (error) {
      // Local save succeeded, backend might not be available
      setSaveStatus('success');
      setTimeout(() => setSaveStatus('idle'), 2000);
    } finally {
      setIsSaving(false);
    }
  };

  const providers = [
    { key: 'openaiApiKey' as const, label: 'OpenAI API Key', placeholder: 'sk-...' },
    { key: 'anthropicApiKey' as const, label: 'Anthropic API Key', placeholder: 'sk-ant-...' },
    { key: 'geminiApiKey' as const, label: 'Gemini API Key', placeholder: 'AIza...' },
    { key: 'groqApiKey' as const, label: 'Groq API Key', placeholder: 'gsk_...' },
    { key: 'ollamaBaseUrl' as const, label: 'Ollama Base URL', placeholder: 'http://localhost:11434' },
  ];

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center">
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/50 backdrop-blur-sm"
        onClick={onClose}
      />

      {/* Modal */}
      <div className={`relative w-full max-w-lg mx-4 rounded-xl shadow-2xl ${
        isDarkMode ? 'bg-gray-800' : 'bg-white'
      }`}>
        {/* Header */}
        <div className={`flex items-center justify-between px-6 py-4 border-b ${
          isDarkMode ? 'border-gray-700' : 'border-gray-200'
        }`}>
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-gradient-to-br from-blue-500 to-purple-600">
              <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
              </svg>
            </div>
            <h2 className={`text-lg font-semibold ${isDarkMode ? 'text-white' : 'text-gray-900'}`}>
              API Settings
            </h2>
          </div>
          <button
            onClick={onClose}
            className={`p-2 rounded-lg transition-colors ${
              isDarkMode ? 'hover:bg-gray-700 text-gray-400' : 'hover:bg-gray-100 text-gray-500'
            }`}
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Body */}
        <div className="px-6 py-4 space-y-4 max-h-[60vh] overflow-y-auto">
          <p className={`text-sm ${isDarkMode ? 'text-gray-400' : 'text-gray-600'}`}>
            Configure your AI provider API keys. Keys are stored locally in your browser and sent to the backend for processing.
          </p>

          {providers.map(({ key, label, placeholder }) => (
            <div key={key}>
              <label className={`block text-sm font-medium mb-1.5 ${
                isDarkMode ? 'text-gray-300' : 'text-gray-700'
              }`}>
                {label}
              </label>
              <input
                type={key === 'ollamaBaseUrl' ? 'text' : 'password'}
                value={localSettings[key]}
                onChange={(e) => handleChange(key, e.target.value)}
                placeholder={placeholder}
                className={`w-full px-3 py-2 rounded-lg border text-sm transition-colors ${
                  isDarkMode
                    ? 'bg-gray-700 border-gray-600 text-white placeholder-gray-500 focus:border-blue-500'
                    : 'bg-white border-gray-300 text-gray-900 placeholder-gray-400 focus:border-blue-500'
                } focus:outline-none focus:ring-2 focus:ring-blue-500/20`}
              />
              {key !== 'ollamaBaseUrl' && localSettings[key] && (
                <p className="mt-1 text-xs text-green-500 flex items-center gap-1">
                  <svg className="w-3 h-3" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                  </svg>
                  Key configured
                </p>
              )}
            </div>
          ))}
        </div>

        {/* Footer */}
        <div className={`flex items-center justify-between px-6 py-4 border-t ${
          isDarkMode ? 'border-gray-700' : 'border-gray-200'
        }`}>
          {saveStatus === 'success' && (
            <span className="text-sm text-green-500 flex items-center gap-1">
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
              </svg>
              Settings saved!
            </span>
          )}
          {saveStatus === 'error' && (
            <span className="text-sm text-red-500">Failed to save settings</span>
          )}
          {saveStatus === 'idle' && <div />}

          <div className="flex items-center gap-3">
            <button
              onClick={onClose}
              className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
                isDarkMode
                  ? 'text-gray-300 hover:bg-gray-700'
                  : 'text-gray-700 hover:bg-gray-100'
              }`}
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={isSaving}
              className={`px-4 py-2 text-sm font-medium rounded-lg transition-colors ${
                isSaving
                  ? 'bg-blue-400 cursor-not-allowed'
                  : 'bg-blue-600 hover:bg-blue-700'
              } text-white`}
            >
              {isSaving ? 'Saving...' : 'Save Settings'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

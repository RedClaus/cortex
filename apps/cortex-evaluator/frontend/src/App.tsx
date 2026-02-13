import { useState } from 'react'
import { BrowserRouter, Routes, Route, NavLink } from 'react-router-dom'
import { useAppStore } from '@/stores'
import { BrainstormCanvas } from '@/components/brainstorm'
import EvaluationHistory from '@/components/evaluations/EvaluationHistory'
import EvaluationDetailModal from '@/components/evaluations/EvaluationDetailModal'
import AnalyticsDashboard from '@/components/analytics/AnalyticsDashboard'
import WorkspaceSelector from '@/components/workspace/WorkspaceSelector'
import { CREditor } from '@/components/cr-editor'
import { SettingsModal } from '@/components/settings'

function Layout({ children }: { children: React.ReactNode }) {
  const { isDarkMode, toggleTheme, selectedProvider, setSelectedProvider, isSettingsOpen, setSettingsOpen } = useAppStore()

  return (
    <div className={`min-h-screen ${isDarkMode ? 'dark bg-gray-900' : 'bg-gray-50'}`}>
      {/* Header */}
      <header className={`sticky top-0 z-50 border-b ${isDarkMode ? 'bg-gray-800 border-gray-700' : 'bg-white border-gray-200'}`}>
        <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
          <div className="flex items-center justify-between h-16">
            {/* Logo */}
            <div className="flex items-center gap-3">
              <div className="w-8 h-8 bg-gradient-to-br from-blue-500 to-purple-600 rounded-lg flex items-center justify-center">
                <span className="text-white font-bold text-sm">CE</span>
              </div>
              <span className={`font-semibold text-lg ${isDarkMode ? 'text-white' : 'text-gray-900'}`}>
                Cortex Evaluator
              </span>
            </div>

            {/* Navigation */}
            <nav className="flex items-center gap-1">
              <NavLink
                to="/"
                className={({ isActive }) =>
                  `px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                      : `${isDarkMode ? 'text-gray-300 hover:bg-gray-700' : 'text-gray-600 hover:bg-gray-100'}`
                  }`
                }
              >
                Dashboard
              </NavLink>
              <NavLink
                to="/brainstorm"
                className={({ isActive }) =>
                  `px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                      : `${isDarkMode ? 'text-gray-300 hover:bg-gray-700' : 'text-gray-600 hover:bg-gray-100'}`
                  }`
                }
              >
                Brainstorm
              </NavLink>
              <NavLink
                to="/history"
                className={({ isActive }) =>
                  `px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                      : `${isDarkMode ? 'text-gray-300 hover:bg-gray-700' : 'text-gray-600 hover:bg-gray-100'}`
                  }`
                }
              >
                History
              </NavLink>
              <NavLink
                to="/cr-editor"
                className={({ isActive }) =>
                  `px-4 py-2 rounded-lg text-sm font-medium transition-colors ${
                    isActive
                      ? 'bg-blue-100 text-blue-700 dark:bg-blue-900 dark:text-blue-200'
                      : `${isDarkMode ? 'text-gray-300 hover:bg-gray-700' : 'text-gray-600 hover:bg-gray-100'}`
                  }`
                }
              >
                CR Editor
              </NavLink>
            </nav>

            {/* Controls */}
            <div className="flex items-center gap-3">
              {/* Provider Selector */}
              <select
                value={selectedProvider}
                onChange={(e) => setSelectedProvider(e.target.value)}
                className={`px-3 py-1.5 text-sm rounded-lg border ${
                  isDarkMode
                    ? 'bg-gray-700 border-gray-600 text-white'
                    : 'bg-white border-gray-300 text-gray-700'
                }`}
              >
                <option value="openai">OpenAI</option>
                <option value="anthropic">Anthropic</option>
                <option value="gemini">Gemini</option>
                <option value="groq">Groq</option>
                <option value="ollama">Ollama</option>
              </select>

              {/* Settings Button */}
              <button
                onClick={() => setSettingsOpen(true)}
                className={`p-2 rounded-lg transition-colors ${
                  isDarkMode
                    ? 'text-gray-400 hover:bg-gray-700 hover:text-gray-200'
                    : 'text-gray-600 hover:bg-gray-100'
                }`}
                title="API Settings"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.065 2.572c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.572 1.065c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.065-2.572c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
              </button>

              {/* Theme Toggle */}
              <button
                onClick={toggleTheme}
                className={`p-2 rounded-lg transition-colors ${
                  isDarkMode
                    ? 'text-yellow-400 hover:bg-gray-700'
                    : 'text-gray-600 hover:bg-gray-100'
                }`}
                title={isDarkMode ? 'Switch to light mode' : 'Switch to dark mode'}
              >
                {isDarkMode ? (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                    <path fillRule="evenodd" d="M10 2a1 1 0 011 1v1a1 1 0 11-2 0V3a1 1 0 011-1zm4 8a4 4 0 11-8 0 4 4 0 018 0zm-.464 4.95l.707.707a1 1 0 001.414-1.414l-.707-.707a1 1 0 00-1.414 1.414zm2.12-10.607a1 1 0 010 1.414l-.706.707a1 1 0 11-1.414-1.414l.707-.707a1 1 0 011.414 0zM17 11a1 1 0 100-2h-1a1 1 0 100 2h1zm-7 4a1 1 0 011 1v1a1 1 0 11-2 0v-1a1 1 0 011-1zM5.05 6.464A1 1 0 106.465 5.05l-.708-.707a1 1 0 00-1.414 1.414l.707.707zm1.414 8.486l-.707.707a1 1 0 01-1.414-1.414l.707-.707a1 1 0 011.414 1.414zM4 11a1 1 0 100-2H3a1 1 0 000 2h1z" clipRule="evenodd" />
                  </svg>
                ) : (
                  <svg className="w-5 h-5" fill="currentColor" viewBox="0 0 20 20">
                    <path d="M17.293 13.293A8 8 0 016.707 2.707a8.001 8.001 0 1010.586 10.586z" />
                  </svg>
                )}
              </button>
            </div>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main>
        {children}
      </main>

      {/* Settings Modal */}
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setSettingsOpen(false)}
      />
    </div>
  )
}

function DashboardPage() {
  const { isDarkMode } = useAppStore()

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-8">
        <h1 className={`text-2xl font-bold ${isDarkMode ? 'text-white' : 'text-gray-900'}`}>
          Dashboard
        </h1>
        <p className={`mt-1 text-sm ${isDarkMode ? 'text-gray-400' : 'text-gray-600'}`}>
          Overview of your evaluations and analytics
        </p>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 mb-8">
        <div className="lg:col-span-2">
          <WorkspaceSelector />
        </div>
        <div>
          <AnalyticsDashboard />
        </div>
      </div>
    </div>
  )
}

function BrainstormPage() {
  return (
    <div className="h-[calc(100vh-64px)]">
      <BrainstormCanvas />
    </div>
  )
}

function HistoryPage() {
  const { isDarkMode } = useAppStore()
  const [selectedEvaluationId, setSelectedEvaluationId] = useState<string | null>(null)

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-8">
        <h1 className={`text-2xl font-bold ${isDarkMode ? 'text-white' : 'text-gray-900'}`}>
          Evaluation History
        </h1>
        <p className={`mt-1 text-sm ${isDarkMode ? 'text-gray-400' : 'text-gray-600'}`}>
          Browse and search your past evaluations
        </p>
      </div>

      <EvaluationHistory onSelectEvaluation={setSelectedEvaluationId} />

      {selectedEvaluationId && (
        <EvaluationDetailModal
          evaluationId={selectedEvaluationId}
          isOpen={!!selectedEvaluationId}
          onClose={() => setSelectedEvaluationId(null)}
        />
      )}
    </div>
  )
}

function CREditorPage() {
  const { isDarkMode } = useAppStore()

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
      <div className="mb-8">
        <h1 className={`text-2xl font-bold ${isDarkMode ? 'text-white' : 'text-gray-900'}`}>
          Change Request Editor
        </h1>
        <p className={`mt-1 text-sm ${isDarkMode ? 'text-gray-400' : 'text-gray-600'}`}>
          Create and edit change requests with markdown support
        </p>
      </div>

      <CREditor />
    </div>
  )
}

export default function App() {
  return (
    <BrowserRouter>
      <Layout>
        <Routes>
          <Route path="/" element={<DashboardPage />} />
          <Route path="/brainstorm" element={<BrainstormPage />} />
          <Route path="/history" element={<HistoryPage />} />
          <Route path="/cr-editor" element={<CREditorPage />} />
        </Routes>
      </Layout>
    </BrowserRouter>
  )
}

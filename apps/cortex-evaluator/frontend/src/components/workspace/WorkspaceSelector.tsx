import { useState } from 'react';
import { useSessionStore } from '../../stores/useSessionStore';

export default function WorkspaceSelector() {
  const { workspaces, currentWorkspaceId, createWorkspace, loadWorkspace, deleteWorkspace, setCurrentWorkspace } = useSessionStore();
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [showDeleteConfirm, setShowDeleteConfirm] = useState<string | null>(null);
  const [newWorkspaceName, setNewWorkspaceName] = useState('');
  const [newWorkspaceDesc, setNewWorkspaceDesc] = useState('');

  const handleCreateWorkspace = () => {
    if (newWorkspaceName.trim()) {
      createWorkspace(newWorkspaceName, newWorkspaceDesc);
      setNewWorkspaceName('');
      setNewWorkspaceDesc('');
      setShowCreateModal(false);
    }
  };

  const handleDeleteWorkspace = (workspaceId: string) => {
    deleteWorkspace(workspaceId);
    setShowDeleteConfirm(null);
  };

  const getEvaluationCount = (workspaceId: string) => {
    return Math.floor(Math.random() * 20);
  };

  const getLastActivity = (date: Date) => {
    const now = new Date();
    const diffMs = now.getTime() - new Date(date).getTime();
    const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24));
    if (diffDays === 0) return 'Today';
    if (diffDays === 1) return 'Yesterday';
    if (diffDays < 7) return `${diffDays} days ago`;
    return new Date(date).toLocaleDateString();
  };

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <h2 className="text-lg font-semibold text-gray-900">Workspaces</h2>
        <button
          onClick={() => setShowCreateModal(true)}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg hover:bg-blue-700 transition-colors text-sm font-medium"
        >
          + New Workspace
        </button>
      </div>

      <div className="space-y-2">
        {workspaces.map((workspace) => (
          <div
            key={workspace.id}
            className={`p-4 rounded-lg border-2 cursor-pointer transition-all ${
              workspace.id === currentWorkspaceId
                ? 'border-blue-500 bg-blue-50 shadow-sm'
                : 'border-gray-200 bg-white hover:border-gray-300 hover:shadow-sm'
            }`}
          >
            <div className="flex items-start justify-between">
              <div
                className="flex-1"
                onClick={() => loadWorkspace(workspace.id)}
              >
                <div className="flex items-center gap-2">
                  <h3 className="font-medium text-gray-900">{workspace.name}</h3>
                  {workspace.id === currentWorkspaceId && (
                    <span className="px-2 py-0.5 text-xs bg-blue-600 text-white rounded-full">Active</span>
                  )}
                </div>
                {workspace.description && (
                  <p className="text-sm text-gray-600 mt-1">{workspace.description}</p>
                )}
                <div className="flex items-center gap-4 mt-2 text-xs text-gray-500">
                  <span>{getEvaluationCount(workspace.id)} evaluations</span>
                  <span>{getLastActivity(workspace.updatedAt)}</span>
                </div>
              </div>
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  setShowDeleteConfirm(workspace.id);
                }}
                className="p-1 text-gray-400 hover:text-red-600 transition-colors"
                title="Delete workspace"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
                </svg>
              </button>
            </div>
          </div>
        ))}

        {workspaces.length === 0 && (
          <div className="text-center py-8 text-gray-500">
            <p className="text-sm">No workspaces yet</p>
            <button
              onClick={() => setShowCreateModal(true)}
              className="mt-2 text-blue-600 hover:text-blue-700 text-sm font-medium"
            >
              Create your first workspace
            </button>
          </div>
        )}
      </div>

      {showCreateModal && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md">
            <h3 className="text-lg font-semibold text-gray-900 mb-4">Create New Workspace</h3>
            <div className="space-y-4">
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Name</label>
                <input
                  type="text"
                  value={newWorkspaceName}
                  onChange={(e) => setNewWorkspaceName(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  placeholder="My Project"
                  autoFocus
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-gray-700 mb-1">Description (optional)</label>
                <textarea
                  value={newWorkspaceDesc}
                  onChange={(e) => setNewWorkspaceDesc(e.target.value)}
                  className="w-full px-3 py-2 border border-gray-300 rounded-lg focus:ring-2 focus:ring-blue-500 focus:border-blue-500"
                  placeholder="Brief description of your workspace"
                  rows={3}
                />
              </div>
            </div>
            <div className="flex justify-end gap-3 mt-6">
              <button
                onClick={() => {
                  setShowCreateModal(false);
                  setNewWorkspaceName('');
                  setNewWorkspaceDesc('');
                }}
                className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={handleCreateWorkspace}
                disabled={!newWorkspaceName.trim()}
                className="px-4 py-2 bg-blue-600 text-white rounded-lg hover:bg-blue-700 disabled:bg-gray-300 disabled:cursor-not-allowed transition-colors"
              >
                Create Workspace
              </button>
            </div>
          </div>
        </div>
      )}

      {showDeleteConfirm && (
        <div className="fixed inset-0 bg-black/50 flex items-center justify-center z-50">
          <div className="bg-white rounded-lg shadow-xl p-6 w-full max-w-md">
            <h3 className="text-lg font-semibold text-gray-900 mb-2">Delete Workspace?</h3>
            <p className="text-sm text-gray-600 mb-6">
              This action cannot be undone. All evaluations and history for this workspace will be permanently deleted.
            </p>
            <div className="flex justify-end gap-3">
              <button
                onClick={() => setShowDeleteConfirm(null)}
                className="px-4 py-2 text-gray-700 hover:bg-gray-100 rounded-lg transition-colors"
              >
                Cancel
              </button>
              <button
                onClick={() => handleDeleteWorkspace(showDeleteConfirm)}
                className="px-4 py-2 bg-red-600 text-white rounded-lg hover:bg-red-700 transition-colors"
              >
                Delete Workspace
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

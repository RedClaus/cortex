import { describe, it, expect, beforeEach, vi } from 'vitest';
import { render, screen } from '@testing-library/react';
import WorkspaceSelector from '../components/workspace/WorkspaceSelector';
import { setActivePinia } from 'pinia';

vi.mock('../stores/useSessionStore');

describe('WorkspaceSelector Component', () => {
  beforeEach(async () => {
    const pinia = (await import('pinia')).createPinia();
    setActivePinia(pinia);
    vi.clearAllMocks();
  });

  it('should render workspace list', () => {
    const { useSessionStore } = await import('../stores/useSessionStore');
    useSessionStore = useSessionStore();
    const mockSetWorkspaces = vi.fn();
    (useSessionStore as any).mockReturnValue({
      workspaces: [
        { id: 'ws-1', name: 'Project A', description: 'Test project', createdAt: new Date(), updatedAt: new Date(), settings: { autoSave: true, theme: 'dark', defaultProvider: 'gemini' } },
        { id: 'ws-2', name: 'Project B', description: 'Another project', createdAt: new Date(), updatedAt: new Date(), settings: { autoSave: false, theme: 'light', defaultProvider: 'claude' } }
      ],
      currentWorkspaceId: 'ws-1',
      createWorkspace: mockSetWorkspaces,
      loadWorkspace: mockSetWorkspaces,
      deleteWorkspace: mockSetWorkspaces,
      setCurrentWorkspace: mockSetWorkspaces
    });

    render(<WorkspaceSelector />);

    expect(screen.getByText('Workspaces')).toBeInTheDocument();
    expect(screen.getByText('Project A')).toBeInTheDocument();
    expect(screen.getByText('Project B')).toBeInTheDocument();
  });

  it('should show "New Workspace" button', () => {
    const { useSessionStore } = await import('../stores/useSessionStore');
    useSessionStore = useSessionStore();
    const mockSetWorkspaces = vi.fn();
    (useSessionStore as any).mockReturnValue({
      workspaces: [],
      currentWorkspaceId: null,
      createWorkspace: mockSetWorkspaces,
      loadWorkspace: mockSetWorkspaces,
      deleteWorkspace: mockSetWorkspaces,
      setCurrentWorkspace: mockSetWorkspaces
    });

    render(<WorkspaceSelector />);

    expect(screen.getByText('+ New Workspace')).toBeInTheDocument();
  });

  it('should show "No workspaces yet" message when empty', () => {
    const { useSessionStore } = await import('../stores/useSessionStore');
    useSessionStore = useSessionStore();
    const mockSetWorkspaces = vi.fn();
    (useSessionStore as any).mockReturnValue({
      workspaces: [],
      currentWorkspaceId: null,
      createWorkspace: mockSetWorkspaces,
      loadWorkspace: mockSetWorkspaces,
      deleteWorkspace: mockSetWorkspaces,
      setCurrentWorkspace: mockSetWorkspaces
    });

    render(<WorkspaceSelector />);

    expect(screen.getByText('No workspaces yet')).toBeInTheDocument();
    expect(screen.getByText('Create your first workspace')).toBeInTheDocument();
  });

  it('should show evaluation count', () => {
    const { useSessionStore } = await import('../stores/useSessionStore');
    useSessionStore = useSessionStore();
    const mockSetWorkspaces = vi.fn();
    (useSessionStore as any).mockReturnValue({
      workspaces: [
        { id: 'ws-1', name: 'Test', description: '', createdAt: new Date(), updatedAt: new Date(), settings: { autoSave: true, theme: 'light', defaultProvider: 'gemini' } }
      ],
      currentWorkspaceId: 'ws-1',
      createWorkspace: mockSetWorkspaces,
      loadWorkspace: mockSetWorkspaces,
      deleteWorkspace: mockSetWorkspaces,
      setCurrentWorkspace: mockSetWorkspaces
    });

    render(<WorkspaceSelector />);

    const evalCount = screen.getByText(/evaluations/i);
    expect(evalCount).toBeInTheDocument();
  });
});

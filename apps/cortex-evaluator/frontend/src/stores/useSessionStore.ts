import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import { SessionStore, Workspace } from './types';

interface SessionSlice {
  workspaces: Workspace[];
  currentWorkspaceId: string | null;
  setCurrentWorkspace: (workspaceId: string | null) => void;
}

interface WorkspaceSlice {
  createWorkspace: (name: string, description: string) => Workspace;
  loadWorkspace: (workspaceId: string) => void;
  deleteWorkspace: (workspaceId: string) => void;
}

const createSessionSlice: (
  set: (partial: Partial<SessionStore>) => void
) => SessionSlice = (set) => ({
  workspaces: [],
  currentWorkspaceId: null,
  setCurrentWorkspace: (workspaceId) => set({ currentWorkspaceId: workspaceId }),
});

const createWorkspaceSlice: (
  set: (partial: Partial<SessionStore> | ((state: SessionStore) => Partial<SessionStore>)) => void,
  get: () => SessionStore
) => WorkspaceSlice = (set, get) => ({
  createWorkspace: (name, description) => {
    const workspace: Workspace = {
      id: crypto.randomUUID(),
      name,
      description,
      createdAt: new Date(),
      updatedAt: new Date(),
      settings: {
        autoSave: true,
        theme: 'light',
        defaultProvider: 'openai',
      },
    };
    
    set((state) => ({
      workspaces: [...state.workspaces, workspace],
      currentWorkspaceId: workspace.id,
    }));
    
    return workspace;
  },
  
  loadWorkspace: (workspaceId) => {
    set({ currentWorkspaceId: workspaceId });
  },
  
  deleteWorkspace: (workspaceId) => {
    set((state) => ({
      workspaces: state.workspaces.filter((w) => w.id !== workspaceId),
      currentWorkspaceId:
        state.currentWorkspaceId === workspaceId
          ? state.workspaces.find((w) => w.id !== workspaceId)?.id || null
          : state.currentWorkspaceId,
    }));
  },
});

export const useSessionStore = create<SessionStore>()(
  persist(
    (set, get) => ({
      ...createSessionSlice(set),
      ...createWorkspaceSlice(set, get),
    }),
    {
      name: 'cortex-evaluator-sessions',
      partialize: (state) => ({
        workspaces: state.workspaces,
        currentWorkspaceId: state.currentWorkspaceId,
      }),
    }
  )
);

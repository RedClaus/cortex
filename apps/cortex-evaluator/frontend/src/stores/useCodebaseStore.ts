import { create } from 'zustand';
import { CodebaseStore, CodeFile, SystemDocumentation } from './types';

interface CodebaseSlice {
  files: CodeFile[];
  setFiles: (files: CodeFile[]) => void;
  addFile: (file: CodeFile) => void;
  removeFile: (fileId: string) => void;
  scanDirectory: (directoryHandle: FileSystemDirectoryHandle) => Promise<CodeFile[]>;
  fetchGitHubRepo: (url: string) => Promise<CodeFile[]>;
}

interface DocumentationSlice {
  systemDoc: SystemDocumentation | null;
  setSystemDoc: (doc: SystemDocumentation | null) => void;
}

const createCodebaseSlice: (
  set: (partial: Partial<CodebaseStore> | ((state: CodebaseStore) => Partial<CodebaseStore>)) => void
) => CodebaseSlice = (set) => ({
  files: [],
  setFiles: (files) => set({ files }),
  addFile: (file) => set((state) => ({ files: [...state.files, file] })),
  removeFile: (fileId) =>
    set((state) => ({
      files: state.files.filter((f) => f.id !== fileId),
    })),
  scanDirectory: async (directoryHandle: FileSystemDirectoryHandle) => {
    const files: CodeFile[] = [];
    
    async function* walkDirectory(handle: FileSystemDirectoryHandle, path = '') {
      for await (const entry of handle.values()) {
        if (entry.kind === 'file') {
          const fileHandle = entry as FileSystemFileHandle;
          const file = await fileHandle.getFile();
          
          if (!file.name.startsWith('.') && file.size < 1024 * 1024) {
            const text = await file.text();
            const extension = file.name.split('.').pop() || '';
            
            files.push({
              id: `${path}/${file.name}`,
              name: file.name,
              path: `${path}/${file.name}`,
              content: text,
              language: extension,
              size: file.size,
              lastModified: new Date(file.lastModified),
            });
          }
        } else if (entry.kind === 'directory') {
          yield* walkDirectory(entry as FileSystemDirectoryHandle, `${path}/${entry.name}`);
        }
      }
    }
    
    await walkDirectory(directoryHandle);
    set({ files });
    return files;
  },
  fetchGitHubRepo: async (url: string) => {
    const response = await fetch(`/api/codebase/github?url=${encodeURIComponent(url)}`);
    if (!response.ok) {
      throw new Error('Failed to fetch GitHub repository');
    }
    const files: CodeFile[] = await response.json();
    set({ files });
    return files;
  },
});

const createDocumentationSlice: (
  set: (partial: Partial<CodebaseStore>) => void
) => DocumentationSlice = (set) => ({
  systemDoc: null,
  setSystemDoc: (doc) => set({ systemDoc: doc }),
});

export const useCodebaseStore = create<CodebaseStore>((set) => ({
  ...createCodebaseSlice(set),
  ...createDocumentationSlice(set),
}));

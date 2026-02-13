import { describe, it, expect, beforeEach, vi } from 'vitest';
import { setActivePinia } from 'pinia';

describe('Codebase Store', () => {
  let useCodebaseStore: any;
  let pinia: any;

  beforeEach(async () => {
    pinia = (await import('pinia')).createPinia();
    setActivePinia(pinia);
    vi.useFakeTimers();
  });

  it('should initialize with empty files and null systemDoc', () => {
    const { useCodebaseStore } = await import('../stores/useCodebaseStore');
    useCodebaseStore = useCodebaseStore;
    const { files, systemDoc } = useCodebaseStore();

    expect(files).toEqual([]);
    expect(systemDoc).toBe(null);
  });

  it('should set files', () => {
    const { useCodebaseStore } = await import('../stores/useCodebaseStore');
    useCodebaseStore = useCodebaseStore;
    const { setFiles, files } = useCodebaseStore();

    const newFiles = [
      {
        id: 'file-1',
        name: 'app.py',
        path: 'src/app.py',
        content: 'from fastapi import FastAPI',
        language: 'python',
        size: 1234,
        lastModified: new Date()
      }
    ];

    setFiles(newFiles);

    expect(files).toEqual(newFiles);
  });

  it('should add file to existing files', () => {
    const { useCodebaseStore } = await import('../stores/useCodebaseStore');
    useCodebaseStore = useCodebaseStore;
    const { addFile, files } = useCodebaseStore();

    const initialCount = files.length;

    addFile({
      id: 'new-file',
      name: 'new.py',
      path: 'src/new.py',
      content: 'print("hello")',
      language: 'python',
      size: 100,
      lastModified: new Date()
    });

    expect(files.length).toBe(initialCount + 1);
    expect(files[files.length - 1].id).toBe('new-file');
  });

  it('should remove file by id', () => {
    const { useCodebaseStore } = await import('../stores/useCodebaseStore');
    useCodebaseStore = useCodebaseStore;
    const { setFiles, addFile, removeFile, files } = useCodebaseStore();

    addFile({ id: 'file-1', name: 'test.py', path: 'test.py', content: '', language: 'python', size: 100, lastModified: new Date() });
    addFile({ id: 'file-2', name: 'test2.py', path: 'test2.py', content: '', language: 'python', size: 100, lastModified: new Date() });

    expect(files.length).toBe(2);

    removeFile('file-1');

    expect(files.length).toBe(1);
    expect(files.find((f: any) => f.id === 'file-1')).toBeUndefined();
  });

  it('should set system documentation', () => {
    const { useCodebaseStore } = await import('../stores/useCodebaseStore');
    useCodebaseStore = useCodebaseStore;
    const { setSystemDoc, systemDoc } = useCodebaseStore();

    const doc = {
      id: 'doc-1',
      title: 'Test Documentation',
      content: 'This is the system documentation',
      sections: [
        { id: 's1', title: 'Overview', content: 'Overview content', order: 1 },
        { id: 's2', title: 'API', content: 'API documentation', order: 2 }
      ],
      lastUpdated: new Date()
    };

    setSystemDoc(doc);

    expect(systemDoc).toEqual(doc);
  });

  it('should set systemDoc to null', () => {
    const { useCodebaseStore } = await import('../stores/useCodebaseStore');
    useCodebaseStore = useCodebaseStore;
    const { setSystemDoc, systemDoc } = useCodebaseStore();

    setSystemDoc(null);

    expect(systemDoc).toBe(null);
  });
});

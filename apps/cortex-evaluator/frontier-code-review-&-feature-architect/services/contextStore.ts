
import { CodeFile } from "../types";

const IGNORE_DIRS = new Set(['node_modules', '.git', 'dist', 'build', '.next', '.vscode', 'venv', '__pycache__']);
const ALLOWED_EXTENSIONS = new Set(['js', 'ts', 'tsx', 'jsx', 'py', 'go', 'java', 'c', 'cpp', 'h', 'rs', 'php', 'rb', 'md', 'txt', 'json', 'yml', 'yaml', 'css', 'html']);

export async function scanDirectory(
  dirHandle: FileSystemDirectoryHandle,
  onProgress: (current: string, count: number) => void,
  path = ''
): Promise<CodeFile[]> {
  const files: CodeFile[] = [];
  let count = 0;

  async function recursiveScan(handle: FileSystemDirectoryHandle, currentPath: string) {
    // handle.values() returns an AsyncIterable of FileSystemHandle
    // @ts-ignore
    for await (const entry of handle.values()) {
      if (entry.kind === 'directory') {
        if (!IGNORE_DIRS.has(entry.name)) {
          // Fix: cast FileSystemHandle to FileSystemDirectoryHandle to satisfy recursiveScan parameter type
          await recursiveScan(entry as FileSystemDirectoryHandle, `${currentPath}${entry.name}/`);
        }
      } else if (entry.kind === 'file') {
        const ext = entry.name.split('.').pop()?.toLowerCase() || '';
        if (ALLOWED_EXTENSIONS.has(ext)) {
          // Fix: cast FileSystemHandle to FileSystemFileHandle to access the getFile method
          const fileHandle = entry as FileSystemFileHandle;
          const file = await fileHandle.getFile();
          const content = await file.text();
          count++;
          onProgress(entry.name, count);
          files.push({
            name: entry.name,
            path: `${currentPath}${entry.name}`,
            content: content,
            type: ext
          });
        }
      }
    }
  }

  await recursiveScan(dirHandle, path);
  return files;
}

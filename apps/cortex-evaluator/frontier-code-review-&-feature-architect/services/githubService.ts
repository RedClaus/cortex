
import { CodeFile } from "../types";

const GITHUB_API_BASE = 'https://api.github.com';
const ALLOWED_EXTENSIONS = new Set(['js', 'ts', 'tsx', 'jsx', 'py', 'go', 'java', 'c', 'cpp', 'h', 'rs', 'php', 'rb', 'md', 'txt', 'json', 'yml', 'yaml', 'css', 'html']);

/**
 * Fetches the entire structure and content of a GitHub repository.
 * Note: This uses public APIs and raw content links. 
 * For very large repos or high volume, a GitHub token might be required (not implemented here for simplicity).
 */
export async function fetchGitHubRepo(
  url: string,
  onProgress: (current: string, count: number) => void
): Promise<CodeFile[]> {
  // Parse URL: https://github.com/owner/repo
  const regex = /github\.com\/([^/]+)\/([^/]+)/;
  const match = url.match(regex);
  if (!match) throw new Error("Invalid GitHub URL. Expected format: https://github.com/owner/repo");

  const owner = match[1];
  let repo = match[2].replace(/\.git$/, '');

  // 1. Get default branch
  const repoInfoResponse = await fetch(`${GITHUB_API_BASE}/repos/${owner}/${repo}`);
  if (!repoInfoResponse.ok) {
    if (repoInfoResponse.status === 404) throw new Error("Repository not found or is private.");
    throw new Error(`GitHub API error: ${repoInfoResponse.statusText}`);
  }
  const repoInfo = await repoInfoResponse.json();
  const branch = repoInfo.default_branch || 'main';

  // 2. Get recursive tree
  const treeResponse = await fetch(`${GITHUB_API_BASE}/repos/${owner}/${repo}/git/trees/${branch}?recursive=1`);
  if (!treeResponse.ok) throw new Error("Could not fetch repository structure.");
  const treeData = await treeResponse.json();

  if (!treeData.tree || !Array.isArray(treeData.tree)) {
    throw new Error("Repository tree is empty or inaccessible.");
  }

  const files: CodeFile[] = [];
  let count = 0;

  // 3. Process files
  for (const item of treeData.tree) {
    if (item.type === 'blob') {
      const ext = item.path.split('.').pop()?.toLowerCase() || '';
      if (ALLOWED_EXTENSIONS.has(ext)) {
        // We use the raw usercontent link for easier fetching without base64 decoding overhead for large blobs
        const rawUrl = `https://raw.githubusercontent.com/${owner}/${repo}/${branch}/${item.path}`;
        
        try {
          const contentRes = await fetch(rawUrl);
          if (contentRes.ok) {
            const content = await contentRes.text();
            count++;
            onProgress(item.path, count);
            files.push({
              name: item.path.split('/').pop() || item.path,
              path: item.path,
              content: content,
              type: ext
            });
          }
        } catch (e) {
          console.warn(`Failed to fetch content for ${item.path}`, e);
        }
      }
    }
  }

  return files;
}

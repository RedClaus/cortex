// Package brainstorm provides GitHub repository fetching for external analysis.
package brainstorm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

// GitHubRepo represents fetched GitHub repository information.
type GitHubRepo struct {
	URL         string `json:"url"`
	Owner       string `json:"owner"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Language    string `json:"language"`
	Stars       int    `json:"stars"`
	Forks       int    `json:"forks"`
	README      string `json:"readme"`
	Topics      []string `json:"topics"`
	License     string `json:"license"`
	UpdatedAt   string `json:"updated_at"`
}

// GitHubFetcher handles fetching repository information from GitHub.
type GitHubFetcher struct {
	client *http.Client
}

// NewGitHubFetcher creates a new GitHub fetcher.
func NewGitHubFetcher() *GitHubFetcher {
	return &GitHubFetcher{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// githubURLPattern matches GitHub repository URLs.
var githubURLPattern = regexp.MustCompile(`(?:https?://)?(?:www\.)?github\.com/([^/\s]+)/([^/\s.]+)(?:\.git)?`)

// ExtractGitHubURLs finds all GitHub repository URLs in text.
func ExtractGitHubURLs(text string) []string {
	matches := githubURLPattern.FindAllString(text, -1)
	// Deduplicate
	seen := make(map[string]bool)
	var result []string
	for _, m := range matches {
		// Normalize URL
		m = strings.TrimSuffix(m, ".git")
		if !strings.HasPrefix(m, "http") {
			m = "https://" + m
		}
		if !seen[m] {
			seen[m] = true
			result = append(result, m)
		}
	}
	return result
}

// ParseGitHubURL extracts owner and repo name from a GitHub URL.
func ParseGitHubURL(url string) (owner, repo string, ok bool) {
	matches := githubURLPattern.FindStringSubmatch(url)
	if len(matches) < 3 {
		return "", "", false
	}
	return matches[1], strings.TrimSuffix(matches[2], ".git"), true
}

// FetchRepo fetches repository information from GitHub.
func (f *GitHubFetcher) FetchRepo(url string) (*GitHubRepo, error) {
	owner, repoName, ok := ParseGitHubURL(url)
	if !ok {
		return nil, fmt.Errorf("invalid GitHub URL: %s", url)
	}

	repo := &GitHubRepo{
		URL:   url,
		Owner: owner,
		Name:  repoName,
	}

	// Fetch repo metadata from GitHub API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, repoName)
	if err := f.fetchRepoMetadata(apiURL, repo); err != nil {
		// Non-fatal: continue without metadata
		repo.Description = "(Could not fetch repo metadata)"
	}

	// Fetch README
	readmeURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/main/README.md", owner, repoName)
	readme, err := f.fetchURL(readmeURL)
	if err != nil {
		// Try master branch
		readmeURL = fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/master/README.md", owner, repoName)
		readme, err = f.fetchURL(readmeURL)
	}
	if err == nil {
		// Truncate if too long
		if len(readme) > 10000 {
			readme = readme[:10000] + "\n\n[README truncated...]"
		}
		repo.README = readme
	} else {
		repo.README = "(Could not fetch README)"
	}

	return repo, nil
}

// fetchRepoMetadata fetches repository metadata from GitHub API.
func (f *GitHubFetcher) fetchRepoMetadata(apiURL string, repo *GitHubRepo) error {
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github.v3+json")
	req.Header.Set("User-Agent", "Cortex-Evaluator")

	resp, err := f.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	var data struct {
		Description string   `json:"description"`
		Language    string   `json:"language"`
		Stars       int      `json:"stargazers_count"`
		Forks       int      `json:"forks_count"`
		Topics      []string `json:"topics"`
		License     *struct {
			Name string `json:"name"`
		} `json:"license"`
		UpdatedAt string `json:"updated_at"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return err
	}

	repo.Description = data.Description
	repo.Language = data.Language
	repo.Stars = data.Stars
	repo.Forks = data.Forks
	repo.Topics = data.Topics
	repo.UpdatedAt = data.UpdatedAt
	if data.License != nil {
		repo.License = data.License.Name
	}

	return nil
}

// fetchURL fetches content from a URL.
func (f *GitHubFetcher) fetchURL(url string) (string, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Cortex-Evaluator")

	resp, err := f.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(body), nil
}

// FormatRepoContext formats a fetched repo for inclusion in LLM context.
func (repo *GitHubRepo) FormatContext() string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("## External Repository: %s/%s\n\n", repo.Owner, repo.Name))
	sb.WriteString(fmt.Sprintf("**URL:** %s\n", repo.URL))

	if repo.Description != "" {
		sb.WriteString(fmt.Sprintf("**Description:** %s\n", repo.Description))
	}
	if repo.Language != "" {
		sb.WriteString(fmt.Sprintf("**Primary Language:** %s\n", repo.Language))
	}
	if repo.Stars > 0 || repo.Forks > 0 {
		sb.WriteString(fmt.Sprintf("**Stars:** %d | **Forks:** %d\n", repo.Stars, repo.Forks))
	}
	if len(repo.Topics) > 0 {
		sb.WriteString(fmt.Sprintf("**Topics:** %s\n", strings.Join(repo.Topics, ", ")))
	}
	if repo.License != "" {
		sb.WriteString(fmt.Sprintf("**License:** %s\n", repo.License))
	}
	if repo.UpdatedAt != "" {
		sb.WriteString(fmt.Sprintf("**Last Updated:** %s\n", repo.UpdatedAt))
	}

	if repo.README != "" && repo.README != "(Could not fetch README)" {
		sb.WriteString("\n### README\n\n")
		sb.WriteString(repo.README)
	}

	return sb.String()
}

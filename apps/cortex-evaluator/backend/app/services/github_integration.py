"""
GitHub Integration Service - Create and manage GitHub issues
"""
import logging
import re
from typing import Optional

import httpx
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type


logger = logging.getLogger(__name__)


class GitHubIntegrationError(Exception):
    """Base exception for GitHub integration errors."""
    pass


class GitHubRateLimitError(GitHubIntegrationError):
    """Raised when GitHub rate limit is exceeded."""
    pass


class GitHubIntegration:
    """Service for interacting with GitHub API to create and manage issues.

    Supports:
    - Creating issues with labels, milestones, and assignees
    - Fetching issues with pagination
    - Listing repositories
    - Validating repository access

    Rate limit handling:
    - GitHub API allows 5000 requests/hour for authenticated requests
    - Handles 403 status codes with X-RateLimit-Remaining headers
    """

    def __init__(self, api_token: str, base_url: str = "https://api.github.com"):
        """Initialize GitHub integration service.

        Args:
            api_token: GitHub personal access token
            base_url: GitHub API base URL (default: https://api.github.com)

        Raises:
            ValueError: If api_token is invalid format
        """
        if not api_token or not isinstance(api_token, str):
            raise ValueError("api_token must be a non-empty string")

        if not self._validate_token_format(api_token):
            raise ValueError("api_token must be a valid GitHub token (starts with 'ghp_', 'github_pat_', or 'gho_')")

        self.api_token = api_token
        self.base_url = base_url.rstrip("/")
        self.client = httpx.AsyncClient(
            timeout=30.0,
            headers={
                "Authorization": f"Bearer {api_token}",
                "Accept": "application/vnd.github.v3+json",
                "User-Agent": "Cortex-Evaluator/1.0"
            }
        )

        logger.info("GitHubIntegration initialized")

    @staticmethod
    def _validate_token_format(token: str) -> bool:
        """Validate GitHub token format.

        Args:
            token: Token string to validate

        Returns:
            True if token has valid prefix, False otherwise
        """
        valid_prefixes = ("ghp_", "github_pat_", "gho_", "ghu_")
        return token.startswith(valid_prefixes)

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.HTTPError, httpx.TimeoutException)),
        reraise=True
    )
    async def create_issue(
        self,
        owner: str,
        repo: str,
        title: str,
        body: str,
        labels: Optional[list[str]] = None,
        milestone: Optional[int] = None,
        assignees: Optional[list[str]] = None
    ) -> dict:
        """Create a GitHub issue.

        Args:
            owner: Repository owner (username or organization)
            repo: Repository name
            title: Issue title
            body: Issue body in markdown format
            labels: Optional list of labels to add
            milestone: Optional milestone number
            assignees: Optional list of usernames to assign

        Returns:
            Dictionary with issue data including:
                - html_url: URL to view issue in browser
                - number: Issue number
                - id: Issue ID

        Raises:
            GitHubRateLimitError: If rate limit exceeded
            GitHubIntegrationError: If issue creation fails
            httpx.HTTPError: For network errors
        """
        if not title:
            raise ValueError("title is required")

        if not owner or not repo:
            raise ValueError("owner and repo are required")

        url = f"{self.base_url}/repos/{owner}/{repo}/issues"

        payload = {
            "title": title,
            "body": body or ""
        }

        if labels:
            payload["labels"] = labels

        if milestone is not None:
            payload["milestone"] = milestone

        if assignees:
            payload["assignees"] = assignees

        try:
            response = await self.client.post(url, json=payload)

            if response.status_code == 403:
                remaining = response.headers.get("X-RateLimit-Remaining", "0")
                reset_time = response.headers.get("X-RateLimit-Reset", "unknown")
                raise GitHubRateLimitError(
                    f"GitHub rate limit exceeded. Remaining: {remaining}, "
                    f"Reset at: {reset_time}"
                )

            response.raise_for_status()

            issue_data = response.json()
            result = {
                "html_url": issue_data.get("html_url"),
                "number": issue_data.get("number"),
                "id": issue_data.get("id"),
                "state": issue_data.get("state"),
                "title": issue_data.get("title")
            }

            logger.info(
                f"Created issue #{result['number']} in {owner}/{repo}: "
                f"{result['title']}"
            )

            return result

        except GitHubRateLimitError:
            raise
        except httpx.HTTPStatusError as e:
            error_msg = f"HTTP error creating issue: {e}"
            logger.error(error_msg)
            raise GitHubIntegrationError(error_msg) from e
        except httpx.HTTPError as e:
            logger.error(f"HTTP error creating issue: {e}")
            raise
        except Exception as e:
            logger.error(f"Error creating GitHub issue: {e}")
            raise GitHubIntegrationError(str(e)) from e

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.HTTPError, httpx.TimeoutException)),
        reraise=True
    )
    async def get_issues(
        self,
        owner: str,
        repo: str,
        state: str = "open",
        per_page: int = 30,
        page: int = 1
    ) -> list[dict]:
        """Fetch issues from a repository with pagination support.

        Args:
            owner: Repository owner
            repo: Repository name
            state: Issue state ('open', 'closed', 'all')
            per_page: Number of issues per page (max 100)
            page: Page number (1-based)

        Returns:
            List of issue dictionaries with metadata:
                - number: Issue number
                - id: Issue ID
                - title: Issue title
                - state: Issue state
                - html_url: Issue URL
                - labels: List of label dicts
                - milestone: Milestone dict or None
                - assignee: Assignee dict or None

        Raises:
            GitHubIntegrationError: If fetching issues fails
            httpx.HTTPError: For network errors
        """
        if state not in ("open", "closed", "all"):
            raise ValueError("state must be 'open', 'closed', or 'all'")

        if per_page < 1 or per_page > 100:
            raise ValueError("per_page must be between 1 and 100")

        url = f"{self.base_url}/repos/{owner}/{repo}/issues"

        params = {
            "state": state,
            "per_page": per_page,
            "page": page
        }

        try:
            response = await self.client.get(url, params=params)
            response.raise_for_status()

            issues_data = response.json()
            issues = []

            for issue in issues_data:
                issues.append({
                    "number": issue.get("number"),
                    "id": issue.get("id"),
                    "title": issue.get("title"),
                    "state": issue.get("state"),
                    "html_url": issue.get("html_url"),
                    "created_at": issue.get("created_at"),
                    "updated_at": issue.get("updated_at"),
                    "labels": [
                        {
                            "name": label.get("name"),
                            "color": label.get("color")
                        }
                        for label in issue.get("labels", [])
                    ],
                    "milestone": (
                        {
                            "number": issue.get("milestone", {}).get("number"),
                            "title": issue.get("milestone", {}).get("title")
                        }
                        if issue.get("milestone") else None
                    ),
                    "assignee": (
                        {
                            "login": issue.get("assignee", {}).get("login"),
                            "html_url": issue.get("assignee", {}).get("html_url")
                        }
                        if issue.get("assignee") else None
                    )
                })

            logger.info(
                f"Fetched {len(issues)} issues from {owner}/{repo} "
                f"(page {page}, state: {state})"
            )

            return issues

        except httpx.HTTPStatusError as e:
            error_msg = f"HTTP error fetching issues: {e}"
            logger.error(error_msg)
            raise GitHubIntegrationError(error_msg) from e
        except httpx.HTTPError as e:
            logger.error(f"HTTP error fetching issues: {e}")
            raise
        except Exception as e:
            logger.error(f"Error fetching GitHub issues: {e}")
            raise GitHubIntegrationError(str(e)) from e

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.HTTPError, httpx.TimeoutException)),
        reraise=True
    )
    async def get_repositories(self, user: str) -> list[dict]:
        """List all repositories for a user.

        Args:
            user: GitHub username

        Returns:
            List of repository dictionaries:
                - name: Repository name
                - full_name: Owner/repo
                - description: Repository description
                - language: Primary language
                - private: Whether repo is private
                - html_url: Repository URL
                - stargazers_count: Number of stars

        Raises:
            GitHubIntegrationError: If fetching repositories fails
            httpx.HTTPError: For network errors
        """
        url = f"{self.base_url}/user/repos" if user == "me" else f"{self.base_url}/users/{user}/repos"

        try:
            response = await self.client.get(url)
            response.raise_for_status()

            repos_data = response.json()
            repositories = []

            for repo in repos_data:
                repositories.append({
                    "name": repo.get("name"),
                    "full_name": repo.get("full_name"),
                    "description": repo.get("description"),
                    "language": repo.get("language"),
                    "private": repo.get("private", False),
                    "html_url": repo.get("html_url"),
                    "stargazers_count": repo.get("stargazers_count", 0),
                    "forks_count": repo.get("forks_count", 0),
                    "open_issues_count": repo.get("open_issues_count", 0)
                })

            logger.info(f"Fetched {len(repositories)} repositories for {user}")

            return repositories

        except httpx.HTTPStatusError as e:
            error_msg = f"HTTP error fetching repositories: {e}"
            logger.error(error_msg)
            raise GitHubIntegrationError(error_msg) from e
        except httpx.HTTPError as e:
            logger.error(f"HTTP error fetching repositories: {e}")
            raise
        except Exception as e:
            logger.error(f"Error fetching GitHub repositories: {e}")
            raise GitHubIntegrationError(str(e)) from e

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.HTTPError, httpx.TimeoutException)),
        reraise=True
    )
    async def validate_repo_access(self, owner: str, repo: str) -> bool:
        """Validate that the authenticated user has access to a repository.

        Args:
            owner: Repository owner
            repo: Repository name

        Returns:
            True if repository is accessible (200 status), False otherwise

        Raises:
            httpx.HTTPError: For network errors
        """
        url = f"{self.base_url}/repos/{owner}/{repo}"

        try:
            response = await self.client.get(url)

            if response.status_code == 200:
                logger.info(f"Repository {owner}/{repo} is accessible")
                return True
            elif response.status_code == 404:
                logger.warning(f"Repository {owner}/{repo} not found")
                return False
            elif response.status_code == 403:
                logger.warning(f"No access to repository {owner}/{repo}")
                return False
            else:
                logger.warning(
                    f"Unexpected status code {response.status_code} for {owner}/{repo}"
                )
                return False

        except httpx.HTTPError as e:
            logger.error(f"HTTP error validating repo access: {e}")
            raise

    async def close(self):
        """Close the HTTP client."""
        await self.client.aclose()

    async def __aenter__(self):
        """Async context manager entry."""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit."""
        await self.close()

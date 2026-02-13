"""
Example usage of GitHubIntegration service
"""

import asyncio
import os
from app.services.github_integration import (
    GitHubIntegration,
    GitHubIntegrationError,
    GitHubRateLimitError
)


async def main():
    """Example usage of GitHubIntegration"""

    # Initialize with your GitHub token
    api_token = os.getenv("GITHUB_TOKEN", "ghp_your_token_here")

    async with GitHubIntegration(api_token=api_token) as github:
        try:
            # Validate repository access
            has_access = await github.validate_repo_access("octocat", "Hello-World")
            print(f"Has access to octocat/Hello-World: {has_access}")

            # Create an issue
            issue = await github.create_issue(
                owner="octocat",
                repo="Hello-World",
                title="Test Issue from Cortex Evaluator",
                body="""## Test Issue

This issue was created automatically by the Cortex Evaluator service.

### Details
- Created via GitHubIntegration service
- Testing issue creation with labels

### Checklist
- [x] Create issue
- [ ] Verify issue appears in repo
- [ ] Close issue when done
""",
                labels=["bug", "priority:high", "automation"],
                milestone=1,
                assignees=["octocat"]
            )

            print(f"✓ Created issue #{issue['number']}")
            print(f"  URL: {issue['html_url']}")
            print(f"  State: {issue['state']}")

            # Fetch issues from repository
            issues = await github.get_issues(
                owner="octocat",
                repo="Hello-World",
                state="open",
                per_page=10,
                page=1
            )

            print(f"\n✓ Found {len(issues)} open issues:")
            for issue in issues[:5]:
                labels_str = ", ".join(
                    l['name'] for l in issue['labels']
                )
                print(f"  #{issue['number']} - {issue['title']}")
                if labels_str:
                    print(f"    Labels: {labels_str}")

            # List repositories
            repos = await github.get_repositories("octocat")
            print(f"\n✓ Found {len(repos)} repositories:")
            for repo in repos[:5]:
                print(f"  - {repo['full_name']}: {repo['language'] or 'No language'}")
                if repo['description']:
                    print(f"    {repo['description']}")

        except GitHubRateLimitError as e:
            print(f"❌ Rate limit exceeded: {e}")
        except GitHubIntegrationError as e:
            print(f"❌ GitHub integration error: {e}")


async def example_rate_limit_handling():
    """Example: Handling rate limits gracefully"""

    api_token = os.getenv("GITHUB_TOKEN", "ghp_your_token_here")

    async with GitHubIntegration(api_token=api_token) as github:
        try:
            # This might trigger rate limit if you make many requests
            for i in range(100):
                await github.create_issue(
                    owner="owner",
                    repo="repo",
                    title=f"Test Issue {i}",
                    body="Testing rate limit handling"
                )
                print(f"Created issue {i}")

        except GitHubRateLimitError as e:
            print(f"⚠️  Hit rate limit: {e}")
            print("   Wait for reset time mentioned in error message")
        except GitHubIntegrationError as e:
            print(f"⚠️  Integration error: {e}")


async def example_paginated_issues():
    """Example: Fetching all issues with pagination"""

    api_token = os.getenv("GITHUB_TOKEN", "ghp_your_token_here")

    async with GitHubIntegration(api_token=api_token) as github:
        all_issues = []
        page = 1
        per_page = 30

        while True:
            issues = await github.get_issues(
                owner="owner",
                repo="repo",
                state="open",
                per_page=per_page,
                page=page
            )

            if not issues:
                break

            all_issues.extend(issues)
            print(f"Fetched page {page}: {len(issues)} issues")

            page += 1

            if len(issues) < per_page:
                break

        print(f"\nTotal issues fetched: {len(all_issues)}")


if __name__ == "__main__":
    print("GitHub Integration Service Example")
    print("=" * 50)
    asyncio.run(main())

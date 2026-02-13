"""
Integration API Router
Handles platform-specific integrations (GitHub, Jira, Linear)
"""
import logging
import httpx
from typing import Optional, Dict, List
from fastapi import APIRouter, HTTPException, Depends
from pydantic import BaseModel

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/integrations", tags=["integrations"])


class CreateIssueRequest(BaseModel):
    platform: str
    title: str
    body: str
    metadata: Optional[Dict] = None


class CreateIssueResponse(BaseModel):
    url: str
    id: str
    status: str


@router.post("/issues", response_model=CreateIssueResponse)
async def create_issue(request: CreateIssueRequest):
    """
    Create an issue in the specified platform

    - Supports GitHub, Jira, Linear
    - Returns issue URL and ID
    """
    logger.info(f"Creating {request.platform} issue: {request.title}")

    try:
        if request.platform == 'github':
            return await create_github_issue(request)
        elif request.platform == 'jira':
            return await create_jira_issue(request)
        elif request.platform == 'linear':
            return await create_linear_issue(request)
        else:
            raise HTTPException(status_code=400, detail=f"Unsupported platform: {request.platform}")

    except Exception as e:
        logger.error(f"Error creating {request.platform} issue: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


async def create_github_issue(request: CreateIssueRequest) -> CreateIssueResponse:
    """Create GitHub issue from CR"""
    metadata = request.metadata or {}
    labels = metadata.get('labels', [])
    milestone = metadata.get('milestone')

    headers = {
        "Authorization": f"token {get_github_token()}",
        "Accept": "application/vnd.github.v3+json"
    }

    payload = {"title": request.title, "body": request.body}
    if labels:
        payload["labels"] = labels
    if milestone:
        payload["milestone"] = milestone

    async with httpx.AsyncClient() as client:
        response = await client.post(
            f"https://api.github.com/repos/{get_github_repo()}/issues",
            json=payload,
            headers=headers
        )
        response.raise_for_status()

        issue = response.json()
        return CreateIssueResponse(
            url=issue["html_url"],
            id=str(issue["number"]),
            status="created"
        )


async def create_jira_issue(request: CreateIssueRequest) -> CreateIssueResponse:
    """Create Jira epic/issue from CR"""
    metadata = request.metadata or {}
    priority = metadata.get('priority', 'Medium')
    story_points = metadata.get('story_points', 5)

    headers = {
        "Authorization": f"Bearer {get_jira_token()}",
        "Content-Type": "application/json"
    }

    fields = {
        "project": {"key": get_jira_project()},
        "summary": request.title,
        "description": request.body,
        "issuetype": {"name": "Epic"},
        "priority": {"name": priority}
    }

    async with httpx.AsyncClient() as client:
        response = await client.post(
            f"{get_jira_base_url()}/rest/api/3/issue",
            json={"fields": fields},
            headers=headers
        )
        response.raise_for_status()

        issue = response.json()
        return CreateIssueResponse(
            url=f"{get_jira_base_url()}/browse/{issue['key']}",
            id=issue["key"],
            status="created"
        )


async def create_linear_issue(request: CreateIssueRequest) -> CreateIssueResponse:
    """Create Linear ticket from CR"""
    metadata = request.metadata or {}
    priority = metadata.get('priority', 'Medium')
    project = metadata.get('project')
    cycle = metadata.get('cycle')

    headers = {
        "Authorization": f"Bearer {get_linear_token()}",
        "Content-Type": "application/json"
    }

    query = """
    mutation createIssue($title: String!, $description: String!, $priority: Priority!, $projectId: String, $cycleId: String) {
      issueCreate(input: {title: $title, description: $description, priority: $priority, projectId: $projectId, cycleId: $cycleId}) {
        success
        issue {
          id
          identifier
          url
        }
      }
    }
    """

    variables = {
        "title": request.title,
        "description": request.body,
        "priority": priority,
        "projectId": project,
        "cycleId": cycle
    }

    async with httpx.AsyncClient() as client:
        response = await client.post(
            "https://api.linear.app/graphql",
            json={"query": query, "variables": variables},
            headers=headers
        )
        response.raise_for_status()

        result = response.json()
        issue = result["data"]["issueCreate"]["issue"]

        return CreateIssueResponse(
            url=issue["url"],
            id=issue["identifier"],
            status="created"
        )


def get_github_token() -> str:
    """Get GitHub API token from config"""
    return "ghp_token_placeholder"


def get_github_repo() -> str:
    """Get GitHub repo from config (owner/repo format)"""
    return "owner/repo"


def get_jira_token() -> str:
    """Get Jira API token from config"""
    return "jira_token_placeholder"


def get_jira_project() -> str:
    """Get Jira project key from config"""
    return "PROJECT"


def get_jira_base_url() -> str:
    """Get Jira base URL from config"""
    return "https://your-domain.atlassian.net"


def get_linear_token() -> str:
    """Get Linear API token from config"""
    return "linear_token_placeholder"

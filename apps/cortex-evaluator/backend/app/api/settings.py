"""
Settings API Router
Handles API key configuration endpoints
"""
import logging
import os
from pathlib import Path
from typing import Optional
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/settings", tags=["settings"])


class ApiSettingsRequest(BaseModel):
    """Request schema for API settings"""
    openaiApiKey: Optional[str] = None
    anthropicApiKey: Optional[str] = None
    geminiApiKey: Optional[str] = None
    groqApiKey: Optional[str] = None
    ollamaBaseUrl: Optional[str] = None


class ApiSettingsResponse(BaseModel):
    """Response schema for API settings (masked)"""
    openaiConfigured: bool
    anthropicConfigured: bool
    geminiConfigured: bool
    groqConfigured: bool
    ollamaBaseUrl: str


def mask_key(key: Optional[str]) -> bool:
    """Check if a key is configured (non-empty)"""
    return bool(key and len(key) > 0)


def get_env_file_path() -> Path:
    """Get the path to the .env file"""
    return Path(__file__).parent.parent.parent / ".env"


def read_env_file() -> dict:
    """Read the current .env file"""
    env_path = get_env_file_path()
    env_vars = {}

    if env_path.exists():
        with open(env_path, 'r') as f:
            for line in f:
                line = line.strip()
                if line and not line.startswith('#') and '=' in line:
                    key, value = line.split('=', 1)
                    env_vars[key.strip()] = value.strip()

    return env_vars


def write_env_file(env_vars: dict) -> None:
    """Write the .env file with updated values"""
    env_path = get_env_file_path()

    # Preserve comments and structure
    lines = []
    if env_path.exists():
        with open(env_path, 'r') as f:
            for line in f:
                stripped = line.strip()
                if stripped.startswith('#') or not stripped:
                    lines.append(line.rstrip())
                elif '=' in stripped:
                    key = stripped.split('=', 1)[0].strip()
                    if key in env_vars:
                        lines.append(f"{key}={env_vars[key]}")
                        del env_vars[key]
                    else:
                        lines.append(line.rstrip())

    # Add any new keys
    for key, value in env_vars.items():
        lines.append(f"{key}={value}")

    with open(env_path, 'w') as f:
        f.write('\n'.join(lines) + '\n')


@router.get("/", response_model=ApiSettingsResponse)
async def get_settings():
    """
    Get current API settings status (keys are masked)

    Returns whether each provider is configured, not the actual keys
    """
    logger.info("Fetching API settings status")

    env_vars = read_env_file()

    return ApiSettingsResponse(
        openaiConfigured=mask_key(env_vars.get('OPENAI_API_KEY') or os.getenv('OPENAI_API_KEY')),
        anthropicConfigured=mask_key(env_vars.get('ANTHROPIC_API_KEY') or os.getenv('ANTHROPIC_API_KEY')),
        geminiConfigured=mask_key(env_vars.get('GEMINI_API_KEY') or os.getenv('GEMINI_API_KEY')),
        groqConfigured=mask_key(env_vars.get('GROQ_API_KEY') or os.getenv('GROQ_API_KEY')),
        ollamaBaseUrl=env_vars.get('OLLAMA_BASE_URL') or os.getenv('OLLAMA_BASE_URL', 'http://localhost:11434')
    )


@router.post("/", response_model=ApiSettingsResponse)
async def update_settings(request: ApiSettingsRequest):
    """
    Update API settings

    - Saves API keys to .env file
    - Updates environment variables for current session
    - Returns updated status (masked)
    """
    logger.info("Updating API settings")

    try:
        env_vars = read_env_file()

        # Map frontend keys to env var names
        key_mapping = {
            'openaiApiKey': 'OPENAI_API_KEY',
            'anthropicApiKey': 'ANTHROPIC_API_KEY',
            'geminiApiKey': 'GEMINI_API_KEY',
            'groqApiKey': 'GROQ_API_KEY',
            'ollamaBaseUrl': 'OLLAMA_BASE_URL',
        }

        # Update only provided values
        for frontend_key, env_key in key_mapping.items():
            value = getattr(request, frontend_key, None)
            if value is not None:
                env_vars[env_key] = value
                # Also update os.environ for current session
                os.environ[env_key] = value

        # Write back to .env
        write_env_file(env_vars)

        logger.info("API settings updated successfully")

        return ApiSettingsResponse(
            openaiConfigured=mask_key(env_vars.get('OPENAI_API_KEY')),
            anthropicConfigured=mask_key(env_vars.get('ANTHROPIC_API_KEY')),
            geminiConfigured=mask_key(env_vars.get('GEMINI_API_KEY')),
            groqConfigured=mask_key(env_vars.get('GROQ_API_KEY')),
            ollamaBaseUrl=env_vars.get('OLLAMA_BASE_URL', 'http://localhost:11434')
        )

    except Exception as e:
        logger.error(f"Error updating settings: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/validate")
async def validate_settings(request: ApiSettingsRequest):
    """
    Validate API keys by making test requests

    - Tests each provided API key
    - Returns validation status for each
    """
    logger.info("Validating API settings")

    results = {}

    # For now, just check if keys are non-empty
    # In production, you'd make actual API calls to validate
    if request.openaiApiKey:
        results['openai'] = request.openaiApiKey.startswith('sk-')

    if request.anthropicApiKey:
        results['anthropic'] = request.anthropicApiKey.startswith('sk-ant-')

    if request.geminiApiKey:
        results['gemini'] = request.geminiApiKey.startswith('AIza')

    if request.groqApiKey:
        results['groq'] = request.groqApiKey.startswith('gsk_')

    if request.ollamaBaseUrl:
        # Could make a test request to Ollama
        results['ollama'] = request.ollamaBaseUrl.startswith('http')

    return {
        "valid": all(results.values()) if results else True,
        "results": results
    }

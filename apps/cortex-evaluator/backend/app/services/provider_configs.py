"""
Provider Configuration Loader

Loads API keys and endpoints for all AI providers from environment variables.
"""
import os
from typing import Optional
from dotenv import load_dotenv

# Load .env file so os.getenv() can access the values
load_dotenv()


def load_provider_configs() -> dict[str, dict[str, Optional[str]]]:
    """Load provider configurations from environment.

    Returns:
        Dictionary mapping provider names to their configs:
        {
            "gemini": {"api_key": "...", "model": "..."},
            "claude": {"api_key": "...", "model": "..."},
            "openai": {"api_key": "...", "model": "..."},
            "ollama": {"base_url": "...", "model": "..."},
            "groq": {"api_key": "...", "model": "..."}
        }
    """
    return {
        "gemini": {
            "api_key": os.getenv("GEMINI_API_KEY", ""),
            "model": os.getenv("GEMINI_MODEL", "gemini-2.0-flash"),
        },
        "claude": {
            "api_key": os.getenv("ANTHROPIC_API_KEY", ""),
            "model": os.getenv("ANTHROPIC_MODEL", "claude-3-opus-20240229"),
        },
        "openai": {
            "api_key": os.getenv("OPENAI_API_KEY", ""),
            "model": os.getenv("OPENAI_MODEL", "gpt-4o"),
            "base_url": os.getenv("OPENAI_BASE_URL"),
        },
        "ollama": {
            "base_url": os.getenv("OLLAMA_BASE_URL", "http://localhost:11434"),
            "model": os.getenv("OLLAMA_MODEL", "llama3"),
        },
        "groq": {
            "api_key": os.getenv("GROQ_API_KEY", ""),
            "model": os.getenv("GROQ_MODEL", "llama-3.3-70b-versatile"),
            "base_url": "https://api.groq.com/openai/v1",
        },
    }


def get_provider_config(provider_name: str) -> dict[str, Optional[str]]:
    """Get configuration for a specific provider.

    Args:
        provider_name: Name of the provider (gemini, claude, openai, ollama, groq)

    Returns:
        Provider configuration dictionary

    Raises:
        ValueError: If provider name is unknown
    """
    configs = load_provider_configs()
    if provider_name not in configs:
        raise ValueError(f"Unknown provider: {provider_name}")
    return configs[provider_name]

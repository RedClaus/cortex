"""
Cortex Evaluator - Backend Configuration
"""
from pydantic_settings import BaseSettings, SettingsConfigDict


class Settings(BaseSettings):
    """Application settings with environment variable support"""

    # Database
    DATABASE_URL: str = "sqlite:///./data/cortex.db"

    # AI Provider API Keys
    GEMINI_API_KEY: str = ""
    OPENAI_API_KEY: str = ""
    ANTHROPIC_API_KEY: str = ""
    GROQ_API_KEY: str = ""
    OLLAMA_BASE_URL: str = "http://localhost:11434"

    # Vector Database
    CHROMA_PATH: str = "./data/chroma"
    CHROMA_HOST: str = "localhost"
    CHROMA_PORT: int = 8001

    # Redis
    REDIS_URL: str = "redis://localhost:6379"

    # GitHub
    GITHUB_API_TOKEN: str = ""

    # CORS
    CORS_ORIGINS: list[str] = ["http://localhost:3000", "http://localhost:5173"]

    class Config(SettingsConfigDict):
        env_file = ".env"
        env_file_encoding = "utf-8"


settings = Settings()

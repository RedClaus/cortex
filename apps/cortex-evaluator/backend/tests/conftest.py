"""
Test fixtures for backend tests
"""
import asyncio
from typing import AsyncGenerator, Generator
from unittest.mock import AsyncMock, MagicMock
import pytest
from sqlalchemy.ext.asyncio import create_async_engine, AsyncSession
from sqlalchemy.orm import sessionmaker

from app.models.database import Project, Codebase, Evaluation, EvaluationResult, BrainstormSession
from app.services.ai_router import CortexRouter
from app.services.vector_db import VectorStore
from app.services.circuit_breaker import CircuitBreaker, CircuitBreakerConfig


@pytest.fixture
def event_loop():
    """Create event loop for async tests"""
    loop = asyncio.get_event_loop_policy().new_event_loop()
    yield loop
    loop.close()


@pytest.fixture
async def mock_db_session() -> AsyncGenerator[AsyncSession, None]:
    """Mock database session for testing"""
    # Create in-memory SQLite database
    engine = create_async_engine(
        "sqlite+aiosqlite:///:memory:",
        echo=False
    )
    async_session_maker = sessionmaker(
        engine, class_=AsyncSession, expire_on_commit=False
    )

    async with async_session_maker() as session:
        yield session

    await engine.dispose()


@pytest.fixture
def mock_provider_configs():
    """Mock provider configurations"""
    return {
        "gemini": {
            "api_key": "test_key",
            "model": "gemini-1.5-pro"
        },
        "claude": {
            "api_key": "test_key",
            "model": "claude-3-opus-20240229"
        },
        "openai": {
            "api_key": "test_key",
            "model": "gpt-4o",
            "base_url": None
        },
        "ollama": {
            "base_url": "http://localhost:11434",
            "model": "llama3"
        },
        "groq": {
            "api_key": "test_key",
            "model": "llama3-70b-8192",
            "base_url": "https://api.groq.com/openai/v1"
        }
    }


@pytest.fixture
def mock_circuit_breaker():
    """Create mock circuit breaker for testing"""
    config = CircuitBreakerConfig(
        failure_threshold=3,
        success_threshold=2,
        timeout=5.0
    )
    return CircuitBreaker("test_provider", config)


@pytest.fixture
def sample_codebase():
    """Sample codebase for testing"""
    return {
        "src/app.py": """
from fastapi import FastAPI

app = FastAPI()

@app.get("/")
def read_root():
    return {"message": "Hello World"}
""",
        "src/models.py": """
from pydantic import BaseModel

class User(BaseModel):
    name: str
    email: str
""",
        "README.md": """
# Test Project

This is a test project for Cortex Evaluator.
"""
    }


@pytest.fixture
def sample_input_data():
    """Sample input data for evaluation"""
    return {
        "query": "Analyze the code quality",
        "has_vision": False,
        "metadata": {}
    }


@pytest.fixture
def sample_project():
    """Sample project for database tests"""
    return Project(
        name="Test Project",
        description="A test project for unit tests"
    )


@pytest.fixture
def sample_codebase_obj():
    """Sample codebase object for database tests"""
    return Codebase(
        name="test-codebase",
        type="local",
        source_url=None
    )


@pytest.fixture
def sample_evaluation():
    """Sample evaluation for database tests"""
    return Evaluation(
        input_type="repo",
        input_name="test-repo",
        provider_id="gemini"
    )


@pytest.fixture
def sample_evaluation_result():
    """Sample evaluation result for database tests"""
    return EvaluationResult(
        value_score=85,
        executive_summary="Code is well-structured",
        technical_feasibility="Highly feasible",
        gap_analysis="Minor improvements needed",
        suggested_cr="Add error handling"
    )


@pytest.fixture
def sample_brainstorm_session():
    """Sample brainstorm session for database tests"""
    return BrainstormSession(
        title="Test Brainstorm",
        nodes=[],
        edges=[]
    )


@pytest.fixture
def mock_arxiv_response_xml():
    """Mock arXiv API XML response"""
    return """<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <entry>
    <id>http://arxiv.org/abs/2301.12345</id>
    <title>Test Paper on AI Evaluation</title>
    <author>
      <name>John Doe</name>
    </author>
    <summary>This is a test paper about AI evaluation methods.</summary>
    <published>2023-01-15T00:00:00Z</published>
    <category term="cs.AI" scheme="http://arxiv.org/schemes/2008/subjects/class"/>
    <link href="http://arxiv.org/pdf/2301.12345.pdf" rel="related" type="application/pdf"/>
  </entry>
  <entry>
    <id>http://arxiv.org/abs/2302.54321</id>
    <title>Another Test Paper</title>
    <author>
      <name>Jane Smith</name>
    </author>
    <summary>Another test paper about machine learning.</summary>
    <published>2023-02-20T00:00:00Z</published>
    <category term="cs.LG" scheme="http://arxiv.org/schemes/2008/subjects/class"/>
    <link href="http://arxiv.org/pdf/2302.54321.pdf" rel="related" type="application/pdf"/>
  </entry>
</feed>"""


@pytest.fixture
def mock_pdf_bytes():
    """Mock PDF file bytes"""
    return b"%PDF-1.4\n%test pdf content"

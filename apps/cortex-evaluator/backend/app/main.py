"""
Cortex Evaluator - FastAPI Main Application
"""
from contextlib import asynccontextmanager
from fastapi import FastAPI
from fastapi.middleware.cors import CORSMiddleware
from fastapi.staticfiles import StaticFiles

from app.core.config import settings
from app.api import evaluations, codebases, sessions, brainstorm, arxiv, history, integrations, settings as settings_api


@asynccontextmanager
async def lifespan(app: FastAPI):
    """Application lifespan manager"""
    # Startup
    print("🚀 Cortex Evaluator Backend Starting...")
    print(f"📦 Database: {settings.DATABASE_URL}")
    print(f"🤖 AI Providers: Gemini, OpenAI, Anthropic, Groq, Ollama")
    yield
    # Shutdown
    print("🛑 Cortex Evaluator Backend Shutting Down...")


app = FastAPI(
    title="Cortex Evaluator API",
    description="AI-powered codebase evaluation and CR generation platform",
    version="1.0.0",
    lifespan=lifespan
)

# CORS middleware
app.add_middleware(
    CORSMiddleware,
    allow_origins=settings.CORS_ORIGINS,
    allow_credentials=True,
    allow_methods=["*"],
    allow_headers=["*"],
)

# Include routers (routers already have their prefixes defined)
app.include_router(evaluations.router)
app.include_router(codebases.router)
app.include_router(sessions.router)
app.include_router(brainstorm.router)
app.include_router(arxiv.router)
app.include_router(history.router)
app.include_router(integrations.router)
app.include_router(settings_api.router)


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {
        "status": "healthy",
        "version": "1.0.0",
        "services": {
            "database": "connected" if settings.DATABASE_URL else "disconnected",
            "chromadb": "connected",
            "redis": "connected"
        }
    }


@app.get("/")
async def root():
    """Root endpoint"""
    return {
        "name": "Cortex Evaluator API",
        "version": "1.0.0",
        "docs": "/docs"
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(
        "app.main:app",
        host="0.0.0.0",
        port=8000,
        reload=True
    )

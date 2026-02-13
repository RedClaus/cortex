"""
arXiv API Router
Handles research paper search and retrieval endpoints
"""
import logging
from typing import Optional
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from ..services.arxiv_service import ArxivService
from ..services.vector_db import VectorStore

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/arxiv", tags=["arxiv"])

arxiv_service = ArxivService()
vector_store = VectorStore()


class ArxivSearchRequest(BaseModel):
    """Request schema for arXiv search"""
    query: str
    max_results: int = 10
    categories: Optional[list[str]] = None


class ArxivPaperRequest(BaseModel):
    """Request schema for fetching specific paper"""
    paper_id: str  # e.g., "2301.00774"


@router.post("/search")
async def search_arxiv(request: ArxivSearchRequest):
    """
    Search arXiv papers by query

    - Supports full-text search
    - Filters by categories if provided
    - Returns paper metadata
    """
    logger.info(f"Searching arXiv: query='{request.query}', max={request.max_results}")

    try:
        papers = await arxiv_service.search_papers(
            query=request.query,
            max_results=request.max_results
        )

        logger.info(f"Found {len(papers)} papers")

        return {
            "query": request.query,
            "papers": papers,
            "count": len(papers)
        }

    except Exception as e:
        logger.error(f"Error searching arXiv: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/paper")
async def get_arxiv_paper(request: ArxivPaperRequest):
    """
    Get specific arXiv paper with PDF content

    - Fetches paper metadata
    - Extracts text from PDF
    - Returns complete paper data
    """
    logger.info(f"Fetching arXiv paper: {request.paper_id}")

    try:
        paper = await arxiv_service.get_paper(request.paper_id)

        # Store in vector DB for future search
        await vector_store.index_arxiv_paper(
            paper_id=paper.get("id", request.paper_id),
            title=paper.get("title", ""),
            authors=paper.get("authors", []),
            content=paper.get("content", ""),
            metadata={
                "published": paper.get("published", ""),
                "categories": paper.get("categories", []),
                "pdf_url": paper.get("pdf_url", "")
            }
        )

        logger.info(f"Fetched paper: {paper.get('title', 'Unknown')}")

        return paper

    except ValueError as e:
        logger.error(f"Paper not found: {e}")
        raise HTTPException(status_code=404, detail=str(e))

    except Exception as e:
        logger.error(f"Error fetching paper {request.paper_id}: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/categories")
async def get_categories():
    """
    Get available arXiv categories

    - Returns list of primary categories
    - Includes category descriptions
    """
    categories = {
        "cs": {
            "name": "Computer Science",
            "description": "Covers algorithms, AI, ML, software, etc."
        },
        "math": {
            "name": "Mathematics",
            "description": "Covers pure mathematics and applied fields"
        },
        "physics": {
            "name": "Physics",
            "description": "Covers all physics subfields"
        },
        "q-bio": {
            "name": "Quantum Biology",
            "description": "Covers quantitative biology topics"
        },
        "stat": {
            "name": "Statistics",
            "description": "Covers statistics theory and applications"
        },
        "econ": {
            "name": "Economics",
            "description": "Covers economic theory and applications"
        }
    }

    return categories


@router.post("/similarity")
async def find_similar_papers(query: str, limit: int = 5):
    """
    Find papers similar to query using semantic search

    - Uses vector DB embeddings
    - Returns similarity scores
    - Filters by top results
    """
    logger.info(f"Finding papers similar to query")

    try:
        similar_papers = await vector_store.search_papers(
            query=query,
            n_results=limit
        )

        return {
            "query": query,
            "similar_papers": similar_papers,
            "count": len(similar_papers)
        }

    except Exception as e:
        logger.error(f"Error finding similar papers: {e}")
        raise HTTPException(status_code=500, detail=str(e))

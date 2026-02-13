"""
Evaluations API Router
Handles AI-powered codebase evaluation endpoints
"""
import logging
import uuid
from typing import Optional
from fastapi import APIRouter, HTTPException, BackgroundTasks, Depends
from pydantic import BaseModel
from sqlalchemy.ext.asyncio import AsyncSession
from sqlmodel import select

from ..models.database import (
    Evaluation, EvaluationResult, Project, Codebase, CodebaseFile
)
from ..models.schemas import (
    EvaluationCreate, EvaluationResultCreate
)
from ..services.ai_router import CortexRouter
from ..services.vector_db import VectorStore
from ..core.config import settings

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/evaluations", tags=["evaluations"])

# Initialize services
ai_router = CortexRouter()
vector_store = VectorStore()


class EvaluationRequest(BaseModel):
    """Request schema for evaluation endpoint"""
    codebase_id: str
    input_type: str  # 'pdf', 'repo', 'snippet', 'arxiv', 'url'
    input_content: Optional[str] = None
    file_data: Optional[dict] = None  # For PDFs
    provider_preference: Optional[str] = None  # Optional override
    user_intent: Optional[str] = None  # 'strong', 'local', 'cheap'


class EvaluationResponse(BaseModel):
    """Response schema for evaluation endpoint"""
    id: str
    value_score: int
    executive_summary: str
    technical_feasibility: str
    gap_analysis: str
    suggested_cr: str
    provider_used: str
    similar_evaluations: Optional[list] = None


@router.post("/analyze", response_model=EvaluationResponse)
async def analyze_evaluation(
    request: EvaluationRequest,
    background_tasks: BackgroundTasks
):
    """
    Run evaluation against codebase with AI analysis

    - Analyzes provided input against codebase context
    - Uses AI router with fallback for reliability
    - Stores evaluation in database
    - Indexes in vector DB for similarity search
    - Returns similar evaluations
    """
    logger.info(f"Starting evaluation for codebase {request.codebase_id}")

    # TODO: Add database session dependency
    # session: AsyncSession = Depends(get_session)

    try:
        # 1. Fetch codebase from database
        # For now, return mock data until DB is fully connected
        codebase_files = []
        system_doc = None

        # 2. Prepare input data
        input_data = {
            "type": request.input_type,
            "name": f"input_{uuid.uuid4().hex[:8]}",
            "content": request.input_content or "",
            "fileData": request.file_data
        }

        # 3. Run AI analysis with router
        logger.info("Routing AI analysis request...")
        result = await ai_router.analyze_with_fallback(
            codebase={f['path']: f['content'] for f in codebase_files},
            input_data=input_data,
            system_doc=system_doc,
            user_intent=request.user_intent
        )

        logger.info(f"AI analysis completed with provider: {result.get('provider', 'unknown')}")

        # 4. Extract analysis results
        analysis_result = result.get("result", result)
        provider_used = result.get("provider", request.provider_preference or "unknown")

        # Handle both text and JSON response formats
        if isinstance(analysis_result, str):
            # Parse structured response from text
            analysis_text = analysis_result

            # For now, create default response structure
            value_score = 75
            executive_summary = analysis_text[:500]
            technical_feasibility = "Requires further analysis"
            gap_analysis = "Gap analysis pending"
            suggested_cr = analysis_text
        else:
            # Assume dict format with structured fields
            value_score = analysis_result.get("valueScore", 75)
            executive_summary = analysis_result.get("executiveSummary", "")
            technical_feasibility = analysis_result.get("technicalFeasibility", "")
            gap_analysis = analysis_result.get("gapAnalysis", "")
            suggested_cr = analysis_result.get("suggestedCR", "")

        # 5. Generate evaluation ID
        evaluation_id = str(uuid.uuid4())

        # TODO: Save evaluation to database
        # evaluation = Evaluation(
        #     id=evaluation_id,
        #     project_id=project_id,  # Get from codebase
        #     input_type=request.input_type,
        #     input_name=input_data["name"],
        #     input_content=request.input_content,
        #     provider_id=provider_used
        # )
        # session.add(evaluation)
        # session.commit()

        # evaluation_result = EvaluationResult(
        #     evaluation_id=evaluation_id,
        #     value_score=value_score,
        #     executive_summary=executive_summary,
        #     technical_feasibility=technical_feasibility,
        #     gap_analysis=gap_analysis,
        #     suggested_cr=suggested_cr
        # )
        # session.add(evaluation_result)
        # session.commit()

        # 6. Store in vector DB (background task)
        background_tasks.add_task(
            _store_evaluation_in_vector_db,
            evaluation_id,
            executive_summary,
            suggested_cr,
            {
                "provider": provider_used,
                "value_score": value_score,
                "input_type": request.input_type,
                "codebase_id": request.codebase_id
            }
        )

        # 7. Find similar evaluations
        similar = []
        try:
            similar = await vector_store.search_similar_evaluations(
                query=executive_summary,
                n_results=5
            )
        except Exception as e:
            logger.warning(f"Failed to find similar evaluations: {e}")

        logger.info(f"Evaluation {evaluation_id} completed successfully")

        return EvaluationResponse(
            id=evaluation_id,
            value_score=value_score,
            executive_summary=executive_summary,
            technical_feasibility=technical_feasibility,
            gap_analysis=gap_analysis,
            suggested_cr=suggested_cr,
            provider_used=provider_used,
            similar_evaluations=similar[:3]  # Return top 3
        )

    except Exception as e:
        logger.error(f"Error during evaluation: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


async def _store_evaluation_in_vector_db(
    evaluation_id: str,
    executive_summary: str,
    suggested_cr: str,
    metadata: dict
):
    """Background task to store evaluation in vector DB"""
    try:
        await vector_store.store_evaluation(
            evaluation_id=evaluation_id,
            summary=executive_summary,
            cr=suggested_cr,
            metadata=metadata
        )
    except Exception as e:
        logger.error(f"Failed to store evaluation in vector DB: {e}")


@router.get("/history")
async def get_evaluation_history(
    project_id: Optional[str] = None,
    limit: int = 50,
    offset: int = 0
):
    """
    Get paginated list of evaluations

    - Filter by project_id if provided
    - Supports pagination with limit and offset
    - Returns evaluations with basic info (results are separate)
    """
    logger.info(f"Fetching evaluation history: project={project_id}, limit={limit}, offset={offset}")

    # TODO: Implement database query
    # stmt = select(Evaluation).offset(offset).limit(limit)
    # if project_id:
    #     stmt = stmt.where(Evaluation.project_id == project_id)
    # result = await session.execute(stmt)
    # evaluations = result.scalars().all()

    # Return mock data for now
    return {
        "evaluations": [],
        "total": 0,
        "limit": limit,
        "offset": offset
    }


@router.get("/{evaluation_id}")
async def get_evaluation(evaluation_id: str):
    """
    Get specific evaluation with full results

    - Returns complete evaluation with analysis results
    - Includes CR and metadata
    """
    logger.info(f"Fetching evaluation {evaluation_id}")

    # TODO: Implement database query
    # evaluation = await session.get(Evaluation, evaluation_id)
    # if not evaluation:
    #     raise HTTPException(status_code=404, detail="Evaluation not found")

    # Return mock data for now
    return {
        "id": evaluation_id,
        "project_id": None,
        "input_type": "unknown",
        "input_name": "Mock Input",
        "provider_id": "gemini",
        "created_at": None,
        "result": None
    }


@router.get("/{evaluation_id}/similar")
async def get_similar_evaluations(evaluation_id: str, limit: int = 10):
    """
    Get evaluations similar to given ID

    - Uses vector DB semantic search
    - Returns similar evaluations with similarity scores
    """
    logger.info(f"Finding evaluations similar to {evaluation_id}")

    try:
        # TODO: Get evaluation from DB first
        # evaluation = await session.get(Evaluation, evaluation_id)
        # if not evaluation:
        #     raise HTTPException(status_code=404, detail="Evaluation not found")

        # For now, use a generic query
        similar = await vector_store.search_similar_evaluations(
            query="code analysis",
            n_results=limit
        )

        return {
            "evaluation_id": evaluation_id,
            "similar_evaluations": similar,
            "count": len(similar)
        }

    except Exception as e:
        logger.error(f"Error finding similar evaluations: {e}")
        raise HTTPException(status_code=500, detail=str(e))


@router.get("/stats")
async def get_evaluation_stats(project_id: Optional[str] = None):
    """
    Get evaluation statistics

    - Total evaluations count
    - Average value score
    - Provider usage distribution
    - Evaluation type distribution
    - Filter by project_id if provided
    """
    logger.info(f"Fetching evaluation stats: project={project_id}")

    # TODO: Implement database aggregation queries
    # total_count = select(func.count(Evaluation.id))
    # avg_score = select(func.avg(EvaluationResult.value_score))
    # provider_dist = select(Evaluation.provider_id, func.count(Evaluation.id)).group_by(Evaluation.provider_id)

    return {
        "total_evaluations": 0,
        "avg_value_score": 0.0,
        "provider_usage": {},
        "type_distribution": {},
        "project_id": project_id
    }

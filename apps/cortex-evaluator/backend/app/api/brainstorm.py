"""
Brainstorm API Router
Handles AI-powered brainstorming and ideation endpoints
"""
import logging
from typing import Optional, List
from fastapi import APIRouter, HTTPException
from pydantic import BaseModel

from ..services.ai_router import CortexRouter

logger = logging.getLogger(__name__)

router = APIRouter(prefix="/api/brainstorm", tags=["brainstorm"])

ai_router = CortexRouter()


class BrainstormRequest(BaseModel):
    """Request schema for brainstorming"""
    topic: str
    constraints: Optional[List[str]] = None
    provider_preference: Optional[str] = None


class BrainstormResponse(BaseModel):
    """Response schema for brainstorming"""
    ideas: List[dict]
    provider_used: str
    topic: str


@router.post("/ideas", response_model=BrainstormResponse)
async def generate_ideas(request: BrainstormRequest):
    """
    Generate brainstorming ideas using AI

    - Uses AI router with provider selection
    - Supports optional constraints
    - Returns structured idea list
    """
    logger.info(f"Generating ideas for topic: {request.topic}")

    try:
        # Select provider and generate ideas
        decision = ai_router.route_analysis(
            codebase={},
            input_data={"type": "brainstorm", "topic": request.topic},
            system_doc=None,
            user_intent=None
        )

        provider = decision.provider

        # Generate ideas using selected provider
        ideas = await provider.brainstorm(
            topic=request.topic,
            constraints=request.constraints
        )

        logger.info(f"Generated {len(ideas)} ideas using {provider.name}")

        return BrainstormResponse(
            ideas=ideas,
            provider_used=provider.name,
            topic=request.topic
        )

    except Exception as e:
        logger.error(f"Error generating ideas: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


class ExpandRequest(BaseModel):
    """Request schema for idea expansion"""
    idea: str
    context: Optional[str] = None


class ExpandResponse(BaseModel):
    """Response schema for idea expansion"""
    title: str
    description: str
    considerations: List[str]
    nextSteps: List[str]


@router.post("/expand", response_model=ExpandResponse)
async def expand_idea(request: ExpandRequest):
    """
    Expand on a specific idea using AI

    - Routes to available AI provider
    - Provides detailed analysis of idea
    - Generates implementation suggestions
    - Considers existing context
    """
    logger.info(f"Expanding idea: {request.idea[:50]}...")

    try:
        # Route to an AI provider
        decision = ai_router.route_analysis(
            codebase={},
            input_data={"type": "expand", "idea": request.idea},
            system_doc=None,
            user_intent=None
        )

        provider = decision.provider
        logger.info(f"Using provider: {provider.name} for idea expansion")

        # Call the provider's expand_idea method
        expansion = await provider.expand_idea(
            idea=request.idea,
            context=request.context
        )

        logger.info(f"Expansion complete: {expansion.get('title', 'untitled')}")

        # Ensure the response has all required fields
        return ExpandResponse(
            title=expansion.get("title", request.idea),
            description=expansion.get("description", ""),
            considerations=expansion.get("considerations", []),
            nextSteps=expansion.get("nextSteps", expansion.get("next_steps", []))
        )

    except Exception as e:
        logger.error(f"Error expanding idea: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))


@router.post("/evaluate")
async def evaluate_ideas(
    ideas: List[str],
    criteria: Optional[List[str]] = None
):
    """
    Evaluate and rank brainstorming ideas

    - Scores ideas against criteria
    - Provides recommendation
    - Returns ranked list
    """
    logger.info(f"Evaluating {len(ideas)} ideas")

    try:
        criteria = criteria or ["feasibility", "impact", "effort", "risk"]

        # For now, return mock evaluation
        evaluations = []
        for i, idea in enumerate(ideas):
            evaluations.append({
                "id": i,
                "idea": idea,
                "scores": {
                    "feasibility": 75 + (i % 3) * 5,
                    "impact": 80 - (i % 4) * 10,
                    "effort": 50 + (i % 2) * 20,
                    "risk": 30 + (i % 5) * 10
                },
                "overall_score": 75,
                "rank": i + 1
            })

        # Sort by overall score
        evaluations.sort(key=lambda x: x["overall_score"], reverse=True)

        # Update ranks
        for i, eval in enumerate(evaluations):
            eval["rank"] = i + 1

        return {
            "ideas": evaluations,
            "criteria": criteria,
            "top_idea": evaluations[0] if evaluations else None
        }

    except Exception as e:
        logger.error(f"Error evaluating ideas: {e}")
        raise HTTPException(status_code=500, detail=str(e))


class ConnectRequest(BaseModel):
    """Request schema for connecting ideas"""
    idea_a: str
    idea_b: str
    relationship: str = "related"


class ConnectionAnalysis(BaseModel):
    """Analysis result for connected ideas"""
    synergy: str
    conflicts: str
    complementary_aspects: List[str]
    integration_approach: str
    isValid: bool
    bridgeConcept: Optional[str] = None


class ConnectResponse(BaseModel):
    """Response schema for connect endpoint"""
    ideaA: str
    ideaB: str
    relationship: str
    analysis: ConnectionAnalysis


@router.post("/connect", response_model=ConnectResponse)
async def connect_ideas(request: ConnectRequest):
    """
    Find connections between two ideas using AI

    - Analyzes relationship (related, conflicting, complementary)
    - Validates logical connection
    - Suggests bridge concept to link ideas
    - Provides integration approaches
    """
    logger.info(f"Connecting ideas: {request.idea_a[:30]}... <-> {request.idea_b[:30]}...")

    # Get all providers to try (fast lane + smart lane)
    all_providers = ai_router.fast_lane + ai_router.smart_lane
    last_error = None

    for provider in all_providers:
        try:
            logger.info(f"Trying provider: {provider.name} for connection analysis")

            analysis = await provider.connect_ideas(
                idea_a=request.idea_a,
                idea_b=request.idea_b,
                relationship=request.relationship
            )

            logger.info(f"Connection analysis complete with {provider.name}: valid={analysis.get('isValid', True)}")

            return ConnectResponse(
                ideaA=request.idea_a,
                ideaB=request.idea_b,
                relationship=request.relationship,
                analysis=ConnectionAnalysis(
                    synergy=analysis.get("synergy", ""),
                    conflicts=analysis.get("conflicts", ""),
                    complementary_aspects=analysis.get("complementary_aspects", []),
                    integration_approach=analysis.get("integration_approach", ""),
                    isValid=analysis.get("isValid", True),
                    bridgeConcept=analysis.get("bridgeConcept")
                )
            )

        except Exception as e:
            logger.warning(f"Provider {provider.name} failed: {e}")
            last_error = e
            continue

    # All providers failed
    logger.error(f"All providers failed for connect_ideas: {last_error}", exc_info=True)
    raise HTTPException(status_code=500, detail=f"All AI providers failed: {last_error}")


@router.get("/templates")
async def get_templates():
    """
    Get brainstorming templates

    - Returns predefined session templates
    - Includes node structures and example topics
    """
    templates = {
        "feature_brainstorm": {
            "name": "Feature Brainstorm",
            "description": "Generate and explore new feature ideas",
            "initial_nodes": [
                {"type": "problem", "label": "Problem Statement"},
                {"type": "idea", "label": "Idea 1"},
                {"type": "idea", "label": "Idea 2"},
                {"type": "reference", "label": "Reference"}
            ]
        },
        "bug_triage": {
            "name": "Bug Triage",
            "description": "Organize and prioritize bug reports",
            "initial_nodes": [
                {"type": "problem", "label": "Bug Report"},
                {"type": "idea", "label": "Analysis"},
                {"type": "idea", "label": "Solution 1"},
                {"type": "idea", "label": "Solution 2"}
            ]
        },
        "architecture_review": {
            "name": "Architecture Review",
            "description": "Evaluate and improve system architecture",
            "initial_nodes": [
                {"type": "problem", "label": "Current Issue"},
                {"type": "idea", "label": "Improvement 1"},
                {"type": "idea", "label": "Improvement 2"},
                {"type": "reference", "label": "Documentation"}
            ]
        }
    }

    return templates


class BreakdownRequest(BaseModel):
    """Request schema for CR breakdown"""
    executive_summary: str
    suggested_cr: str
    context: Optional[str] = None


@router.post("/breakdown")
async def breakdown_cr(request: BreakdownRequest):
    """
    Generate detailed CR breakdown from analysis result

    - Breaks down into tasks, estimations, dependencies, risks
    - Returns structured DetailedCR object
    - Uses smart lane for complex breakdown
    """
    logger.info(f"Generating CR breakdown")

    try:
        prompt = f"""
        Given the following analysis, create a detailed Change Request breakdown:

        Executive Summary: {request.executive_summary}
        Suggested CR: {request.suggested_cr}

        Break down into:
        1. Summary (1-2 sentences)
        2. Type (feature/refactor/bugfix/research)
        3. Tasks (5-10 items, each with title, description, acceptance criteria, priority)
        4. Estimation (optimistic, expected, pessimistic in hours/days; complexity 1-13 Fibonacci)
        5. Dependencies (what needs to be done first)
        6. Risk Factors (potential blockers)
        7. Testing Requirements (how to verify)
        8. Documentation Needs (what docs to update)

        Return as structured JSON with this exact structure:
        {{
          "summary": "...",
          "type": "feature|refactor|bugfix|research",
          "tasks": [
            {{
              "id": "uuid",
              "title": "...",
              "description": "...",
              "acceptance_criteria": ["...", "..."],
              "estimate_hours": 4,
              "priority": "critical|high|medium|low",
              "status": "pending"
            }}
          ],
          "estimation": {{
            "optimistic": {{"value": 2, "unit": "days"}},
            "expected": {{"value": 3, "unit": "days"}},
            "pessimistic": {{"value": 5, "unit": "days"}},
            "complexity": 5
          }},
          "dependencies": [
            {{
              "id": "uuid",
              "title": "...",
              "type": "internal|external|blocking",
              "description": "..."
            }}
          ],
          "riskFactors": [
            {{
              "id": "uuid",
              "title": "...",
              "description": "...",
              "severity": "low|medium|high|critical",
              "mitigation": "..."
            }}
          ],
          "testingRequirements": ["...", "..."],
          "documentationNeeds": ["...", "..."],
          "template_id": "claude-code",
          "formatted_output": ""
        }}
        """

        result = await ai_router.route_analysis(
            codebase={},
            input_data={"type": "text", "content": prompt},
            system_doc=None,
            user_intent="smart"
        )

        return result

    except Exception as e:
        logger.error(f"Error generating breakdown: {e}", exc_info=True)
        raise HTTPException(status_code=500, detail=str(e))

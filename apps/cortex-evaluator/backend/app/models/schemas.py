"""
Cortex Evaluator - Pydantic Schemas
"""
from datetime import datetime
from typing import Optional
from uuid import UUID

from pydantic import BaseModel, Field, ConfigDict


class ProjectBase(BaseModel):
    name: str = Field(..., max_length=255)
    description: Optional[str] = Field(None, max_length=10000)


class ProjectCreate(ProjectBase):
    pass


class ProjectUpdate(BaseModel):
    name: Optional[str] = Field(None, max_length=255)
    description: Optional[str] = Field(None, max_length=10000)


class CodebaseBase(BaseModel):
    name: str = Field(..., max_length=255)
    type: str = Field(..., max_length=50)  # 'local', 'github', 'gitlab'
    source_url: Optional[str] = Field(None, max_length=2048)
    metadata: Optional[dict] = None


class CodebaseCreate(CodebaseBase):
    project_id: UUID


class CodebaseUpdate(BaseModel):
    name: Optional[str] = Field(None, max_length=255)
    type: Optional[str] = Field(None, max_length=50)
    source_url: Optional[str] = Field(None, max_length=2048)
    metadata: Optional[dict] = None


class CodebaseFileBase(BaseModel):
    name: str = Field(..., max_length=255)
    path: str = Field(..., max_length=1024)
    content: Optional[str] = Field(None, max_length=10000000)
    file_type: Optional[str] = Field(None, max_length=50)


class CodebaseFileCreate(CodebaseFileBase):
    codebase_id: UUID


class EvaluationBase(BaseModel):
    input_type: str = Field(..., max_length=50)  # 'pdf', 'repo', 'snippet', 'arxiv', 'url'
    input_name: str = Field(..., max_length=255)
    input_content: Optional[str] = Field(None, max_length=5000000)
    provider_id: str = Field(..., max_length=50)  # 'gemini', 'openai', 'anthropic', etc.


class EvaluationCreate(EvaluationBase):
    project_id: UUID


class EvaluationResultBase(BaseModel):
    value_score: int = Field(..., ge=0, le=100)
    executive_summary: str = Field(..., max_length=100000)
    technical_feasibility: Optional[str] = Field(None, max_length=100000)
    gap_analysis: Optional[str] = Field(None, max_length=100000)
    suggested_cr: str = Field(..., max_length=500000)
    metadata: Optional[dict] = None


class EvaluationResultCreate(EvaluationResultBase):
    evaluation_id: UUID


class BrainstormSessionBase(BaseModel):
    title: str = Field(..., max_length=255)
    nodes: dict = Field(..., description="React Flow nodes")
    edges: dict = Field(..., description="React Flow edges")
    viewport: Optional[dict] = Field(None, description="Camera position {x, y, zoom}")


class BrainstormSessionCreate(BrainstormSessionBase):
    project_id: UUID


class BrainstormSessionUpdate(BaseModel):
    title: Optional[str] = Field(None, max_length=255)
    nodes: Optional[dict] = None
    edges: Optional[dict] = None
    viewport: Optional[dict] = None
    updated_at: Optional[datetime] = None


class ChangeRequestBase(BaseModel):
    title: str = Field(..., max_length=255)
    type: str = Field(..., max_length=50)  # 'feature', 'refactor', 'bugfix', 'research'
    summary: Optional[str] = Field(None, max_length=50000)
    tasks: Optional[list] = None
    estimation: Optional[dict] = Field(
        None,
        description="Estimation object with optimistic, expected, pessimistic, complexity"
    )
    dependencies: Optional[list] = None
    risk_factors: Optional[list] = None
    template_id: Optional[str] = Field(None, max_length=50)  # 'claude-code', 'jira', 'github', etc.
    status: str = Field("pending", max_length=50)  # 'pending', 'in-progress', 'completed', 'rejected'


class ChangeRequestCreate(ChangeRequestBase):
    evaluation_id: UUID
    project_id: UUID


class ChangeRequestUpdate(BaseModel):
    title: Optional[str] = Field(None, max_length=255)
    type: Optional[str] = Field(None, max_length=50)
    summary: Optional[str] = Field(None, max_length=50000)
    tasks: Optional[list] = None
    estimation: Optional[dict] = None
    dependencies: Optional[list] = None
    risk_factors: Optional[list] = None
    template_id: Optional[str] = Field(None, max_length=50)
    status: Optional[str] = Field(None, max_length=50)

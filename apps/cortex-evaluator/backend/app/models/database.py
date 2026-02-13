"""
Cortex Evaluator - Database Models
"""
from datetime import datetime
from typing import Optional
from uuid import UUID, uuid4

from sqlmodel import JSON, Column, DateTime, Field, Relationship, SQLModel, Text
from sqlalchemy import func


class Project(SQLModel, table=True):
    __tablename__ = "projects"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    name: str = Field(index=True, max_length=255)
    description: Optional[str] = Field(default=None, sa_column=Column(Text))
    created_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )

    codebases: list["Codebase"] = Relationship(back_populates="project")
    evaluations: list["Evaluation"] = Relationship(back_populates="project")
    brainstorm_sessions: list["BrainstormSession"] = Relationship(back_populates="project")
    change_requests: list["ChangeRequest"] = Relationship(back_populates="project")


class Codebase(SQLModel, table=True):
    __tablename__ = "codebases"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    project_id: UUID = Field(foreign_key="projects.id", nullable=False)
    name: str = Field(index=True, max_length=255)
    type: str = Field(max_length=50)  # 'local', 'github', 'gitlab'
    source_url: Optional[str] = Field(default=None, max_length=2048)
    meta_data: Optional[dict] = Field(default=None, sa_column=Column(JSON))
    created_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )

    project: Optional[Project] = Relationship(back_populates="codebases")
    files: list["CodebaseFile"] = Relationship(back_populates="codebase")


class CodebaseFile(SQLModel, table=True):
    __tablename__ = "codebase_files"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    codebase_id: UUID = Field(foreign_key="codebases.id", nullable=False)
    name: str = Field(max_length=255)
    path: str = Field(max_length=1024)
    content: Optional[str] = Field(default=None, sa_column=Column(Text))
    file_type: Optional[str] = Field(default=None, max_length=50)
    indexed_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )

    codebase: Optional[Codebase] = Relationship(back_populates="files")


class Evaluation(SQLModel, table=True):
    __tablename__ = "evaluations"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    project_id: UUID = Field(foreign_key="projects.id", nullable=False)
    input_type: str = Field(max_length=50)  # 'pdf', 'repo', 'snippet', 'arxiv', 'url'
    input_name: str = Field(max_length=255)
    input_content: Optional[str] = Field(default=None, sa_column=Column(Text))
    file_data: Optional[bytes] = Field(default=None, sa_column=Column(Text))
    provider_id: str = Field(max_length=50)  # 'gemini', 'openai', 'anthropic', etc.
    created_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )

    project: Optional[Project] = Relationship(back_populates="evaluations")
    results: list["EvaluationResult"] = Relationship(back_populates="evaluation")
    change_requests: list["ChangeRequest"] = Relationship(back_populates="evaluation")


class EvaluationResult(SQLModel, table=True):
    __tablename__ = "evaluation_results"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    evaluation_id: UUID = Field(foreign_key="evaluations.id", nullable=False)
    value_score: int = Field(ge=0, le=100)
    executive_summary: str = Field(sa_column=Column(Text))
    technical_feasibility: Optional[str] = Field(default=None, sa_column=Column(Text))
    gap_analysis: Optional[str] = Field(default=None, sa_column=Column(Text))
    suggested_cr: str = Field(sa_column=Column(Text))
    meta_data: Optional[dict] = Field(default=None, sa_column=Column(JSON))
    created_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )

    evaluation: Optional[Evaluation] = Relationship(back_populates="results")


class BrainstormSession(SQLModel, table=True):
    __tablename__ = "brainstorm_sessions"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    project_id: UUID = Field(foreign_key="projects.id", nullable=False)
    title: str = Field(max_length=255)
    nodes: dict = Field(sa_column=Column(JSON))  # React Flow nodes
    edges: dict = Field(sa_column=Column(JSON))  # React Flow edges
    viewport: Optional[dict] = Field(default=None, sa_column=Column(JSON))  # Camera position {x, y, zoom}
    created_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )
    updated_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(
            DateTime(timezone=True),
            server_default=func.now(),
            onupdate=func.now()
        )
    )

    project: Optional[Project] = Relationship(back_populates="brainstorm_sessions")


class ChangeRequest(SQLModel, table=True):
    __tablename__ = "change_requests"

    id: UUID = Field(default_factory=uuid4, primary_key=True)
    evaluation_id: UUID = Field(foreign_key="evaluations.id", nullable=False)
    project_id: UUID = Field(foreign_key="projects.id", nullable=False)
    title: str = Field(max_length=255)
    type: str = Field(max_length=50)  # 'feature', 'refactor', 'bugfix', 'research'
    summary: Optional[str] = Field(default=None, sa_column=Column(Text))
    tasks: Optional[list] = Field(default=None, sa_column=Column(JSON))  # Array of task objects
    estimation: Optional[dict] = Field(default=None, sa_column=Column(JSON))  # {optimistic, expected, pessimistic, complexity}
    dependencies: Optional[list] = Field(default=None, sa_column=Column(JSON))
    risk_factors: Optional[list] = Field(default=None, sa_column=Column(JSON))
    template_id: Optional[str] = Field(default=None, max_length=50)  # 'claude-code', 'jira', 'github', etc.
    status: str = Field(default="pending", max_length=50)  # 'pending', 'in-progress', 'completed', 'rejected'
    created_at: datetime = Field(
        default_factory=datetime.utcnow,
        sa_column=Column(DateTime(timezone=True), server_default=func.now())
    )

    evaluation: Optional[Evaluation] = Relationship(back_populates="change_requests")
    project: Optional[Project] = Relationship(back_populates="change_requests")

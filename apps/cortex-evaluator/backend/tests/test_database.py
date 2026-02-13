"""
Tests for database models - SQLModel creation, relationships, and operations
"""
import pytest
from uuid import uuid4
from datetime import datetime
from app.models.database import (
    Project,
    Codebase,
    CodebaseFile,
    Evaluation,
    EvaluationResult,
    BrainstormSession,
    ChangeRequest
)


class TestProjectModel:
    """Test Project model attributes and relationships"""

    def test_project_creation(self):
        """Should create Project with all fields"""
        project = Project(
            name="Test Project",
            description="A test project"
        )

        assert project.name == "Test Project"
        assert project.description == "A test project"
        assert isinstance(project.id, type(uuid4()))
        assert isinstance(project.created_at, datetime)

    def test_project_relationships(self):
        """Should initialize empty relationship lists"""
        project = Project(name="Test")

        assert project.codebases == []
        assert project.evaluations == []
        assert project.brainstorm_sessions == []
        assert project.change_requests == []


class TestCodebaseModel:
    """Test Codebase model attributes and relationships"""

    def test_codebase_creation(self):
        """Should create Codebase with all fields"""
        project_id = uuid4()
        codebase = Codebase(
            project_id=project_id,
            name="test-codebase",
            type="github",
            source_url="https://github.com/user/repo",
            meta_data={"branch": "main", "commit": "abc123"}
        )

        assert codebase.name == "test-codebase"
        assert codebase.type == "github"
        assert codebase.project_id == project_id
        assert codebase.source_url == "https://github.com/user/repo"
        assert codebase.meta_data == {"branch": "main", "commit": "abc123"}
        assert isinstance(codebase.id, type(uuid4()))

    def test_codebase_relationships(self):
        """Should initialize files relationship"""
        codebase = Codebase(
            project_id=uuid4(),
            name="test",
            type="local"
        )

        assert codebase.files == []

    def test_codebase_types(self):
        """Should accept valid codebase types"""
        project_id = uuid4()
        valid_types = ["local", "github", "gitlab"]

        for codebase_type in valid_types:
            codebase = Codebase(
                project_id=project_id,
                name="test",
                type=codebase_type
            )
            assert codebase.type == codebase_type


class TestCodebaseFileModel:
    """Test CodebaseFile model attributes"""

    def test_codebase_file_creation(self):
        """Should create CodebaseFile with all fields"""
        codebase_id = uuid4()
        file_obj = CodebaseFile(
            codebase_id=codebase_id,
            name="app.py",
            path="src/app.py",
            content="from fastapi import FastAPI",
            file_type="python",
            size=1234
        )

        assert file_obj.name == "app.py"
        assert file_obj.path == "src/app.py"
        assert file_obj.content == "from fastapi import FastAPI"
        assert file_obj.file_type == "python"
        assert file_obj.size == 1234
        assert file_obj.codebase_id == codebase_id
        assert isinstance(file_obj.indexed_at, datetime)

    def test_codebase_file_optional_content(self):
        """Content should be optional (for large files)"""
        file_obj = CodebaseFile(
            codebase_id=uuid4(),
            name="large.py",
            path="src/large.py"
        )

        assert file_obj.content is None


class TestEvaluationModel:
    """Test Evaluation model attributes and relationships"""

    def test_evaluation_creation(self):
        """Should create Evaluation with all fields"""
        project_id = uuid4()
        evaluation = Evaluation(
            project_id=project_id,
            input_type="pdf",
            input_name="research_paper.pdf",
            input_content="Analysis of paper",
            provider_id="gemini"
        )

        assert evaluation.input_type == "pdf"
        assert evaluation.input_name == "research_paper.pdf"
        assert evaluation.provider_id == "gemini"
        assert evaluation.project_id == project_id

    def test_evaluation_input_types(self):
        """Should accept valid input types"""
        project_id = uuid4()
        valid_types = ["pdf", "repo", "snippet", "arxiv", "url"]

        for input_type in valid_types:
            evaluation = Evaluation(
                project_id=project_id,
                input_type=input_type,
                input_name=f"test.{input_type}",
                provider_id="claude"
            )
            assert evaluation.input_type == input_type

    def test_evaluation_relationships(self):
        """Should initialize results and change_requests relationships"""
        evaluation = Evaluation(
            project_id=uuid4(),
            input_type="repo",
            input_name="test-repo",
            provider_id="openai"
        )

        assert evaluation.results == []
        assert evaluation.change_requests == []


class TestEvaluationResultModel:
    """Test EvaluationResult model attributes and constraints"""

    def test_evaluation_result_creation(self):
        """Should create EvaluationResult with all fields"""
        evaluation_id = uuid4()
        result = EvaluationResult(
            evaluation_id=evaluation_id,
            value_score=85,
            executive_summary="Code is well-structured",
            technical_feasibility="Highly feasible",
            gap_analysis="Minor improvements needed",
            suggested_cr="Add error handling",
            meta_data={"model": "gemini-1.5-pro", "tokens": 1500}
        )

        assert result.value_score == 85
        assert result.executive_summary == "Code is well-structured"
        assert result.technical_feasibility == "Highly feasible"
        assert result.gap_analysis == "Minor improvements needed"
        assert result.suggested_cr == "Add error handling"
        assert result.meta_data == {"model": "gemini-1.5-pro", "tokens": 1500}
        assert result.evaluation_id == evaluation_id

    def test_evaluation_result_score_range(self):
        """Value score should be constrained to 0-100"""
        evaluation_id = uuid4()

        # Valid scores
        for score in [0, 50, 100]:
            result = EvaluationResult(
                evaluation_id=evaluation_id,
                value_score=score,
                executive_summary="test"
            )
            assert result.value_score == score

        # Invalid scores should raise validation error
        for invalid_score in [-1, 101, 150]:
            with pytest.raises(ValueError):
                EvaluationResult(
                    evaluation_id=evaluation_id,
                    value_score=invalid_score,
                    executive_summary="test"
                )


class TestBrainstormSessionModel:
    """Test BrainstormSession model attributes"""

    def test_brainstorm_session_creation(self):
        """Should create BrainstormSession with all fields"""
        project_id = uuid4()
        session = BrainstormSession(
            project_id=project_id,
            title="Architecture Planning",
            nodes=[
                {"id": "node1", "type": "problem", "content": "test"},
                {"id": "node2", "type": "solution", "content": "solution"}
            ],
            edges=[
                {"id": "edge1", "source": "node1", "target": "node2"}
            ],
            viewport={"x": 100, "y": 50, "zoom": 1.5}
        )

        assert session.title == "Architecture Planning"
        assert len(session.nodes) == 2
        assert len(session.edges) == 1
        assert session.viewport == {"x": 100, "y": 50, "zoom": 1.5}

    def test_brainstorm_session_empty(self):
        """Should create empty session"""
        session = BrainstormSession(
            project_id=uuid4(),
            title="Empty Session",
            nodes=[],
            edges=[]
        )

        assert session.nodes == []
        assert session.edges == []

    def test_brainstorm_session_viewport_optional(self):
        """Viewport should be optional"""
        session = BrainstormSession(
            project_id=uuid4(),
            title="Test",
            nodes=[],
            edges=[]
        )

        assert session.viewport is None


class TestChangeRequestModel:
    """Test ChangeRequest model attributes"""

    def test_change_request_creation(self):
        """Should create ChangeRequest with all fields"""
        evaluation_id = uuid4()
        project_id = uuid4()
        cr = ChangeRequest(
            evaluation_id=evaluation_id,
            project_id=project_id,
            title="Add error handling to API",
            type="feature",
            summary="Implement try-catch blocks",
            tasks=[
                {"title": "Add try-catch", "description": "Wrap API calls", "priority": "high"},
                {"title": "Add logging", "description": "Log errors", "priority": "medium"}
            ],
            estimation={
                "optimistic": {"value": 2, "unit": "days"},
                "expected": {"value": 3, "unit": "days"},
                "pessimistic": {"value": 5, "unit": "days"},
                "complexity": "medium"
            },
            dependencies=[
                {"title": "Logger setup", "description": "Configure logging framework"}
            ],
            risk_factors=[
                {"title": "Breaking changes", "description": "Could affect clients", "severity": "medium", "mitigation": "Backwards compatibility"}
            ],
            template_id="claude-code",
            status="pending"
        )

        assert cr.title == "Add error handling to API"
        assert cr.type == "feature"
        assert len(cr.tasks) == 2
        assert cr.estimation["complexity"] == "medium"
        assert len(cr.dependencies) == 1
        assert len(cr.risk_factors) == 1
        assert cr.template_id == "claude-code"
        assert cr.status == "pending"

    def test_change_request_types(self):
        """Should accept valid CR types"""
        evaluation_id = uuid4()
        project_id = uuid4()
        valid_types = ["feature", "refactor", "bugfix", "research"]

        for cr_type in valid_types:
            cr = ChangeRequest(
                evaluation_id=evaluation_id,
                project_id=project_id,
                title="Test",
                type=cr_type
            )
            assert cr.type == cr_type

    def test_change_request_statuses(self):
        """Should accept valid CR statuses"""
        evaluation_id = uuid4()
        project_id = uuid4()
        valid_statuses = ["pending", "in-progress", "completed", "rejected"]

        for status in valid_statuses:
            cr = ChangeRequest(
                evaluation_id=evaluation_id,
                project_id=project_id,
                title="Test",
                type="feature",
                status=status
            )
            assert cr.status == status

    def test_change_request_default_status(self):
        """Default status should be pending"""
        cr = ChangeRequest(
            evaluation_id=uuid4(),
            project_id=uuid4(),
            title="Test",
            type="feature"
        )

        assert cr.status == "pending"


class TestRelationships:
    """Test model relationships and cascade behavior"""

    def test_project_to_codebase_relationship(self):
        """Project should have one-to-many relationship with Codebase"""
        project = Project(name="Test Project")
        codebase1 = Codebase(
            project_id=project.id,
            name="repo1",
            type="github"
        )
        codebase2 = Codebase(
            project_id=project.id,
            name="repo2",
            type="local"
        )

        project.codebases = [codebase1, codebase2]

        assert len(project.codebases) == 2
        assert codebase1.project_id == project.id
        assert codebase2.project_id == project.id

    def test_codebase_to_files_relationship(self):
        """Codebase should have one-to-many relationship with CodebaseFile"""
        codebase = Codebase(
            project_id=uuid4(),
            name="test",
            type="local"
        )
        file1 = CodebaseFile(
            codebase_id=codebase.id,
            name="app.py",
            path="src/app.py"
        )
        file2 = CodebaseFile(
            codebase_id=codebase.id,
            name="main.ts",
            path="src/main.ts"
        )

        codebase.files = [file1, file2]

        assert len(codebase.files) == 2
        assert file1.codebase_id == codebase.id
        assert file2.codebase_id == codebase.id

    def test_evaluation_to_results_relationship(self):
        """Evaluation should have one-to-many relationship with EvaluationResult"""
        evaluation = Evaluation(
            project_id=uuid4(),
            input_type="repo",
            input_name="test",
            provider_id="gemini"
        )
        result1 = EvaluationResult(
            evaluation_id=evaluation.id,
            value_score=85,
            executive_summary="Test"
        )
        result2 = EvaluationResult(
            evaluation_id=evaluation.id,
            value_score=90,
            executive_summary="Test2"
        )

        evaluation.results = [result1, result2]

        assert len(evaluation.results) == 2
        assert result1.evaluation_id == evaluation.id
        assert result2.evaluation_id == evaluation.id

    def test_evaluation_to_change_requests_relationship(self):
        """Evaluation should have one-to-many relationship with ChangeRequest"""
        evaluation = Evaluation(
            project_id=uuid4(),
            input_type="repo",
            input_name="test",
            provider_id="gemini"
        )
        cr1 = ChangeRequest(
            evaluation_id=evaluation.id,
            project_id=uuid4(),
            title="CR1",
            type="feature"
        )
        cr2 = ChangeRequest(
            evaluation_id=evaluation.id,
            project_id=uuid4(),
            title="CR2",
            type="bugfix"
        )

        evaluation.change_requests = [cr1, cr2]

        assert len(evaluation.change_requests) == 2
        assert cr1.evaluation_id == evaluation.id
        assert cr2.evaluation_id == evaluation.id

    def test_project_to_evaluations_relationship(self):
        """Project should have one-to-many relationship with Evaluation"""
        project = Project(name="Test Project")
        eval1 = Evaluation(
            project_id=project.id,
            input_type="repo",
            input_name="repo1",
            provider_id="gemini"
        )
        eval2 = Evaluation(
            project_id=project.id,
            input_type="pdf",
            input_name="paper.pdf",
            provider_id="claude"
        )

        project.evaluations = [eval1, eval2]

        assert len(project.evaluations) == 2
        assert eval1.project_id == project.id
        assert eval2.project_id == project.id

    def test_project_to_brainstorm_sessions_relationship(self):
        """Project should have one-to-many relationship with BrainstormSession"""
        project = Project(name="Test Project")
        session1 = BrainstormSession(
            project_id=project.id,
            title="Session1",
            nodes=[],
            edges=[]
        )
        session2 = BrainstormSession(
            project_id=project.id,
            title="Session2",
            nodes=[],
            edges=[]
        )

        project.brainstorm_sessions = [session1, session2]

        assert len(project.brainstorm_sessions) == 2
        assert session1.project_id == project.id
        assert session2.project_id == project.id

    def test_project_to_change_requests_relationship(self):
        """Project should have one-to-many relationship with ChangeRequest"""
        project = Project(name="Test Project")
        cr1 = ChangeRequest(
            evaluation_id=uuid4(),
            project_id=project.id,
            title="CR1",
            type="feature"
        )
        cr2 = ChangeRequest(
            evaluation_id=uuid4(),
            project_id=project.id,
            title="CR2",
            type="refactor"
        )

        project.change_requests = [cr1, cr2]

        assert len(project.change_requests) == 2
        assert cr1.project_id == project.id
        assert cr2.project_id == project.id

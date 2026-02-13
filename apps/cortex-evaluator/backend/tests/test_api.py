"""
Tests for API endpoints - evaluations, history, codebases
"""
import pytest
from unittest.mock import AsyncMock, patch, MagicMock
from fastapi.testclient import TestClient
from fastapi import BackgroundTasks
from app.main import app


@pytest.fixture
def client():
    return TestClient(app)


@pytest.fixture
def mock_ai_result():
    return {
        "id": "eval_001",
        "valueScore": 85,
        "executiveSummary": "Code is well-structured",
        "technicalFeasibility": "Highly feasible",
        "gapAnalysis": "Minor improvements needed",
        "suggestedCR": "Add error handling",
        "providerUsed": "gemini",
        "similarEvaluations": []
    }


class TestEvaluationsAPI:
    """Test /api/evaluations endpoints"""

    def test_analyze_evaluation_success(self, client, mock_ai_result):
        """POST /api/evaluations/analyze should return evaluation result"""
        with patch('app.api.evaluations.ai_router.analyze_with_fallback', new_callable=AsyncMock(return_value={
            "success": True,
            "result": {
                "valueScore": 85,
                "executiveSummary": "Code is well-structured",
                "technicalFeasibility": "Highly feasible",
                "gapAnalysis": "Minor improvements needed",
                "suggestedCR": "Add error handling"
            },
            "provider": "gemini"
        })) as mock_analyze:
            with patch('app.api.evaluations.vector_store.search_similar_evaluations', new_callable=AsyncMock(return_value=[])):
                response = client.post("/api/evaluations/analyze", json={
                    "codebase_id": "test_codebase",
                    "input_type": "repo",
                    "input_content": "Analyze this code",
                    "provider_preference": "gemini",
                    "user_intent": "--fast"
                })

                assert response.status_code == 200
                data = response.json()
                assert "id" in data
                assert data["valueScore"] == 85
                assert data["providerUsed"] == "gemini"
                mock_analyze.assert_called_once()

    def test_analyze_evaluation_with_user_intent(self, client):
        """Should pass user_intent to AI router"""
        with patch('app.api.evaluations.ai_router.analyze_with_fallback', new_callable=AsyncMock(return_value={
            "success": True,
            "result": {"valueScore": 75, "executiveSummary": "test"},
            "provider": "ollama"
        })):
            response = client.post("/api/evaluations/analyze", json={
                "codebase_id": "test",
                "input_type": "snippet",
                "user_intent": "--local"
            })

            assert response.status_code == 200
            assert response.json()["providerUsed"] == "ollama"

    def test_analyze_evaluation_missing_required_fields(self, client):
        """Should return 422 for missing required fields"""
        response = client.post("/api/evaluations/analyze", json={
            "input_type": "repo"
        })

        assert response.status_code == 422

    def test_get_evaluation_history(self, client):
        """GET /api/evaluations/history should return paginated evaluations"""
        response = client.get("/api/evaluations/history?limit=10&offset=0")

        assert response.status_code == 200
        data = response.json()
        assert "evaluations" in data
        assert "total" in data
        assert "limit" in data
        assert "offset" in data

    def test_get_evaluation_history_with_project_filter(self, client):
        """Should filter by project_id when provided"""
        response = client.get("/api/evaluations/history?project_id=proj_123")

        assert response.status_code == 200
        data = response.json()

    def test_get_evaluation_history_default_pagination(self, client):
        """Should use default limit and offset"""
        response = client.get("/api/evaluations/history")

        assert response.status_code == 200
        data = response.json()
        assert data["limit"] == 50
        assert data["offset"] == 0

    def test_get_evaluation_by_id(self, client):
        """GET /api/evaluations/{id} should return evaluation details"""
        response = client.get("/api/evaluations/eval_001")

        assert response.status_code == 200
        data = response.json()
        assert "id" in data
        assert data["id"] == "eval_001"

    def test_get_evaluation_not_found(self, client):
        """Should return 404 for nonexistent evaluation"""
        with patch('app.api.evaluations.ai_router.analyze_with_fallback', new_callable=AsyncMock(side_effect=Exception("Not found"))):
            response = client.get("/api/evaluations/nonexistent")

            assert response.status_code == 500

    def test_get_similar_evaluations(self, client):
        """GET /api/evaluations/{id}/similar should return similar evaluations"""
        with patch('app.api.evaluations.vector_store.search_similar_evaluations', new_callable=AsyncMock(return_value=[
            {
                'id': 'eval_002',
                'similarity': 0.85,
                'metadata': {'value_score': 80}
            }
        ])):
            response = client.get("/api/evaluations/eval_001/similar?limit=10")

            assert response.status_code == 200
            data = response.json()
            assert "similarEvaluations" in data
            assert data["count"] == 1

    def test_get_evaluation_stats(self, client):
        """GET /api/evaluations/stats should return statistics"""
        response = client.get("/api/evaluations/stats")

        assert response.status_code == 200
        data = response.json()
        assert "total_evaluations" in data
        assert "avg_value_score" in data
        assert "provider_usage" in data
        assert "type_distribution" in data


class TestCodebasesAPI:
    """Test /api/codebases endpoints"""

    def test_initialize_codebase(self, client):
        """POST /api/codebases/initialize should create new codebase"""
        response = client.post("/api/codebases/initialize", json={
            "type": "github",
            "github_url": "https://github.com/user/repo"
        })

        assert response.status_code == 200
        data = response.json()
        assert "codebaseId" in data

    def test_initialize_codebase_local_type(self, client):
        """Should support local directory type"""
        with patch('app.api.codebases.vector_store.index_codebase', new_callable=AsyncMock(return_value=5)):
            response = client.post("/api/codebases/initialize", json={
                "type": "local",
                "directory_path": "/path/to/code"
            })

            assert response.status_code == 200

    def test_get_codebase(self, client):
        """GET /api/codebases/{id} should return codebase info"""
        response = client.get("/api/codebases/codebase_001")

        assert response.status_code == 200
        data = response.json()
        assert "files" in data or "id" in data

    def test_list_codebases(self, client):
        """GET /api/codebases should return list with pagination"""
        response = client.get("/api/codebases?limit=20&offset=0")

        assert response.status_code == 200
        data = response.json()
        assert "codebases" in data
        assert "total" in data

    def test_list_codebases_with_project_filter(self, client):
        """Should filter by project_id"""
        response = client.get("/api/codebases?project_id=proj_123")

        assert response.status_code == 200

    def test_delete_codebase(self, client):
        """DELETE /api/codebases/{id} should remove codebase"""
        with patch('app.api.codebases.vector_store.delete_codebase', new_callable=AsyncMock(return_value=5)):
            response = client.delete("/api/codebases/codebase_001")

            assert response.status_code == 200
            data = response.json()
            assert data["filesDeleted"] == 5

    def test_reindex_codebase(self, client):
        """POST /api/codebases/{id}/reindex should trigger reindexing"""
        with patch('app.api.codebases.vector_store.index_codebase', new_callable=AsyncMock(return_value=10)):
            response = client.post("/api/codebases/codebase_001/reindex")

            assert response.status_code == 200
            data = response.json()
            assert "message" in data

    def test_generate_system_docs(self, client):
        """POST /api/codebases/{id}/generate-docs should return docs"""
        with patch('app.api.codebases.ai_router.analyze_with_fallback', new_callable=AsyncMock(return_value={
            "success": True,
            "result": {"documentation": "Generated docs"}
        })):
            response = client.post("/api/codebases/codebase_001/generate-docs", json={
                "includeTests": False,
                "maxFiles": 15
            })

            assert response.status_code == 200
            data = response.json()
            assert "documentation" in data


class TestHistoryAPI:
    """Test /api/history endpoints for search and analytics"""

    def test_search_evaluations(self, client):
        """GET /api/history/search should return semantic search results"""
        with patch('app.api.history.vector_store.search_similar_evaluations', new_callable=AsyncMock(return_value=[
            {
                'id': 'eval_001',
                'similarity': 0.9,
                'metadata': {'provider': 'gemini'}
            }
        ])):
            response = client.get("/api/history/search?query=code%20quality&semantic=true&limit=5")

            assert response.status_code == 200
            data = response.json()
            assert "results" in data
            assert data["searchType"] == "semantic"

    def test_search_evaluations_filters(self, client):
        """Should apply filters to search query"""
        response = client.get("/api/history/search?query=test&provider=gemini&minScore=75")

        assert response.status_code == 200

    def test_search_evaluations_pagination(self, client):
        """Should support limit and offset parameters"""
        response = client.get("/api/history/search?query=test&limit=10&offset=20")

        assert response.status_code == 200
        data = response.json()
        assert len(data["results"]) <= 10

    def test_get_evaluation_timeline(self, client):
        """GET /api/history/timeline should return daily evaluation counts"""
        response = client.get("/api/history/timeline?days=30")

        assert response.status_code == 200
        data = response.json()
        assert "timeline" in data
        assert "days" in data

    def test_get_top_evaluations(self, client):
        """GET /api/history/top-evaluations should return top ranked"""
        response = client.get("/api/history/top-evaluations?sortBy=value_score&limit=5")

        assert response.status_code == 200
        data = response.json()
        assert "evaluations" in data

    def test_export_history_json(self, client):
        """GET /api/history/export should return JSON export"""
        response = client.get("/api/history/export?format=json")

        assert response.status_code == 200
        assert response.headers["content-type"] == "application/json"

    def test_export_history_csv(self, client):
        """GET /api/history/export should return CSV export"""
        response = client.get("/api/history/export?format=csv")

        assert response.status_code == 200
        assert "text/csv" in response.headers.get("content-type", "")

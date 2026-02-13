"""
Tests for Vector DB - ChromaDB operations for code, evaluations, and papers
"""
import pytest
from unittest.mock import MagicMock, patch, AsyncMock
from app.services.vector_db import VectorStore


@pytest.mark.asyncio
class TestVectorStoreInitialization:
    """Test VectorStore initialization and configuration"""

    async def test_vectorstore_initializes_collections(self):
        """VectorStore should initialize all three collections"""
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            with patch('app.services.vector_db.settings.CHROMA_PATH', './test_data'):
                store = VectorStore()
                assert store.code_snippets_collection is not None
                assert store.evaluations_collection is not None
                assert store.papers_collection is not None

    async def test_vectorstore_uses_correct_embedding_function(self):
        """Should use OpenAI embeddings if key available, else SentenceTransformer"""
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            # Test with OpenAI key
            with patch('app.services.vector_db.settings.OPENAI_API_KEY', 'test_key'):
                store = VectorStore()
                assert 'OpenAI' in str(type(store.embedding_function))

            # Test without OpenAI key
            with patch('app.services.vector_db.settings.OPENAI_API_KEY', ''):
                store = VectorStore()
                assert 'SentenceTransformer' in str(type(store.embedding_function))


@pytest.mark.asyncio
class TestCodebaseIndexing:
    """Test codebase indexing operations"""

    @pytest.fixture
    def sample_files(self):
        return [
            {
                'file_path': 'src/app.py',
                'content': 'from fastapi import FastAPI\napp = FastAPI()',
                'language': 'python',
                'function_name': None
            },
            {
                'file_path': 'src/utils.py',
                'content': 'def helper():\n    return "hello"',
                'language': 'python',
                'function_name': 'helper'
            },
            {
                'file_path': 'README.md',
                'content': '# Test Project\n\nThis is a README',
                'language': 'markdown',
                'function_name': None
            }
        ]

    @pytest.fixture
    def vector_store(self):
        """Create vector store with mocked client"""
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            with patch('app.services.vector_db.settings.CHROMA_PATH', './test_data'):
                return VectorStore()

    async def test_index_codebase_empty_files(self, vector_store):
        """Should handle empty files list gracefully"""
        count = await vector_store.index_codebase('test_id', [])
        assert count == 0

    async def test_index_codebase_success(self, vector_store, sample_files):
        """Should index all files and return count"""
        count = await vector_store.index_codebase('test_codebase', sample_files)
        assert count == 3

    async def test_index_codebase_with_progress_callback(self, vector_store, sample_files):
        """Should call progress callback during indexing"""
        progress_calls = []

        def on_progress(current, total):
            progress_calls.append((current, total))

        await vector_store.index_codebase('test_codebase', sample_files, on_progress)

        assert len(progress_calls) == 1
        assert progress_calls[0] == (3, 3)

    async def test_index_codebase_handles_duplicates(self, vector_store, sample_files):
        """Should handle duplicate file IDs gracefully"""
        with patch.object(vector_store.code_snippets_collection, 'get', return_value={'ids': ['test_codebase:src/app.py']}):
            count = await vector_store.index_codebase('test_codebase', sample_files)
            assert count == 2  # Should skip duplicate

    async def test_index_codebase_batches_correctly(self, vector_store):
        """Should batch process files (100 per batch)"""
        large_file_list = [
            {
                'file_path': f'file_{i}.py',
                'content': 'x' * 100,
                'language': 'python',
                'function_name': None
            }
            for i in range(250)
        ]

        with patch.object(vector_store.code_snippets_collection, 'add'):
            await vector_store.index_codebase('test_codebase', large_file_list)
            assert vector_store.code_snippets_collection.add.call_count == 3  # 250 / 100 = 3 batches


@pytest.mark.asyncio
class TestCodeSearch:
    """Test code similarity search operations"""

    @pytest.fixture
    def vector_store(self):
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            with patch('app.services.vector_db.settings.CHROMA_PATH', './test_data'):
                return VectorStore()

    async def test_search_similar_code_returns_results(self, vector_store):
        """Should return similar code with metadata"""
        mock_query_result = {
            'ids': [['id1', 'id2']],
            'documents': [['code1', 'code2']],
            'metadatas': [[
                {'file_path': 'test.py', 'language': 'python'},
                {'file_path': 'main.ts', 'language': 'typescript'}
            ]],
            'distances': [[0.1, 0.2]]
        }

        with patch.object(vector_store.code_snippets_collection, 'query', return_value=mock_query_result):
            results = await vector_store.search_similar_code('function that does X', n_results=2)

            assert len(results) == 2
            assert results[0]['file_path'] == 'test.py'
            assert results[0]['language'] == 'python'
            assert results[0]['similarity'] == 0.9  # 1 - 0.1
            assert 'distance' in results[0]

    async def test_search_similar_code_filters_by_codebase(self, vector_store):
        """Should filter results by codebase_id"""
        mock_query_result = {
            'ids': [['id1']],
            'documents': [['code']],
            'metadatas': [[{'file_path': 'test.py', 'codebase_id': 'project_a'}]],
            'distances': [[0.1]]
        }

        with patch.object(vector_store.code_snippets_collection, 'query', return_value=mock_query_result) as mock_query:
            await vector_store.search_similar_code('test', codebase_id='project_a', n_results=5)
            mock_query.assert_called_once()
            where_arg = mock_query.call_args[1]['where']
            assert where_arg['codebase_id'] == 'project_a'

    async def test_search_similar_code_no_results(self, vector_store):
        """Should return empty list when no results found"""
        mock_query_result = {
            'ids': [[]],
            'documents': [[]],
            'metadatas': [[]],
            'distances': [[]]
        }

        with patch.object(vector_store.code_snippets_collection, 'query', return_value=mock_query_result):
            results = await vector_store.search_similar_code('test')
            assert len(results) == 0


@pytest.mark.asyncio
class TestEvaluationStorage:
    """Test evaluation storage and retrieval operations"""

    @pytest.fixture
    def vector_store(self):
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            with patch('app.services.vector_db.settings.CHROMA_PATH', './test_data'):
                return VectorStore()

    async def test_store_evaluation_success(self, vector_store):
        """Should store evaluation with metadata"""
        evaluation_id = await vector_store.store_evaluation(
            evaluation_id='eval_001',
            summary='Good code quality',
            cr='Add error handling',
            metadata={
                'project_id': 'project_1',
                'provider': 'gemini',
                'value_score': 85
            }
        )

        assert evaluation_id == 'eval_001'
        vector_store.evaluations_collection.add.assert_called_once()
        call_args = vector_store.evaluations_collection.add.call_args[1]
        assert 'eval_001' in call_args['ids']
        assert call_args['metadatas'][0]['project_id'] == 'project_1'

    async def test_store_evaluation_handles_duplicate(self, vector_store):
        """Should delete and re-add on duplicate ID"""
        with patch.object(vector_store.evaluations_collection, 'delete'):
            with patch.object(vector_store, 'store_evaluation', wraps=vector_store.store_evaluation) as mock_store:
                # First call succeeds
                await vector_store.store_evaluation('eval_001', 'summary', 'cr', {})

                # Second call with same ID triggers delete
                vector_store.evaluations_collection.add.side_effect = Exception("Duplicate")
                with patch('app.services.vector_db.chroma_errors.DuplicateIDException'):
                    await vector_store.store_evaluation('eval_001', 'summary', 'cr', {})

    async def test_search_similar_evaluations(self, vector_store):
        """Should return evaluations sorted by similarity"""
        mock_query_result = {
            'ids': [['eval1', 'eval2']],
            'documents': [['summary1', 'summary2']],
            'metadatas': [[
                {'provider': 'gemini', 'value_score': 80},
                {'provider': 'claude', 'value_score': 90}
            ]],
            'distances': [[0.1, 0.15]]
        }

        with patch.object(vector_store.evaluations_collection, 'query', return_value=mock_query_result):
            results = await vector_store.search_similar_evaluations('code quality', n_results=2)

            assert len(results) == 2
            assert results[0]['similarity'] == 0.9
            assert results[0]['metadata']['provider'] == 'gemini'

    async def test_search_similar_evaluations_with_filters(self, vector_store):
        """Should apply metadata filters to search"""
        mock_query_result = {
            'ids': [['eval1']],
            'documents': [['summary']],
            'metadatas': [[{'provider': 'gemini', 'value_score': 80}]],
            'distances': [[0.1]]
        }

        with patch.object(vector_store.evaluations_collection, 'query', return_value=mock_query_result) as mock_query:
            await vector_store.search_similar_evaluations(
                'test',
                filters={'value_score': {'$gte': 75}},
                n_results=5
            )
            where_arg = mock_query.call_args[1]['where']
            assert where_arg['value_score']['$gte'] == 75


@pytest.mark.asyncio
class TestArxivPaperIndexing:
    """Test arXiv paper indexing operations"""

    @pytest.fixture
    def vector_store(self):
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            with patch('app.services.vector_db.settings.CHROMA_PATH', './test_data'):
                return VectorStore()

    async def test_index_arxiv_paper(self, vector_store):
        """Should index paper with all metadata"""
        paper_id = await vector_store.index_arxiv_paper(
            paper_id='2301.12345',
            title='Test Paper',
            authors=['John Doe', 'Jane Smith'],
            content='Abstract content here',
            metadata={
                'categories': ['cs.AI', 'cs.LG'],
                'published': '2023-01-15'
            }
        )

        assert paper_id == '2301.12345'
        vector_store.papers_collection.add.assert_called_once()
        call_args = vector_store.papers_collection.add.call_args[1]
        assert '2301.12345' in call_args['ids']
        assert 'Test Paper' in call_args['documents'][0]

    async def test_search_papers(self, vector_store):
        """Should search and return similar papers"""
        mock_query_result = {
            'ids': [['2301.12345']],
            'documents': [['Test Paper\n\nAbstract']],
            'metadatas': [[
                {'title': 'Test Paper', 'authors': 'John Doe, Jane Smith', 'categories': ['cs.AI']}
            ]],
            'distances': [[0.05]]
        }

        with patch.object(vector_store.papers_collection, 'query', return_value=mock_query_result):
            results = await vector_store.search_papers('machine learning', n_results=1)

            assert len(results) == 1
            assert results[0]['paper_id'] == '2301.12345'
            assert results[0]['title'] == 'Test Paper'
            assert results[0]['similarity'] == 0.95
            assert isinstance(results[0]['authors'], list)

    async def test_search_papers_with_categories(self, vector_store):
        """Should filter papers by arXiv categories"""
        with patch.object(vector_store.papers_collection, 'query') as mock_query:
            await vector_store.search_papers('AI', categories=['cs.AI', 'cs.LG'], n_results=5)
            where_arg = mock_query.call_args[1]['where']
            assert '$in' in where_arg['categories']
            assert set(where_arg['categories']['$in']) == {'cs.AI', 'cs.LG'}


@pytest.mark.asyncio
class TestCollectionStats:
    """Test collection statistics retrieval"""

    @pytest.fixture
    def vector_store(self):
        with patch('app.services.vector_db.chromadb.PersistentClient'):
            with patch('app.services.vector_db.settings.CHROMA_PATH', './test_data'):
                return VectorStore()

    async def test_get_collection_stats(self, vector_store):
        """Should return counts for all collections"""
        with patch.object(vector_store.code_snippets_collection, 'count', return_value=100):
            with patch.object(vector_store.evaluations_collection, 'count', return_value=50):
                with patch.object(vector_store.papers_collection, 'count', return_value=25):
                    stats = await vector_store.get_collection_stats()

                    assert stats == {
                        'code_snippets': 100,
                        'evaluations': 50,
                        'papers': 25
                    }

    async def test_delete_codebase(self, vector_store):
        """Should delete all documents for a codebase"""
        mock_get_result = {
            'ids': ['id1', 'id2', 'id3']
        }

        with patch.object(vector_store.code_snippets_collection, 'get', return_value=mock_get_result):
            count = await vector_store.delete_codebase('test_codebase')

            assert count == 3
            vector_store.code_snippets_collection.delete.assert_called_once_with(ids=['id1', 'id2', 'id3'])

    async def test_delete_codebase_no_docs(self, vector_store):
        """Should return 0 when no documents found"""
        with patch.object(vector_store.code_snippets_collection, 'get', return_value={'ids': []}):
            count = await vector_store.delete_codebase('nonexistent')
            assert count == 0
            vector_store.code_snippets_collection.delete.assert_not_called()

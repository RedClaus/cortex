"""
Test script for VectorStore service
"""
import asyncio
import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), 'backend'))

from app.services.vector_db import VectorStore


async def test_vector_store():
    """Test basic VectorStore functionality"""
    print("🚀 Testing VectorStore...")
    
    try:
        vector_store = VectorStore()
        print(f"✅ VectorStore initialized")
        
        stats = await vector_store.get_collection_stats()
        print(f"📊 Collection stats: {stats}")
        
        print("\n📝 Testing codebase indexing...")
        test_files = [
            {
                'file_path': 'test/app.py',
                'content': 'def hello(): return "Hello, World!"',
                'language': 'python',
                'function_name': 'hello'
            },
            {
                'file_path': 'test/main.ts',
                'content': 'function greet(name: string): string { return `Hello, ${name}!`; }',
                'language': 'typescript',
                'function_name': 'greet'
            },
            {
                'file_path': 'test/README.md',
                'content': '# Test Project\n\nThis is a test project for Cortex Evaluator.',
                'language': 'markdown'
            }
        ]
        
        progress_count = [0]
        def on_progress(current, total):
            progress_count[0] += 1
            print(f"  Progress: {current}/{total} files")
        
        indexed = await vector_store.index_codebase(
            codebase_id='test_codebase',
            files=test_files,
            on_progress=on_progress
        )
        print(f"✅ Indexed {indexed} files")
        
        print("\n🔍 Testing code search...")
        results = await vector_store.search_similar_code(
            query='function that returns a greeting',
            codebase_id='test_codebase',
            n_results=3
        )
        print(f"✅ Found {len(results)} results")
        for i, result in enumerate(results):
            print(f"\n  Result {i+1}:")
            print(f"    File: {result['file_path']}")
            print(f"    Language: {result['language']}")
            print(f"    Similarity: {result['similarity']:.4f}")
            print(f"    Code: {result['code'][:80]}...")
        
        print("\n💾 Testing evaluation storage...")
        await vector_store.store_evaluation(
            evaluation_id='eval_test_001',
            summary='Good code quality',
            cr='Consider adding error handling for edge cases.',
            metadata={
                'project_id': 'test_project',
                'provider': 'gemini',
                'value_score': 0.85
            }
        )
        print("✅ Evaluation stored")
        
        print("\n🔎 Testing evaluation search...")
        eval_results = await vector_store.search_similar_evaluations(
            query='code review about quality',
            n_results=3
        )
        print(f"✅ Found {len(eval_results)} similar evaluations")
        
        print("\n📚 Testing arXiv paper indexing...")
        await vector_store.index_arxiv_paper(
            paper_id='2301.12345',
            title='Test Paper on AI Evaluations',
            authors=['Author One', 'Author Two'],
            content='This is a test abstract for demonstrating arXiv paper indexing.',
            metadata={
                'categories': ['cs.AI', 'cs.LG'],
                'published': '2023-01-15'
            }
        )
        print("✅ Paper indexed")
        
        print("\n📖 Testing paper search...")
        paper_results = await vector_store.search_papers(
            query='AI evaluation methods',
            n_results=3
        )
        print(f"✅ Found {len(paper_results)} similar papers")
        
        final_stats = await vector_store.get_collection_stats()
        print(f"\n📊 Final collection stats: {final_stats}")
        
        print("\n✅ All tests passed!")
        
    except Exception as e:
        print(f"\n❌ Test failed: {e}")
        import traceback
        traceback.print_exc()
        return False
    
    return True


if __name__ == '__main__':
    result = asyncio.run(test_vector_store())
    sys.exit(0 if result else 1)

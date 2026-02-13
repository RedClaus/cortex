"""
Cortex Evaluator - Vector Database Service
ChromaDB integration for code search, evaluation history, and research papers
"""
import asyncio
import logging
from typing import Callable, Optional, List, Dict, Any
import chromadb
from chromadb.utils import embedding_functions
import chromadb.errors as chroma_errors

from app.core.config import settings

logger = logging.getLogger(__name__)


class VectorStore:
    """Vector database service for code search and evaluation history"""

    def __init__(self):
        """Initialize ChromaDB PersistentClient and collections"""
        self.client: Optional[chromadb.PersistentClient] = None
        self.code_snippets_collection = None
        self.evaluations_collection = None
        self.papers_collection = None
        self.embedding_function = None
        
        self._initialize()

    def _initialize(self):
        """Initialize ChromaDB client and embedding function"""
        try:
            self.client = chromadb.PersistentClient(
                path=settings.CHROMA_PATH
            )
            
            if settings.OPENAI_API_KEY:
                logger.info("Using OpenAI embeddings")
                self.embedding_function = embedding_functions.OpenAIEmbeddingFunction(
                    api_key=settings.OPENAI_API_KEY,
                    model_name="text-embedding-3-small"
                )
            else:
                logger.info("Using SentenceTransformer embeddings (local)")
                self.embedding_function = embedding_functions.SentenceTransformerEmbeddingFunction(
                    model_name="all-MiniLM-L6-v2",
                    device="cpu"
                )
            
            self.code_snippets_collection = self.client.get_or_create_collection(
                name="code_snippets",
                embedding_function=self.embedding_function,
                metadata={"hnsw:space": "cosine", "description": "Code snippets for search"}
            )
            
            self.evaluations_collection = self.client.get_or_create_collection(
                name="evaluations",
                embedding_function=self.embedding_function,
                metadata={"hnsw:space": "cosine", "description": "Evaluation history"}
            )
            
            self.papers_collection = self.client.get_or_create_collection(
                name="papers",
                embedding_function=self.embedding_function,
                metadata={"hnsw:space": "cosine", "description": "arXiv research papers"}
            )
            
            logger.info(f"VectorDB initialized at {settings.CHROMA_PATH}")
            logger.info(f"Collections: code_snippets ({self.code_snippets_collection.count()} docs), "
                       f"evaluations ({self.evaluations_collection.count()} docs), "
                       f"papers ({self.papers_collection.count()} docs)")
            
        except Exception as e:
            logger.error(f"Failed to initialize VectorDB: {e}")
            raise

    async def index_codebase(
        self,
        codebase_id: str,
        files: List[Dict[str, Any]],
        on_progress: Optional[Callable[[int, int], None]] = None
    ) -> int:
        """
        Index a codebase by processing files and generating embeddings
        
        Args:
            codebase_id: Unique identifier for the codebase
            files: List of file dictionaries with keys:
                - file_path: str - Path to the file
                - content: str - File content
                - language: str - Programming language
                - function_name: Optional[str] - Function/class name if applicable
            on_progress: Optional callback function(current, total) for progress updates
        
        Returns:
            Number of files indexed
        """
        if not files:
            logger.warning(f"No files to index for codebase {codebase_id}")
            return 0
        
        batch_size = 100
        total_files = len(files)
        indexed_count = 0
        
        try:
                for i in range(0, total_files, batch_size):
                    batch = files[i:i + batch_size]
                    
                    ids = []
                    documents = []
                    metadatas = []
                    
                    for file in batch:
                        file_id = f"{codebase_id}:{file['file_path']}"
                        ids.append(file_id)
                        documents.append(file['content'])
                        metadatas.append({
                            'file_path': file['file_path'],
                            'language': file.get('language', 'unknown'),
                            'function_name': file.get('function_name', ''),
                            'codebase_id': codebase_id,
                            'size': len(file['content'])
                        })
                    
                    self.code_snippets_collection.add(
                        ids=ids,
                        documents=documents,
                        metadatas=metadatas
                    )
                    
                    indexed_count += len(batch)
                    
                    if on_progress:
                        await asyncio.to_thread(on_progress, indexed_count, total_files)
            
                logger.info(f"Completed indexing {indexed_count} files for codebase {codebase_id}")
                return indexed_count
            
        except chroma_errors.DuplicateIDException as e:
            logger.warning(f"Duplicate IDs detected: {e}")
            try:
                existing_results = self.code_snippets_collection.get(
                    where={"codebase_id": codebase_id}
                )
                existing_ids = set(existing_results['ids'])
                
                new_files = [
                    f for f in files 
                    if f"{codebase_id}:{f['file_path']}" not in existing_ids
                ]
                
                if new_files:
                    logger.info(f"Re-indexing {len(new_files)} new files (excluding duplicates)")
                    return await self.index_codebase(codebase_id, new_files, on_progress)
                else:
                    logger.info("All files already indexed")
                    return 0
                    
            except Exception as retry_e:
                logger.error(f"Failed to handle duplicates: {retry_e}")
                raise
                
        except Exception as e:
            logger.error(f"Error indexing codebase {codebase_id}: {e}")
            raise

    async def search_similar_code(
        self,
        query: str,
        codebase_id: Optional[str] = None,
        n_results: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Search for similar code snippets
        
        Args:
            query: Search query
            codebase_id: Optional filter for specific codebase
            n_results: Number of results to return
        
        Returns:
            List of results with keys: code, file_path, language, similarity
        """
        try:
            where = None
            if codebase_id:
                where = {"codebase_id": codebase_id}
            
            results = self.code_snippets_collection.query(
                query_texts=[query],
                n_results=n_results,
                where=where,
                include=["documents", "metadatas", "distances"]
            )
            
            formatted_results = []
            if results['ids'] and len(results['ids'][0]) > 0:
                for i in range(len(results['ids'][0])):
                    formatted_results.append({
                        'code': results['documents'][0][i],
                        'file_path': results['metadatas'][0][i].get('file_path'),
                        'language': results['metadatas'][0][i].get('language'),
                        'function_name': results['metadatas'][0][i].get('function_name'),
                        'similarity': 1 - results['distances'][0][i],
                        'distance': results['distances'][0][i]
                    })
            
            logger.info(f"Found {len(formatted_results)} similar code snippets for query")
            return formatted_results
            
        except chroma_errors.NoIndexException:
            logger.warning("No embeddings in code_snippets collection")
            return []
            
        except chroma_errors.NotEnoughElementsException:
            logger.warning(f"Not enough results (requested {n_results})")
            if n_results > 1:
                return await self.search_similar_code(query, codebase_id, n_results - 1)
            return []
            
        except Exception as e:
            logger.error(f"Error searching similar code: {e}")
            raise

    async def store_evaluation(
        self,
        evaluation_id: str,
        summary: str,
        cr: str,
        metadata: Dict[str, Any]
    ) -> str:
        """
        Store an evaluation result in the vector database
        
        Args:
            evaluation_id: Unique identifier for the evaluation
            summary: Evaluation summary
            cr: Code review content
            metadata: Additional metadata (project_id, provider, scores, etc.)
        
        Returns:
            ID of the stored evaluation
        """
        try:
            content = f"{summary}\n\n{cr}"
            
            self.evaluations_collection.add(
                ids=[evaluation_id],
                documents=[content],
                metadatas=[{
                    **metadata,
                    'evaluation_id': evaluation_id,
                    'has_summary': bool(summary),
                    'has_cr': bool(cr)
                }]
            )
            
            logger.info(f"Stored evaluation {evaluation_id}")
            return evaluation_id
            
        except chroma_errors.DuplicateIDException:
            logger.warning(f"Evaluation {evaluation_id} already exists, updating...")
            self.evaluations_collection.delete(ids=[evaluation_id])
            return await self.store_evaluation(evaluation_id, summary, cr, metadata)
            
        except Exception as e:
            logger.error(f"Error storing evaluation {evaluation_id}: {e}")
            raise

    async def search_similar_evaluations(
        self,
        query: str,
        project_id: Optional[str] = None,
        filters: Optional[Dict[str, Any]] = None,
        n_results: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Search for similar evaluations
        
        Args:
            query: Search query
            project_id: Optional filter for specific project
            filters: Optional metadata filters (provider, value_score range, etc.)
            n_results: Number of results to return
        
        Returns:
            List of results with keys: id, similarity, metadata
        """
        try:
            where = {}
            if project_id:
                where['project_id'] = project_id
            
            if filters:
                where.update(filters)
            
            if not where:
                where = None
            
            results = self.evaluations_collection.query(
                query_texts=[query],
                n_results=n_results,
                where=where,
                include=["documents", "metadatas", "distances"]
            )
            
            formatted_results = []
            if results['ids'] and len(results['ids'][0]) > 0:
                for i in range(len(results['ids'][0])):
                    metadata = results['metadatas'][0][i]
                    formatted_results.append({
                        'id': results['ids'][0][i],
                        'similarity': 1 - results['distances'][0][i],
                        'distance': results['distances'][0][i],
                        'metadata': metadata,
                        'summary': metadata.get('summary', ''),
                        'content': results['documents'][0][i]
                    })
            
            logger.info(f"Found {len(formatted_results)} similar evaluations")
            return formatted_results
            
        except chroma_errors.NoIndexException:
            logger.warning("No embeddings in evaluations collection")
            return []
            
        except chroma_errors.NotEnoughElementsException:
            logger.warning(f"Not enough evaluation results (requested {n_results})")
            if n_results > 1:
                return await self.search_similar_evaluations(
                    query, project_id, filters, n_results - 1
                )
            return []
            
        except Exception as e:
            logger.error(f"Error searching similar evaluations: {e}")
            raise

    async def index_arxiv_paper(
        self,
        paper_id: str,
        title: str,
        authors: List[str],
        content: str,
        metadata: Dict[str, Any]
    ) -> str:
        """
        Index an arXiv research paper
        
        Args:
            paper_id: arXiv paper ID (e.g., "2301.12345")
            title: Paper title
            authors: List of author names
            content: Paper content/abstract
            metadata: Additional metadata (categories, published date, etc.)
        
        Returns:
            ID of the indexed paper
        """
        try:
            document = f"{title}\n\n{content}"
            
            self.papers_collection.add(
                ids=[paper_id],
                documents=[document],
                metadatas=[{
                    'paper_id': paper_id,
                    'title': title,
                    'authors': ', '.join(authors),
                    'author_count': len(authors),
                    **metadata
                }]
            )
            
            logger.info(f"Indexed arXiv paper {paper_id}: {title}")
            return paper_id
            
        except chroma_errors.DuplicateIDException:
            logger.warning(f"Paper {paper_id} already exists, updating...")
            self.papers_collection.delete(ids=[paper_id])
            return await self.index_arxiv_paper(paper_id, title, authors, content, metadata)
            
        except Exception as e:
            logger.error(f"Error indexing paper {paper_id}: {e}")
            raise

    async def search_papers(
        self,
        query: str,
        categories: Optional[List[str]] = None,
        n_results: int = 5
    ) -> List[Dict[str, Any]]:
        """
        Search for similar research papers
        
        Args:
            query: Search query
            categories: Optional list of arXiv categories to filter
            n_results: Number of results to return
        
        Returns:
            List of results with keys: paper_id, title, authors, similarity, metadata
        """
        try:
            where = None
            if categories:
                where = {"categories": {"$in": categories}}
            
            results = self.papers_collection.query(
                query_texts=[query],
                n_results=n_results,
                where=where,
                include=["documents", "metadatas", "distances"]
            )
            
            formatted_results = []
            if results['ids'] and len(results['ids'][0]) > 0:
                for i in range(len(results['ids'][0])):
                    metadata = results['metadatas'][0][i]
                    formatted_results.append({
                        'paper_id': results['ids'][0][i],
                        'title': metadata.get('title', ''),
                        'authors': metadata.get('authors', '').split(', '),
                        'similarity': 1 - results['distances'][0][i],
                        'distance': results['distances'][0][i],
                        'metadata': metadata,
                        'content': results['documents'][0][i]
                    })
            
            logger.info(f"Found {len(formatted_results)} similar papers")
            return formatted_results
            
        except chroma_errors.NoIndexException:
            logger.warning("No embeddings in papers collection")
            return []
            
        except chroma_errors.NotEnoughElementsException:
            logger.warning(f"Not enough papers (requested {n_results})")
            if n_results > 1:
                return await self.search_papers(query, categories, n_results - 1)
            return []
            
        except Exception as e:
            logger.error(f"Error searching papers: {e}")
            raise

    async def delete_codebase(self, codebase_id: str) -> int:
        """
        Delete all documents for a codebase
        
        Args:
            codebase_id: Unique identifier for the codebase
        
        Returns:
            Number of documents deleted
        """
        try:
            results = self.code_snippets_collection.get(
                where={"codebase_id": codebase_id}
            )
            
            if not results['ids']:
                logger.info(f"No documents found for codebase {codebase_id}")
                return 0
            
            self.code_snippets_collection.delete(
                ids=results['ids']
            )
            
            count = len(results['ids'])
            logger.info(f"Deleted {count} documents for codebase {codebase_id}")
            return count
            
        except Exception as e:
            logger.error(f"Error deleting codebase {codebase_id}: {e}")
            raise

    async def get_collection_stats(self) -> Dict[str, int]:
        """
        Get statistics for all collections
        
        Returns:
            Dictionary with collection names and their document counts
        """
        try:
            return {
                'code_snippets': self.code_snippets_collection.count(),
                'evaluations': self.evaluations_collection.count(),
                'papers': self.papers_collection.count()
            }
        except Exception as e:
            logger.error(f"Error getting collection stats: {e}")
            raise

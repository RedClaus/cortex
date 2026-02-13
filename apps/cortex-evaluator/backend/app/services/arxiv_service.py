"""
ArXiv Service - Search and retrieve papers from arXiv API
"""
import logging
from typing import Optional, Any
from defusedxml import ElementTree as ET
from xml.etree.ElementTree import Element
import httpx
import PyPDF2
from io import BytesIO
from tenacity import retry, stop_after_attempt, wait_exponential, retry_if_exception_type


logger = logging.getLogger(__name__)


class ArxivService:
    """Service for interacting with the arXiv API to search and retrieve papers.

    Note: arXiv API requires rate limiting - maximum 1 request per 3 seconds.
    Implement rate limiting at the API route layer when calling this service.
    """

    def __init__(self):
        self.base_url = "http://export.arxiv.org/api/query"
        self.atom_namespace = "{http://www.w3.org/2005/Atom}"
        self.client = httpx.AsyncClient(timeout=30.0)

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.HTTPError, httpx.TimeoutException)),
        reraise=True
    )
    async def search_papers(self, query: str, max_results: int = 10) -> list[dict]:
        """
        Search arXiv API by topic/author/title

        Args:
            query: Search query string (e.g., "machine learning", "au:Smith")
            max_results: Maximum number of results to return (default: 10)

        Returns:
            List of paper dictionaries with metadata
        """
        try:
            params = {
                "search_query": query,
                "start": 0,
                "max_results": max_results,
            }

            response = await self.client.get(self.base_url, params=params)
            response.raise_for_status()

            xml_content = response.text
            papers = self._parse_xml_response(xml_content)

            logger.info(f"Found {len(papers)} papers for query: {query}")
            return papers

        except httpx.HTTPError as e:
            logger.error(f"HTTP error fetching arXiv papers: {e}")
            raise
        except Exception as e:
            logger.error(f"Error searching arXiv papers: {e}")
            raise

    async def get_paper(self, paper_id: str) -> dict:
        """
        Fetch paper metadata by ID and extract text from PDF

        Args:
            paper_id: arXiv paper ID (e.g., "2306.04338")

        Returns:
            Complete paper dictionary with extracted content
        """
        try:
            query = f"id:{paper_id}"
            papers = await self.search_papers(query, max_results=1)

            if not papers:
                raise ValueError(f"Paper not found: {paper_id}")

            paper = papers[0]

            if "pdf_url" in paper and paper["pdf_url"]:
                paper["content"] = await self._extract_pdf_text(paper["pdf_url"])
            else:
                paper["content"] = ""
                logger.warning(f"No PDF URL found for paper: {paper_id}")

            return paper

        except Exception as e:
            logger.error(f"Error getting paper {paper_id}: {e}")
            raise

    def _parse_xml_response(self, xml: str) -> list[dict]:
        """
        Parse arXiv XML response and extract paper metadata

        Args:
            xml: XML string from arXiv API

        Returns:
            List of structured paper dictionaries
        """
        try:
            root = ET.fromstring(xml)
            papers = []

            for entry in root.findall(self.atom_namespace + "entry"):
                paper = {
                    "id": self._extract_paper_id(entry),
                    "title": self._get_element_text(entry, "title"),
                    "authors": self._extract_authors(entry),
                    "summary": self._get_element_text(entry, "summary"),
                    "published": self._get_element_text(entry, "published"),
                    "categories": self._extract_categories(entry),
                    "pdf_url": self._extract_pdf_url(entry),
                }
                papers.append(paper)

            return papers

        except ET.ParseError as e:
            logger.error(f"Error parsing XML response: {e}")
            raise

    @retry(
        stop=stop_after_attempt(3),
        wait=wait_exponential(multiplier=1, min=2, max=10),
        retry=retry_if_exception_type((httpx.HTTPError, httpx.TimeoutException)),
        reraise=True
    )
    async def _extract_pdf_text(self, pdf_url: str) -> str:
        """
        Download PDF and extract text using PyPDF2

        Args:
            pdf_url: URL to the PDF file

        Returns:
            Extracted text from PDF (limited to 50000 characters)
        """
        try:
            response = await self.client.get(pdf_url)
            response.raise_for_status()

            pdf_bytes = BytesIO(response.content)
            pdf_reader = PyPDF2.PdfReader(pdf_bytes)

            text_parts = []
            for page in pdf_reader.pages:
                try:
                    page_text = page.extract_text()
                    if page_text:
                        text_parts.append(page_text)
                except Exception as e:
                    logger.warning(f"Error extracting text from page: {e}")
                    continue

            full_text = "\n\n".join(text_parts)

            if len(full_text) > 50000:
                full_text = full_text[:50000]
                logger.info(f"Truncated PDF text to 50000 characters")

            return full_text

        except httpx.HTTPError as e:
            logger.error(f"HTTP error downloading PDF: {e}")
            raise
        except PyPDF2.PdfReadError as e:
            logger.error(f"Error reading PDF: {e}")
            raise
        except Exception as e:
            logger.error(f"Error extracting PDF text: {e}")
            raise

    def _get_element_text(self, entry: Element, element_name: str) -> str:
        """
        Get text content from an XML element

        Args:
            entry: Parent XML element
            element_name: Name of child element

        Returns:
            Element text or empty string
        """
        elem = entry.find(self.atom_namespace + element_name)
        if elem is not None and elem.text:
            return elem.text.strip()
        return ""

    def _extract_paper_id(self, entry: Element) -> str:
        """
        Extract paper ID from the entry

        Args:
            entry: XML entry element

        Returns:
            Paper ID string
        """
        id_elem = entry.find(self.atom_namespace + "id")
        if id_elem is not None and id_elem.text:
            id_text = id_elem.text.strip()
            if id_text:
                parts = id_text.split("/")
                return parts[-1]
        return ""

    def _extract_authors(self, entry: Element) -> list[str]:
        """
        Extract list of author names from entry

        Args:
            entry: XML entry element

        Returns:
            List of author names
        """
        authors = []
        for author in entry.findall(self.atom_namespace + "author"):
            name_elem = author.find(self.atom_namespace + "name")
            if name_elem is not None and name_elem.text:
                authors.append(name_elem.text.strip())
        return authors

    def _extract_categories(self, entry: Element) -> list[str]:
        """
        Extract category terms from entry

        Args:
            entry: XML entry element

        Returns:
            List of category terms
        """
        categories = []
        for category in entry.findall(self.atom_namespace + "category"):
            term = category.get("term")
            if term:
                categories.append(term)
        return categories

    def _extract_pdf_url(self, entry: Element) -> Optional[str]:
        """
        Extract PDF URL from entry links

        Args:
            entry: XML entry element

        Returns:
            PDF URL string or None
        """
        for link in entry.findall(self.atom_namespace + "link"):
            link_type = link.get("type")
            link_title = link.get("title", "").lower()
            href = link.get("href")

            if href:
                if link_type == "application/pdf" or "pdf" in link_title:
                    return href
                elif "pdf" in href:
                    return href

        return None

    async def close(self):
        """Close the HTTP client"""
        await self.client.aclose()

    async def __aenter__(self):
        """Async context manager entry"""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit"""
        await self.close()

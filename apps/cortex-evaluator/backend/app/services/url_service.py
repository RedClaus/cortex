"""
URL Content Extraction Service - Fetch and extract clean content from web pages
"""
import logging
from datetime import datetime
from typing import Optional
from urllib.parse import urlparse
import httpx
from bs4 import BeautifulSoup
from readability import Document


logger = logging.getLogger(__name__)


class URLService:
    """Service for fetching and extracting content from web pages."""

    def __init__(self):
        self.client = httpx.AsyncClient(timeout=30.0, follow_redirects=True)

    async def is_valid_url(self, url: str) -> bool:
        """
        Validate URL format using urlparse

        Args:
            url: URL string to validate

        Returns:
            True if URL has valid http/https scheme and netloc, False otherwise
        """
        try:
            parsed = urlparse(url)
            return parsed.scheme in ('http', 'https') and bool(parsed.netloc)
        except Exception as e:
            logger.warning(f"URL validation failed for {url}: {e}")
            return False

    async def extract_content(self, url: str) -> dict:
        """
        Fetch URL and extract clean, readable content

        Args:
            url: URL to fetch and extract content from

        Returns:
            Dictionary with url, title, content, type, and extracted_at

        Raises:
            ValueError: If URL is invalid
            httpx.HTTPError: If fetching the URL fails
        """
        if not await self.is_valid_url(url):
            raise ValueError(f"Invalid URL: {url}")

        try:
            response = await self.client.get(url)
            response.raise_for_status()

            html_content = response.text

            title = ""
            content = ""

            try:
                doc = Document(html_content)
                title = doc.title() or ""
                summary_html = doc.summary()

                soup = BeautifulSoup(summary_html, 'html.parser')
                content = soup.get_text(separator='\n', strip=True)

            except Exception as e:
                logger.warning(f"Readability extraction failed for {url}: {e}, falling back to BeautifulSoup")
                soup = BeautifulSoup(html_content, 'html.parser')

                title_elem = soup.find('title')
                title = title_elem.get_text(strip=True) if title_elem else ""

                for elem in soup(['script', 'style', 'nav', 'header', 'footer', 'aside']):
                    elem.decompose()

                body = soup.find('body')
                content = body.get_text(separator='\n', strip=True) if body else soup.get_text(separator='\n', strip=True)

            if len(content) > 50000:
                content = content[:50000]
                logger.info(f"Truncated content to 50000 characters for {url}")

            return {
                "url": url,
                "title": title,
                "content": content,
                "type": "url",
                "extracted_at": datetime.utcnow().isoformat()
            }

        except httpx.HTTPError as e:
            logger.error(f"HTTP error fetching URL {url}: {e}")
            raise
        except UnicodeDecodeError as e:
            logger.error(f"Encoding error for URL {url}: {e}")
            raise
        except Exception as e:
            logger.error(f"Error extracting content from {url}: {e}")
            raise

    async def get_metadata(self, url: str) -> dict:
        """
        Extract page metadata from URL

        Args:
            url: URL to extract metadata from

        Returns:
            Dictionary with description, keywords, og:title, og:description, and favicon

        Raises:
            ValueError: If URL is invalid
            httpx.HTTPError: If fetching the URL fails
        """
        if not await self.is_valid_url(url):
            raise ValueError(f"Invalid URL: {url}")

        try:
            response = await self.client.get(url)
            response.raise_for_status()

            html_content = response.text
            soup = BeautifulSoup(html_content, 'html.parser')

            metadata = {
                "description": "",
                "keywords": "",
                "og:title": "",
                "og:description": "",
                "favicon": ""
            }

            meta_desc = soup.find('meta', attrs={'name': 'description'})
            if meta_desc and meta_desc.get('content'):
                metadata['description'] = meta_desc['content']

            meta_keywords = soup.find('meta', attrs={'name': 'keywords'})
            if meta_keywords and meta_keywords.get('content'):
                metadata['keywords'] = meta_keywords['content']

            og_title = soup.find('meta', property='og:title')
            if og_title and og_title.get('content'):
                metadata['og:title'] = og_title['content']

            og_desc = soup.find('meta', property='og:description')
            if og_desc and og_desc.get('content'):
                metadata['og:description'] = og_desc['content']

            favicon_link = soup.find('link', rel='icon')
            if not favicon_link:
                favicon_link = soup.find('link', rel='shortcut icon')
            if favicon_link and favicon_link.get('href'):
                favicon_url = favicon_link['href']
                if favicon_url.startswith('//'):
                    parsed_url = urlparse(url)
                    favicon_url = f"{parsed_url.scheme}:{favicon_url}"
                elif favicon_url.startswith('/'):
                    parsed_url = urlparse(url)
                    favicon_url = f"{parsed_url.scheme}://{parsed_url.netloc}{favicon_url}"
                metadata['favicon'] = favicon_url

            return metadata

        except httpx.HTTPError as e:
            logger.error(f"HTTP error fetching metadata from {url}: {e}")
            raise
        except Exception as e:
            logger.error(f"Error extracting metadata from {url}: {e}")
            raise

    async def close(self):
        """Close the HTTP client"""
        await self.client.aclose()

    async def __aenter__(self):
        """Async context manager entry"""
        return self

    async def __aexit__(self, exc_type, exc_val, exc_tb):
        """Async context manager exit"""
        await self.close()

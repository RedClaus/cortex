"""
Tests for arXiv Service - paper search, retrieval, and PDF extraction
"""
import pytest
from unittest.mock import MagicMock, patch, AsyncMock, Mock
from io import BytesIO
from app.services.arxiv_service import ArxivService


@pytest.mark.asyncio
class TestArxivService:
    """Test arXiv service initialization and API interactions"""

    @pytest.fixture
    def service(self):
        return ArxivService()

    @pytest.fixture
    def sample_paper_entry(self):
        return {
            'id': 'http://arxiv.org/abs/2301.12345',
            'title': 'Test Paper on AI Evaluation',
            'authors': [
                {'name': 'John Doe'},
                {'name': 'Jane Smith'}
            ],
            'summary': 'This is a test paper about AI evaluation methods.',
            'published': '2023-01-15T00:00:00Z',
            'categories': [
                {'term': 'cs.AI'},
                {'term': 'cs.LG'}
            ],
            'links': [
                {
                    'href': 'http://arxiv.org/pdf/2301.12345.pdf',
                    'type': 'application/pdf',
                    'title': 'pdf'
                }
            ]
        }

    async def test_arxiv_service_initialization(self, service):
        """Service should initialize with correct base URL and client"""
        assert service.base_url == "http://export.arxiv.org/api/query"
        assert service.client is not None

    async def test_search_papers_success(self, service, mock_arxiv_response_xml):
        """Should parse XML and return list of papers"""
        with patch.object(service.client, 'get', new_callable=AsyncMock) as mock_get:
            mock_get.return_value.status_code = 200
            mock_get.return_value.text = mock_arxiv_response_xml

            papers = await service.search_papers("machine learning", max_results=2)

            assert len(papers) == 2
            assert papers[0]['id'] == '2301.12345'
            assert papers[0]['title'] == 'Test Paper on AI Evaluation'
            assert len(papers[0]['authors']) == 2
            assert papers[0]['authors'][0] == 'John Doe'
            assert 'cs.AI' in papers[0]['categories']

    async def test_search_papers_with_max_results(self, service, mock_arxiv_response_xml):
        """Should respect max_results parameter"""
        with patch.object(service.client, 'get', new_callable=AsyncMock) as mock_get:
            mock_get.return_value.status_code = 200
            mock_get.return_value.text = mock_arxiv_response_xml

            papers = await service.search_papers("AI", max_results=1)

            assert len(papers) == 1

    async def test_search_papers_http_error(self, service):
        """Should raise HTTPError on network failure"""
        from httpx import HTTPError

        with patch.object(service.client, 'get', new_callable=AsyncMock) as mock_get:
            mock_get.side_effect = HTTPError("Connection failed")

            with pytest.raises(HTTPError):
                await service.search_papers("test query")

    async def test_search_papers_xml_parse_error(self, service):
        """Should raise exception on invalid XML"""
        with patch.object(service.client, 'get', new_callable=AsyncMock) as mock_get:
            mock_get.return_value.status_code = 200
            mock_get.return_value.text = "invalid xml"

            from defusedxml import ElementTree as ET
            with pytest.raises(ET.ParseError):
                await service.search_papers("test")

    async def test_get_paper_success(self, service, mock_arxiv_response_xml):
        """Should fetch paper metadata and extract PDF content"""
        from unittest.mock import MagicMock

        mock_pdf_content = b"%PDF-1.4\n1 0 obj<</Type<</Count 0 startxref>>stream\nendstream\nendobj\n%%EOF"

        with patch.object(service.client, 'get', new_callable=AsyncMock) as mock_get:
            # Mock search to return paper
            mock_get.return_value.status_code = 200
            mock_get.return_value.text = mock_arxiv_response_xml

            # Mock PDF download
            with patch.object(service, '_extract_pdf_text', new_callable=AsyncMock(return_value="Extracted PDF text")):
                paper = await service.get_paper("2301.12345")

                assert paper['id'] == '2301.12345'
                assert paper['title'] == 'Test Paper on AI Evaluation'
                assert paper['content'] == 'Extracted PDF text'

    async def test_get_paper_not_found(self, service):
        """Should raise ValueError when paper not found"""
        with patch.object(service.client, 'get', new_callable=AsyncMock) as mock_get:
            mock_get.return_value.status_code = 200
            mock_get.return_value.text = '<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>'

            with pytest.raises(ValueError, match="Paper not found"):
                await service.get_paper("nonexistent")

    async def test_parse_xml_response(self, service, mock_arxiv_response_xml):
        """Should correctly parse all paper fields from XML"""
        papers = service._parse_xml_response(mock_arxiv_response_xml)

        assert len(papers) == 2
        paper1, paper2 = papers

        assert paper1['id'] == '2301.12345'
        assert paper1['title'] == 'Test Paper on AI Evaluation'
        assert paper1['authors'] == ['John Doe', 'Jane Smith']
        assert 'cs.AI' in paper1['categories']
        assert 'cs.LG' in paper1['categories']
        assert paper1['summary'] == 'This is a test paper about AI evaluation methods.'
        assert paper1['published'] == '2023-01-15T00:00:00Z'
        assert paper1['pdf_url'] == 'http://arxiv.org/pdf/2301.12345.pdf'

        assert paper2['id'] == '2302.54321'
        assert paper2['title'] == 'Another Test Paper'

    def test_extract_paper_id(self, service, sample_paper_entry):
        """Should extract paper ID from arXiv URL"""
        from defusedxml import ElementTree as ET

        root = ET.fromstring('<entry xmlns="http://www.w3.org/2005/Atom">{}</entry>'.format(
            ''.join(f'<{k}>{v}</{k}>' for k, v in {
                'id': sample_paper_entry['id'],
                'title': sample_paper_entry['title']
            }.items())
        ))

        paper_id = service._extract_paper_id(root)

        assert paper_id == '2301.12345'

    def test_extract_authors(self, service, sample_paper_entry):
        """Should extract list of author names"""
        from defusedxml import ElementTree as ET

        xml_authors = ''.join(
            f'<author xmlns="http://www.w3.org/2005/Atom"><name>{a["name"]}</name></author>'
            for a in sample_paper_entry['authors']
        )

        root = ET.fromstring('<entry xmlns="http://www.w3.org/2005/Atom">{}</entry>'.format(xml_authors))
        authors = service._extract_authors(root)

        assert authors == ['John Doe', 'Jane Smith']

    def test_extract_categories(self, service, sample_paper_entry):
        """Should extract category terms"""
        from defusedxml import ElementTree as ET

        xml_categories = ''.join(
            f'<category xmlns="http://www.w3.org/2005/Atom" term="{c["term"]}" />'
            for c in sample_paper_entry['categories']
        )

        root = ET.fromstring('<entry xmlns="http://www.w3.org/2005/Atom">{}</entry>'.format(xml_categories))
        categories = service._extract_categories(root)

        assert categories == ['cs.AI', 'cs.LG']

    def test_extract_pdf_url(self, service, sample_paper_entry):
        """Should extract PDF URL from link with type=application/pdf"""
        from defusedxml import ElementTree as ET

        xml_links = ''.join(
            f'<link xmlns="http://www.w3.org/2005/Atom" href="{l["href"]}" type="{l["type"]}" title="{l["title"]}" />'
            for l in sample_paper_entry['links']
        )

        root = ET.fromstring('<entry xmlns="http://www.w3.org/2005/Atom">{}</entry>'.format(xml_links))
        pdf_url = service._extract_pdf_url(root)

        assert pdf_url == 'http://arxiv.org/pdf/2301.12345.pdf'

    def test_get_element_text(self, service):
        """Should extract text content or return empty string"""
        from defusedxml import ElementTree as ET

        root = ET.fromstring('<element xmlns="http://www.w3.org/2005/Atom">Text content</element>')
        assert service._get_element_text(root, 'element') == 'Text content'

        root_empty = ET.fromstring('<element xmlns="http://www.w3.org/2005/Atom"></element>')
        assert service._get_element_text(root_empty, 'element') == ''

    @pytest.mark.asyncio
    async def test_extract_pdf_text_success(self, service):
        """Should extract text from PDF pages"""
        from unittest.mock import MagicMock, mock_open

        mock_pdf_content = b"%PDF-1.4\n1 0 obj<</Type<</Count 1 startxref>>stream\nBT /F1 12 Tf(Test Page 1) ET endstream\nendobj\nstartxref\n%%EOF"
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = mock_pdf_content

        with patch.object(service.client, 'get', new_callable=AsyncMock(return_value=mock_response)):
            from unittest.mock import patch
            with patch('app.services.arxiv_service.BytesIO', return_value=BytesIO(mock_pdf_content)):
                from unittest.mock import patch as patch2
                with patch2('app.services.arxiv_service.PyPDF2.PdfReader'):
                    text = await service._extract_pdf_text("http://test.pdf")

                    assert 'Test Page 1' in text

    @pytest.mark.asyncio
    async def test_extract_pdf_text_truncates_long_content(self, service):
        """Should truncate text to 50000 characters"""
        long_text = 'x' * 60000
        mock_pdf_content = f"%PDF-1.4\n1 0 obj<</Type<</Count 1 startxref>>stream\nBT /F1 12 Tf({long_text}) ET endstream\nendobj\n%%EOF".encode()

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = mock_pdf_content

        with patch.object(service.client, 'get', new_callable=AsyncMock(return_value=mock_response)):
            from unittest.mock import patch
            with patch('app.services.arxiv_service.BytesIO', return_value=BytesIO(mock_pdf_content)):
                from unittest.mock import patch as patch2
                with patch2('app.services.arxiv_service.PyPDF2.PdfReader'):
                    text = await service._extract_pdf_text("http://test.pdf")

                    assert len(text) == 50000
                    assert text == 'x' * 50000

    @pytest.mark.asyncio
    async def test_extract_pdf_text_handles_page_errors(self, service):
        """Should continue on page extraction errors"""
        mock_pdf_content = b"%PDF-1.4\n1 0 obj<</Type<</Count 1 startxref>>stream\nBT /F1 12 Tf(Page 1) ET endstream\nendobj\nstartxref\n%%EOF"

        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = mock_pdf_content

        with patch.object(service.client, 'get', new_callable=AsyncMock(return_value=mock_response)):
            from unittest.mock import patch
            with patch('app.services.arxiv_service.BytesIO', return_value=BytesIO(mock_pdf_content)):
                from unittest.mock import patch as patch2
                with patch2('app.services.arxiv_service.PyPDF2.PdfReader') as mock_reader:
                    # First page succeeds, second fails
                    mock_reader.return_value.pages = [
                        MagicMock(extract_text=MagicMock(return_value='Page 1')),
                        MagicMock(extract_text=MagicMock(side_effect=Exception('Parse error')))
                    ]

                    text = await service._extract_pdf_text("http://test.pdf")

                    # Should include text from successful page
                    assert 'Page 1' in text

    @pytest.mark.asyncio
    async def test_close_client(self, service):
        """Should properly close HTTP client"""
        with patch.object(service.client, 'aclose', new_callable=AsyncMock) as mock_close:
            await service.close()
            mock_close.assert_called_once()

    async def test_context_manager(self, service):
        """Should work as async context manager"""
        with patch.object(service, 'close', new_callable=AsyncMock) as mock_close:
            async with service:
                pass

            mock_close.assert_called_once()

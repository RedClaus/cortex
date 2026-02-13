"""Services package for Cortex Evaluator backend"""

try:
    from .arxiv_service import ArxivService
    _has_arxiv = True
except ImportError:
    _has_arxiv = False
    ArxivService = None

try:
    from .circuit_breaker import CircuitBreaker, CircuitBreakerConfig, CircuitState
except ImportError:
    CircuitBreaker = CircuitBreakerConfig = CircuitState = None

try:
    from .provider_configs import load_provider_configs, get_provider_config
except ImportError:
    load_provider_configs = get_provider_config = None

try:
    from .ai_router import (
        CortexRouter,
        RoutingLane,
        GeminiProvider,
        ClaudeProvider,
        OpenAIProvider,
        OllamaProvider,
        GroqProvider,
    )
except ImportError:
    CortexRouter = None
    RoutingLane = None
    GeminiProvider = ClaudeProvider = OpenAIProvider = OllamaProvider = GroqProvider = None

try:
    from .github_integration import GitHubIntegration, GitHubIntegrationError, GitHubRateLimitError
except ImportError:
    GitHubIntegration = None
    GitHubIntegrationError = None
    GitHubRateLimitError = None

__all__ = [
    "ArxivService" if _has_arxiv else None,
    "CircuitBreaker",
    "CircuitBreakerConfig",
    "CircuitState",
    "load_provider_configs",
    "get_provider_config",
    "CortexRouter",
    "RoutingLane",
    "GeminiProvider",
    "ClaudeProvider",
    "OpenAIProvider",
    "OllamaProvider",
    "GroqProvider",
    "GitHubIntegration",
    "GitHubIntegrationError",
    "GitHubRateLimitError",
]

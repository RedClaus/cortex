"""
Tests for AI Router - routing logic, fallback chains, and circuit breaker
"""
import pytest
from unittest.mock import AsyncMock, MagicMock, patch
from app.services.ai_router import (
    CortexRouter,
    RoutingLane,
    RoutingDecision,
    GeminiProvider,
    ClaudeProvider,
    OpenAIProvider,
    OllamaProvider,
    GroqProvider
)
from app.services.circuit_breaker import CircuitState, CircuitBreakerError


class TestRoutingLane:
    """Test routing lane enumeration and behavior"""

    def test_routing_lane_values(self):
        """Routing lanes should have correct enum values"""
        assert RoutingLane.FAST.value == "fast"
        assert RoutingLane.SMART.value == "smart"


class TestRoutingDecision:
    """Test routing decision dataclass"""

    def test_routing_decision_creation(self):
        """RoutingDecision should be created with all fields"""
        provider = MagicMock(name="test_provider")
        decision = RoutingDecision(
            provider=provider,
            lane=RoutingLane.FAST,
            reason="Test reason"
        )
        assert decision.provider == provider
        assert decision.lane == RoutingLane.FAST
        assert decision.reason == "Test reason"


class TestCortexRouter:
    """Test CortexRouter routing logic and fallback chains"""

    @pytest.fixture
    def router(self, mock_provider_configs):
        """Create router with mock provider configs"""
        with patch('app.services.ai_router.load_provider_configs', return_value=mock_provider_configs):
            return CortexRouter()

    @pytest.fixture
    def mock_analyze_result(self):
        """Mock successful analyze result"""
        return {
            "success": True,
            "result": "Analysis complete",
            "provider": "gemini",
            "model": "gemini-1.5-pro"
        }

    def test_route_analysis_hard_constraints_large_context(self, router):
        """Large context (>100K tokens) should route to SMART lane"""
        large_codebase = {
            f"file_{i}.py": "x" * 10000
            for i in range(11)
        }
        input_data = {"query": "test"}
        decision = router.route_analysis(
            codebase=large_codebase,
            input_data=input_data,
            system_doc=None,
            user_intent=None
        )

        assert decision.lane == RoutingLane.SMART
        assert "context" in decision.reason.lower()

    def test_route_analysis_hard_constraints_vision(self, router):
        """Vision input should route to SMART lane"""
        input_data = {"has_vision": True, "query": "test"}
        decision = router.route_analysis(
            codebase={},
            input_data=input_data,
            system_doc=None,
            user_intent=None
        )

        assert decision.lane == RoutingLane.SMART
        assert "vision" in decision.reason.lower()

    def test_route_analysis_user_intent_strong(self, router):
        """--strong intent should route to SMART lane"""
        decision = router.route_analysis(
            codebase={},
            input_data={"query": "test"},
            system_doc=None,
            user_intent="--strong"
        )

        assert decision.lane == RoutingLane.SMART
        assert "strong" in decision.reason.lower()

    def test_route_analysis_user_intent_local(self, router):
        """--local intent should route to Ollama (FAST lane)"""
        decision = router.route_analysis(
            codebase={},
            input_data={"query": "test"},
            system_doc=None,
            user_intent="--local"
        )

        assert decision.lane == RoutingLane.FAST
        assert decision.provider.name == "ollama"
        assert "local" in decision.reason.lower()

    def test_route_analysis_user_intent_cheap(self, router):
        """--cheap intent should route to FAST lane"""
        decision = router.route_analysis(
            codebase={},
            input_data={"query": "test"},
            system_doc=None,
            user_intent="--cheap"
        )

        assert decision.lane == RoutingLane.FAST
        assert "cheap" in decision.reason.lower()

    def test_route_analysis_default(self, router):
        """No constraints should default to FAST lane"""
        decision = router.route_analysis(
            codebase={},
            input_data={"query": "test"},
            system_doc=None,
            user_intent=None
        )

        assert decision.lane == RoutingLane.FAST
        assert "default" in decision.reason.lower()

    @pytest.mark.asyncio
    async def test_analyze_with_fallback_success_primary(self, router, mock_analyze_result):
        """Successful analysis should return result from primary provider"""
        codebase = {"test.py": "print('hello')"}
        input_data = {"query": "analyze"}

        # Mock provider to return success
        with patch.object(router.fast_lane[0], 'analyze_code', new=AsyncMock(return_value=mock_analyze_result)):
            result = await router.analyze_with_fallback(
                codebase=codebase,
                input_data=input_data,
                system_doc=None,
                user_intent=None
            )

            assert result["success"] == True
            assert "result" in result

    @pytest.mark.asyncio
    async def test_analyze_with_fallback_primary_invalid(self, router):
        """Invalid result from primary should try secondary provider"""
        codebase = {"test.py": "print('hello')"}
        input_data = {"query": "analyze"}

        # Mock primary to return invalid, secondary to return valid
        invalid_result = {"invalid": "data"}
        valid_result = {
            "success": True,
            "result": "Analysis complete",
            "provider": "groq",
            "model": "llama3-70b-8192"
        }

        with patch.object(router.fast_lane[0], 'analyze_code', new=AsyncMock(return_value=invalid_result)):
            with patch.object(router.fast_lane[1], 'analyze_code', new=AsyncMock(return_value=valid_result)):
                result = await router.analyze_with_fallback(
                    codebase=codebase,
                    input_data=input_data,
                    system_doc=None,
                    user_intent=None
                )

                assert result["success"] == True
                assert result["provider"] == "groq"

    @pytest.mark.asyncio
    async def test_analyze_with_fallback_all_fail(self, router):
        """All providers failing should raise exception"""
        codebase = {"test.py": "print('hello')"}
        input_data = {"query": "analyze"}

        with patch.object(router.fast_lane[0], 'analyze_code', new=AsyncMock(side_effect=Exception("Failed"))):
            with patch.object(router.fast_lane[1], 'analyze_code', new=AsyncMock(side_effect=Exception("Failed"))):
                with patch.object(router.fast_lane[2], 'analyze_code', new=AsyncMock(side_effect=Exception("Failed"))):
                    with pytest.raises(Exception, match="All AI providers failed"):
                        await router.analyze_with_fallback(
                            codebase=codebase,
                            input_data=input_data,
                            system_doc=None,
                            user_intent=None
                        )

    def test_validate_result_valid(self, router):
        """Valid result with required fields should pass validation"""
        result = {
            "success": True,
            "result": "test"
        }
        assert router._validate_result(result) == True

    def test_validate_result_missing_fields(self, router):
        """Result missing required fields should fail validation"""
        result = {"success": True}
        assert router._validate_result(result) == False

    def test_validate_result_success_false(self, router):
        """Result with success=False should fail validation"""
        result = {
            "success": False,
            "result": "test"
        }
        assert router._validate_result(result) == False

    def test_get_provider_status(self, router):
        """Provider status should return correct structure"""
        status = router.get_provider_status()

        assert isinstance(status, dict)
        assert "gemini" in status
        assert "claude" in status
        assert "openai" in status
        assert "ollama" in status
        assert "groq" in status

        for provider_name, provider_status in status.items():
            assert "lane" in provider_status
            assert "circuit_state" in provider_status
            assert "failure_rate" in provider_status
            assert provider_status["lane"] in ["fast", "smart"]
            assert provider_status["circuit_state"] in ["closed", "open", "half_open"]
            assert isinstance(provider_status["failure_rate"], float)
            assert 0 <= provider_status["failure_rate"] <= 1


class TestCircuitBreaker:
    """Test circuit breaker state transitions and behavior"""

    def test_circuit_breaker_initial_state(self, mock_circuit_breaker):
        """Circuit breaker should start in CLOSED state"""
        assert mock_circuit_breaker.state == CircuitState.CLOSED
        assert mock_circuit_breaker.failure_count == 0
        assert mock_circuit_breaker.failure_rate == 0.0

    @pytest.mark.asyncio
    async def test_circuit_breaker_success(self, mock_circuit_breaker):
        """Successful call should maintain CLOSED state"""
        async def success_func():
            return "success"

        result = await mock_circuit_breaker.call(success_func)
        assert result == "success"
        assert mock_circuit_breaker.state == CircuitState.CLOSED

    @pytest.mark.asyncio
    async def test_circuit_breaker_failure_below_threshold(self, mock_circuit_breaker):
        """Failures below threshold should not open circuit"""
        config = CircuitBreakerConfig(failure_threshold=5, success_threshold=2)
        cb = CircuitBreaker("test", config)

        async def fail_func():
            raise Exception("Failed")

        # 2 failures (below threshold of 5)
        for _ in range(2):
            with pytest.raises(Exception):
                await cb.call(fail_func)

        assert cb.state == CircuitState.CLOSED
        assert cb.failure_count == 2

    @pytest.mark.asyncio
    async def test_circuit_breaker_failure_exceeds_threshold(self, mock_circuit_breaker):
        """Failures exceeding threshold should open circuit"""
        config = CircuitBreakerConfig(failure_threshold=3, success_threshold=2, timeout=1.0)
        cb = CircuitBreaker("test", config)

        async def fail_func():
            raise Exception("Failed")

        # 3 failures (meets threshold)
        for _ in range(3):
            with pytest.raises(Exception):
                await cb.call(fail_func)

        assert cb.state == CircuitState.OPEN
        assert cb.failure_count == 0

    @pytest.mark.asyncio
    async def test_circuit_breaker_open_blocks_calls(self, mock_circuit_breaker):
        """OPEN circuit should raise CircuitBreakerError"""
        # Force circuit to OPEN
        config = CircuitBreakerConfig(failure_threshold=2, success_threshold=1, timeout=5.0)
        cb = CircuitBreaker("test", config)

        async def fail_func():
            raise Exception("Failed")

        for _ in range(2):
            with pytest.raises(Exception):
                await cb.call(fail_func)

        # Now circuit should be OPEN
        async def any_func():
            return "result"

        with pytest.raises(CircuitBreakerError):
            await cb.call(any_func)

    @pytest.mark.asyncio
    async def test_circuit_breaker_half_open_recovers(self):
        """Successful calls in HALF_OPEN should close circuit"""
        config = CircuitBreakerConfig(failure_threshold=2, success_threshold=2, timeout=1.0)
        cb = CircuitBreaker("test", config)

        async def fail_func():
            raise Exception("Failed")

        # Open circuit
        for _ in range(2):
            with pytest.raises(Exception):
                await cb.call(fail_func)

        assert cb.state == CircuitState.OPEN

        # Wait for timeout to expire
        import time
        time.sleep(1.1)

        async def success_func():
            return "success"

        # Success should move to HALF_OPEN then back to CLOSED
        await cb.call(success_func)
        assert cb.state == CircuitState.HALF_OPEN

        await cb.call(success_func)
        assert cb.state == CircuitState.CLOSED

    def test_circuit_breaker_reset(self, mock_circuit_breaker):
        """Reset should return to CLOSED state"""
        # Simulate failures
        mock_circuit_breaker._failure_count = 5
        mock_circuit_breaker._state = CircuitState.OPEN

        mock_circuit_breaker.reset()

        assert mock_circuit_breaker.state == CircuitState.CLOSED
        assert mock_circuit_breaker.failure_count == 0
        assert mock_circuit_breaker.success_count == 0
        assert len(mock_circuit_breaker._call_history) == 0


class TestProviderConfigs:
    """Test provider configuration loading"""

    @patch('app.services.provider_configs.os.getenv')
    def test_load_provider_configs_returns_all_providers(self, mock_getenv):
        """Should load configs for all 5 providers"""
        mock_getenv.side_effect = lambda key, default: f"mock_{key}"

        from app.services.provider_configs import load_provider_configs

        configs = load_provider_configs()

        assert "gemini" in configs
        assert "claude" in configs
        assert "openai" in configs
        assert "ollama" in configs
        assert "groq" in configs

        for provider, config in configs.items():
            assert isinstance(config, dict)
            assert "model" in config

    @patch('app.services.provider_configs.os.getenv')
    def test_get_provider_config_by_name(self, mock_getenv):
        """Should return specific provider config"""
        mock_getenv.side_effect = lambda key, default: f"mock_{key}"

        from app.services.provider_configs import get_provider_config

        config = get_provider_config("claude")

        assert config["api_key"] == "mock_ANTHROPIC_API_KEY"
        assert config["model"] == "mock_ANTHROPIC_MODEL"

    @patch('app.services.provider_configs.os.getenv')
    def test_get_provider_config_invalid(self, mock_getenv):
        """Should raise ValueError for unknown provider"""
        mock_getenv.side_effect = lambda key, default: f"mock_{key}"

        from app.services.provider_configs import get_provider_config

        with pytest.raises(ValueError, match="Unknown provider"):
            get_provider_config("invalid_provider")

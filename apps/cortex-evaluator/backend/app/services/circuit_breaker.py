"""
Circuit Breaker Pattern Implementation

Provides resilience for external API calls by preventing cascading failures
and allowing time for services to recover.
"""
import asyncio
import time
from collections import deque
from dataclasses import dataclass, field
from enum import Enum
from typing import Callable, TypeVar, Optional

T = TypeVar("T")


class CircuitState(Enum):
    """Circuit breaker states."""
    CLOSED = "closed"      # Normal operation, requests flow through
    OPEN = "open"          # Blocking requests, provider is unhealthy
    HALF_OPEN = "half_open"  # Testing if provider recovered


@dataclass
class CircuitBreakerConfig:
    """Configuration for circuit breaker behavior.

    Attributes:
        failure_threshold: Number of consecutive failures before opening circuit
        success_threshold: Number of successes in half-open to close circuit
        timeout: Seconds to wait before transitioning from OPEN to HALF_OPEN
        window_size: Size of sliding window for tracking call history
    """
    failure_threshold: int = 5
    success_threshold: int = 2
    timeout: float = 60.0
    window_size: int = 100


class CircuitBreakerError(Exception):
    """Raised when circuit breaker is open and blocking requests."""
    def __init__(self, provider_name: str, retry_after: float):
        self.provider_name = provider_name
        self.retry_after = retry_after
        super().__init__(
            f"Circuit breaker OPEN for {provider_name}. "
            f"Retry after {retry_after:.1f}s"
        )


class CircuitBreaker:
    """Circuit breaker for protecting external API calls.

    Tracks failures and successes, opening the circuit when failures exceed threshold.
    Allows gradual recovery through HALF_OPEN state.
    """

    def __init__(self, provider_name: str, config: Optional[CircuitBreakerConfig] = None):
        """Initialize circuit breaker.

        Args:
            provider_name: Name of the provider being protected
            config: Circuit breaker configuration (uses defaults if None)
        """
        self.provider_name = provider_name
        self.config = config or CircuitBreakerConfig()

        self._state = CircuitState.CLOSED
        self._failure_count = 0
        self._success_count = 0
        self._opened_at: Optional[float] = None

        # Sliding window for tracking recent calls
        self._call_history: deque[bool] = deque(maxlen=self.config.window_size)

    async def call(self, func: Callable[..., T], *args, **kwargs) -> T:
        """Execute function with circuit breaker protection.

        Args:
            func: Async function to call
            *args: Positional arguments for func
            **kwargs: Keyword arguments for func

        Returns:
            Result from func

        Raises:
            CircuitBreakerError: If circuit is OPEN
            Exception: Propagates any exception from func
        """
        if self._state == CircuitState.OPEN:
            if self._should_attempt_reset():
                self._transition_to_half_open()
            else:
                retry_after = max(0, self.config.timeout - (time.time() - (self._opened_at or 0)))
                raise CircuitBreakerError(self.provider_name, retry_after)

        try:
            result = await func(*args, **kwargs)
            self._on_success()
            return result
        except Exception as e:
            self._on_failure()
            raise

    def _should_attempt_reset(self) -> bool:
        """Check if enough time has passed to attempt reset."""
        if self._opened_at is None:
            return False
        return time.time() - self._opened_at >= self.config.timeout

    def _on_success(self) -> None:
        """Handle successful call."""
        self._call_history.append(True)

        if self._state == CircuitState.HALF_OPEN:
            self._success_count += 1
            if self._success_count >= self.config.success_threshold:
                self._transition_to_closed()
        elif self._state == CircuitState.CLOSED:
            self._failure_count = 0

    def _on_failure(self) -> None:
        """Handle failed call."""
        self._call_history.append(False)

        if self._state == CircuitState.CLOSED:
            self._failure_count += 1
            if self._failure_count >= self.config.failure_threshold:
                self._transition_to_open()
        elif self._state == CircuitState.HALF_OPEN:
            self._transition_to_open()

    def _transition_to_open(self) -> None:
        """Transition to OPEN state."""
        self._state = CircuitState.OPEN
        self._opened_at = time.time()
        self._success_count = 0
        self._failure_count = 0

    def _transition_to_half_open(self) -> None:
        """Transition to HALF_OPEN state for testing."""
        self._state = CircuitState.HALF_OPEN
        self._success_count = 0

    def _transition_to_closed(self) -> None:
        """Transition to CLOSED state (fully recovered)."""
        self._state = CircuitState.CLOSED
        self._success_count = 0
        self._failure_count = 0
        self._opened_at = None

    def _trim_history(self) -> None:
        """Maintain sliding window size."""
        if len(self._call_history) > self.config.window_size:
            excess = len(self._call_history) - self.config.window_size
            for _ in range(excess):
                self._call_history.popleft()

    @property
    def state(self) -> CircuitState:
        """Current circuit state."""
        return self._state

    @property
    def failure_rate(self) -> float:
        """Calculate failure rate from history."""
        if not self._call_history:
            return 0.0
        failures = sum(1 for success in self._call_history if not success)
        return failures / len(self._call_history)

    def reset(self) -> None:
        """Manually reset circuit breaker to CLOSED state."""
        self._state = CircuitState.CLOSED
        self._failure_count = 0
        self._success_count = 0
        self._opened_at = None
        self._call_history.clear()

    def __repr__(self) -> str:
        return (
            f"CircuitBreaker(provider_name={self.provider_name!r}, "
            f"state={self._state.value}, "
            f"failure_count={self._failure_count}, "
            f"failure_rate={self.failure_rate:.2f})"
        )

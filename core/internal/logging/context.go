package logging

import (
	"context"
	"time"
)

// DetachContext creates a context that won't be cancelled when parent is.
// Uses Go 1.21+ context.WithoutCancel for clean implementation.
//
// This is critical for database logging operations that must complete even
// when the parent request context is cancelled due to timeout.
func DetachContext(parent context.Context) context.Context {
	return context.WithoutCancel(parent)
}

// DetachContextWithTimeout creates a detached context with its own timeout.
// This ensures logging operations have their own deadline independent of
// the parent context's cancellation status.
//
// Example usage:
//
//	logCtx, cancel := logging.DetachContextWithTimeout(ctx, 5*time.Second)
//	defer cancel()
//	err := logger.LogResponse(logCtx, requestID, response)
func DetachContextWithTimeout(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	detached := context.WithoutCancel(parent)
	return context.WithTimeout(detached, timeout)
}

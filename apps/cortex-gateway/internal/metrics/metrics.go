package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	RequestCount = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Name: "cortex_gateway_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"method", "endpoint", "status"},
	)

	RequestDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Name: "cortex_gateway_request_duration_seconds",
			Help: "HTTP request duration in seconds",
		},
		[]string{"method", "endpoint"},
	)

	InferenceLatency = promauto.NewHistogram(
		prometheus.HistogramOpts{
			Name: "cortex_gateway_inference_latency_seconds",
			Help: "Inference latency in seconds",
		},
	)

	ActiveSessions = promauto.NewGauge(
		prometheus.GaugeOpts{
			Name: "cortex_gateway_active_sessions",
			Help: "Number of active sessions",
		},
	)

	MemoryOperations = promauto.NewCounter(
		prometheus.CounterOpts{
			Name: "cortex_gateway_memory_operations_total",
			Help: "Total number of memory operations",
		},
	)
)

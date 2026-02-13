package performance

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"

	"github.com/normanking/cortexavatar/internal/audio"
	"github.com/normanking/cortexavatar/internal/stt"
	"github.com/normanking/cortexavatar/internal/tts"
	"github.com/normanking/cortexavatar/tests/testutil"
)

// BenchmarkConfig holds configuration for performance benchmarks
type BenchmarkConfig struct {
	Iterations      int
	ServiceURL      string
	UseMockService  bool
	AudioDurationMs int
}

// LatencyMetrics holds latency statistics
type LatencyMetrics struct {
	Min    time.Duration
	Max    time.Duration
	Mean   time.Duration
	Median time.Duration
	P95    time.Duration
	P99    time.Duration
}

// MemoryMetrics holds memory usage statistics
type MemoryMetrics struct {
	Baseline    uint64
	Peak        uint64
	Final       uint64
	AllocBytes  uint64
	TotalAllocs uint64
}

// PerformanceReport holds complete benchmark results
type PerformanceReport struct {
	Config         BenchmarkConfig
	VADLatency     LatencyMetrics
	STTLatency     LatencyMetrics
	TTSLatency     LatencyMetrics
	E2ELatency     LatencyMetrics
	Memory         MemoryMetrics
	SuccessRate    float64
	Duration       time.Duration
	IterationsRun  int
	IterationsFail int
}

// TestVoicePipelinePerformance runs comprehensive performance benchmark
func TestVoicePipelinePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping performance test in short mode")
	}

	config := BenchmarkConfig{
		Iterations:      100,
		UseMockService:  true, // Set to false to test against real HF service
		AudioDurationMs: 3000,  // 3 seconds of audio
	}

	report := runPerformanceBenchmark(t, config)
	printPerformanceReport(t, report)

	// Validate performance criteria
	validatePerformanceCriteria(t, report)
}

// runPerformanceBenchmark executes the performance test
func runPerformanceBenchmark(t *testing.T, config BenchmarkConfig) PerformanceReport {
	logger := zerolog.New(os.Stdout).With().Timestamp().Logger()
	ctx := context.Background()

	// Setup mock service if configured
	var mockService *httptest.Server
	if config.UseMockService {
		mockService = testutil.CreateMockHFService(t)
		defer mockService.Close()
		config.ServiceURL = mockService.URL
	}

	// Initialize clients
	vadClient := audio.NewHFVADClient(config.ServiceURL, logger)
	sttProvider := stt.NewHFWhisperProvider(&stt.HFWhisperConfig{
		ServiceURL: config.ServiceURL,
		Timeout:    30,
		Language:   "en",
	}, logger)
	ttsProvider := tts.NewHFMeloProvider(&tts.HFMeloConfig{
		ServiceURL:   config.ServiceURL,
		Timeout:      30,
		DefaultVoice: "EN",
		DefaultSpeed: 1.0,
	}, logger)

	// Collect baseline memory
	runtime.GC()
	var memStart runtime.MemStats
	runtime.ReadMemStats(&memStart)

	// Storage for latency measurements
	vadLatencies := make([]time.Duration, 0, config.Iterations)
	sttLatencies := make([]time.Duration, 0, config.Iterations)
	ttsLatencies := make([]time.Duration, 0, config.Iterations)
	e2eLatencies := make([]time.Duration, 0, config.Iterations)

	successCount := 0
	failCount := 0

	startTime := time.Now()

	// Run iterations
	for i := 0; i < config.Iterations; i++ {
		iterStart := time.Now()

		// Generate test audio
		audioData := testutil.GenerateTestAudio(t, time.Duration(config.AudioDurationMs)*time.Millisecond)

		// Step 1: VAD
		vadStart := time.Now()
		vadResult, err := vadClient.DetectSpeech(ctx, audioData)
		vadLatency := time.Since(vadStart)
		if err != nil {
			t.Logf("Iteration %d: VAD failed: %v", i, err)
			failCount++
			continue
		}
		vadLatencies = append(vadLatencies, vadLatency)

		// Step 2: STT
		sttStart := time.Now()
		transcribeReq := &stt.TranscribeRequest{
			Audio:      audioData,
			Format:     "wav",
			SampleRate: 16000,
			Channels:   1,
			Language:   "en",
		}
		transcribeResp, err := sttProvider.Transcribe(ctx, transcribeReq)
		sttLatency := time.Since(sttStart)
		if err != nil {
			t.Logf("Iteration %d: STT failed: %v", i, err)
			failCount++
			continue
		}
		sttLatencies = append(sttLatencies, sttLatency)

		// Step 3: TTS
		ttsStart := time.Now()
		synthesizeReq := &tts.SynthesizeRequest{
			Text:    transcribeResp.Text,
			VoiceID: "en",
			Speed:   1.0,
		}
		synthesizeResp, err := ttsProvider.Synthesize(ctx, synthesizeReq)
		ttsLatency := time.Since(ttsStart)
		if err != nil {
			t.Logf("Iteration %d: TTS failed: %v", i, err)
			failCount++
			continue
		}
		ttsLatencies = append(ttsLatencies, ttsLatency)

		// Calculate E2E latency
		e2eLatency := time.Since(iterStart)
		e2eLatencies = append(e2eLatencies, e2eLatency)

		successCount++

		// Progress logging every 10 iterations
		if (i+1)%10 == 0 {
			t.Logf("Progress: %d/%d iterations complete", i+1, config.Iterations)
		}

		// Validation
		require.NotNil(t, vadResult)
		require.NotEmpty(t, transcribeResp.Text)
		require.NotEmpty(t, synthesizeResp.Audio)
	}

	totalDuration := time.Since(startTime)

	// Collect final memory
	runtime.GC()
	var memEnd runtime.MemStats
	runtime.ReadMemStats(&memEnd)

	// Build report
	report := PerformanceReport{
		Config:         config,
		VADLatency:     calculateLatencyMetrics(vadLatencies),
		STTLatency:     calculateLatencyMetrics(sttLatencies),
		TTSLatency:     calculateLatencyMetrics(ttsLatencies),
		E2ELatency:     calculateLatencyMetrics(e2eLatencies),
		Memory: MemoryMetrics{
			Baseline:    memStart.Alloc,
			Peak:        memEnd.Alloc,
			Final:       memEnd.Alloc,
			AllocBytes:  memEnd.TotalAlloc - memStart.TotalAlloc,
			TotalAllocs: memEnd.Mallocs - memStart.Mallocs,
		},
		SuccessRate:    float64(successCount) / float64(config.Iterations) * 100,
		Duration:       totalDuration,
		IterationsRun:  successCount,
		IterationsFail: failCount,
	}

	return report
}

// calculateLatencyMetrics computes statistical metrics for latency data
func calculateLatencyMetrics(latencies []time.Duration) LatencyMetrics {
	if len(latencies) == 0 {
		return LatencyMetrics{}
	}

	// Sort for percentile calculation
	sorted := make([]time.Duration, len(latencies))
	copy(sorted, latencies)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })

	// Calculate metrics
	min := sorted[0]
	max := sorted[len(sorted)-1]
	median := sorted[len(sorted)/2]
	p95 := sorted[int(float64(len(sorted))*0.95)]
	p99 := sorted[int(float64(len(sorted))*0.99)]

	var sum time.Duration
	for _, lat := range latencies {
		sum += lat
	}
	mean := sum / time.Duration(len(latencies))

	return LatencyMetrics{
		Min:    min,
		Max:    max,
		Mean:   mean,
		Median: median,
		P95:    p95,
		P99:    p99,
	}
}

// printPerformanceReport prints a detailed performance report
func printPerformanceReport(t *testing.T, report PerformanceReport) {
	t.Log("\n========================================")
	t.Log("      VOICE PIPELINE PERFORMANCE REPORT")
	t.Log("========================================\n")

	t.Logf("Configuration:")
	t.Logf("  Iterations:        %d", report.Config.Iterations)
	t.Logf("  Mock Service:      %v", report.Config.UseMockService)
	t.Logf("  Audio Duration:    %dms\n", report.Config.AudioDurationMs)

	t.Logf("Execution Summary:")
	t.Logf("  Total Duration:    %v", report.Duration)
	t.Logf("  Success Rate:      %.2f%% (%d/%d)", report.SuccessRate, report.IterationsRun, report.Config.Iterations)
	t.Logf("  Failed:            %d\n", report.IterationsFail)

	printLatencyTable(t, "VAD", report.VADLatency)
	printLatencyTable(t, "STT", report.STTLatency)
	printLatencyTable(t, "TTS", report.TTSLatency)
	printLatencyTable(t, "E2E", report.E2ELatency)

	t.Logf("\nMemory Usage:")
	t.Logf("  Baseline:          %s", formatBytes(report.Memory.Baseline))
	t.Logf("  Peak:              %s", formatBytes(report.Memory.Peak))
	t.Logf("  Final:             %s", formatBytes(report.Memory.Final))
	t.Logf("  Total Allocated:   %s", formatBytes(report.Memory.AllocBytes))
	t.Logf("  Total Allocs:      %d", report.Memory.TotalAllocs)

	t.Log("\n========================================")
}

// printLatencyTable prints a formatted latency metrics table
func printLatencyTable(t *testing.T, name string, metrics LatencyMetrics) {
	t.Logf("\n%s Latency:", name)
	t.Logf("  Min:     %v", metrics.Min)
	t.Logf("  Mean:    %v", metrics.Mean)
	t.Logf("  Median:  %v", metrics.Median)
	t.Logf("  P95:     %v", metrics.P95)
	t.Logf("  P99:     %v", metrics.P99)
	t.Logf("  Max:     %v", metrics.Max)
}

// validatePerformanceCriteria checks if performance meets targets
func validatePerformanceCriteria(t *testing.T, report PerformanceReport) {
	t.Log("\n========================================")
	t.Log("      PERFORMANCE VALIDATION")
	t.Log("========================================\n")

	// Success rate: Should be > 95%
	if report.SuccessRate < 95.0 {
		t.Errorf("❌ Success rate %.2f%% below target (95%%)", report.SuccessRate)
	} else {
		t.Logf("✅ Success rate: %.2f%%", report.SuccessRate)
	}

	// E2E latency: P95 should be < 2s (or < 1s for mock)
	target := 2 * time.Second
	if report.Config.UseMockService {
		target = 1 * time.Second
	}
	if report.E2ELatency.P95 > target {
		t.Errorf("❌ E2E P95 latency %v exceeds target %v", report.E2ELatency.P95, target)
	} else {
		t.Logf("✅ E2E P95 latency: %v (target: %v)", report.E2ELatency.P95, target)
	}

	// Memory: Should not grow unbounded (< 50% increase)
	memGrowth := float64(report.Memory.Final-report.Memory.Baseline) / float64(report.Memory.Baseline) * 100
	if memGrowth > 50 {
		t.Errorf("❌ Memory growth %.2f%% exceeds 50%%", memGrowth)
	} else {
		t.Logf("✅ Memory growth: %.2f%%", memGrowth)
	}

	// STT latency: P95 should be < 500ms (for real service)
	if !report.Config.UseMockService && report.STTLatency.P95 > 500*time.Millisecond {
		t.Errorf("❌ STT P95 latency %v exceeds 500ms", report.STTLatency.P95)
	} else if !report.Config.UseMockService {
		t.Logf("✅ STT P95 latency: %v (target: 500ms)", report.STTLatency.P95)
	}

	// TTS latency: P95 should be < 700ms (for real service)
	if !report.Config.UseMockService && report.TTSLatency.P95 > 700*time.Millisecond {
		t.Errorf("❌ TTS P95 latency %v exceeds 700ms", report.TTSLatency.P95)
	} else if !report.Config.UseMockService {
		t.Logf("✅ TTS P95 latency: %v (target: 700ms)", report.TTSLatency.P95)
	}

	t.Log("\n========================================")
}

// formatBytes formats byte count as human-readable string
func formatBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

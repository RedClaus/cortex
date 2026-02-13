package voice

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ═══════════════════════════════════════════════════════════════════════════════
// MODE DETECTOR TESTS
// NFR-002: Mode detection overhead < 1ms
// ═══════════════════════════════════════════════════════════════════════════════

func TestNewModeDetector(t *testing.T) {
	d := NewModeDetector()
	require.NotNil(t, d)

	// Default state should be text mode
	assert.Equal(t, ModeText, d.CurrentMode())
	assert.False(t, d.IsVoiceMode())
	assert.True(t, d.IsTextMode())
	assert.False(t, d.STTActive())
	assert.False(t, d.TTSEnabled())
	assert.False(t, d.HasExplicitMode())
}

func TestOutputModeString(t *testing.T) {
	assert.Equal(t, "text", ModeText.String())
	assert.Equal(t, "voice", ModeVoice.String())
}

func TestSetSTTActive(t *testing.T) {
	d := NewModeDetector()

	// Initially not active
	assert.False(t, d.STTActive())

	// Set active
	d.SetSTTActive(true)
	assert.True(t, d.STTActive())

	// Set inactive
	d.SetSTTActive(false)
	assert.False(t, d.STTActive())
}

func TestSetTTSEnabled(t *testing.T) {
	d := NewModeDetector()

	// Initially not enabled
	assert.False(t, d.TTSEnabled())

	// Set enabled
	d.SetTTSEnabled(true)
	assert.True(t, d.TTSEnabled())

	// Set disabled
	d.SetTTSEnabled(false)
	assert.False(t, d.TTSEnabled())
}

func TestSetExplicitMode(t *testing.T) {
	d := NewModeDetector()

	// No explicit mode initially
	assert.False(t, d.HasExplicitMode())

	// Set explicit voice mode
	d.SetExplicitMode(ModeVoice)
	assert.True(t, d.HasExplicitMode())
	assert.Equal(t, ModeVoice, d.CurrentMode())

	// Set explicit text mode
	d.SetExplicitMode(ModeText)
	assert.True(t, d.HasExplicitMode())
	assert.Equal(t, ModeText, d.CurrentMode())
}

func TestClearExplicitMode(t *testing.T) {
	d := NewModeDetector()

	// Set and clear explicit mode
	d.SetExplicitMode(ModeVoice)
	assert.True(t, d.HasExplicitMode())

	d.ClearExplicitMode()
	assert.False(t, d.HasExplicitMode())

	// Should revert to automatic detection (text mode, since STT/TTS not active)
	assert.Equal(t, ModeText, d.CurrentMode())
}

func TestCurrentModeAutomatic(t *testing.T) {
	d := NewModeDetector()

	// Text mode when nothing is active
	assert.Equal(t, ModeText, d.CurrentMode())

	// Still text when only STT is active
	d.SetSTTActive(true)
	assert.Equal(t, ModeText, d.CurrentMode())

	// Still text when only TTS is enabled
	d.SetSTTActive(false)
	d.SetTTSEnabled(true)
	assert.Equal(t, ModeText, d.CurrentMode())

	// Voice mode when both STT active and TTS enabled
	d.SetSTTActive(true)
	assert.Equal(t, ModeVoice, d.CurrentMode())
	assert.True(t, d.IsVoiceMode())
	assert.False(t, d.IsTextMode())
}

func TestExplicitModeOverridesAutomatic(t *testing.T) {
	d := NewModeDetector()

	// Set up conditions for automatic voice mode
	d.SetSTTActive(true)
	d.SetTTSEnabled(true)
	assert.Equal(t, ModeVoice, d.CurrentMode())

	// Explicit text mode should override
	d.SetExplicitMode(ModeText)
	assert.Equal(t, ModeText, d.CurrentMode())

	// Clear explicit mode should revert to automatic voice mode
	d.ClearExplicitMode()
	assert.Equal(t, ModeVoice, d.CurrentMode())
}

func TestModeDetectorState(t *testing.T) {
	d := NewModeDetector()
	d.SetSTTActive(true)
	d.SetTTSEnabled(true)
	d.SetExplicitMode(ModeVoice)

	state := d.State()

	assert.True(t, state.STTActive)
	assert.True(t, state.TTSEnabled)
	assert.Equal(t, ModeVoice, state.ExplicitMode)
	assert.True(t, state.HasExplicit)
	assert.Equal(t, ModeVoice, state.CurrentMode)
}

func TestModeDetectorReset(t *testing.T) {
	d := NewModeDetector()

	// Set various states
	d.SetSTTActive(true)
	d.SetTTSEnabled(true)
	d.SetExplicitMode(ModeVoice)

	// Reset
	d.Reset()

	// Verify all reset to defaults
	assert.False(t, d.STTActive())
	assert.False(t, d.TTSEnabled())
	assert.False(t, d.HasExplicitMode())
	assert.Equal(t, ModeText, d.CurrentMode())
}

func TestModeDetectorConcurrency(t *testing.T) {
	d := NewModeDetector()
	iterations := 1000

	var wg sync.WaitGroup
	wg.Add(4)

	// Concurrent STT updates
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			d.SetSTTActive(i%2 == 0)
		}
	}()

	// Concurrent TTS updates
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			d.SetTTSEnabled(i%2 == 0)
		}
	}()

	// Concurrent mode reads
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = d.CurrentMode()
			_ = d.IsVoiceMode()
		}
	}()

	// Concurrent explicit mode changes
	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			if i%3 == 0 {
				d.SetExplicitMode(ModeVoice)
			} else if i%3 == 1 {
				d.SetExplicitMode(ModeText)
			} else {
				d.ClearExplicitMode()
			}
		}
	}()

	wg.Wait()

	// Should not panic and should have valid state
	mode := d.CurrentMode()
	assert.True(t, mode == ModeText || mode == ModeVoice)
}

// NFR-002: Mode detection overhead < 1ms
func TestModeDetectorPerformance(t *testing.T) {
	d := NewModeDetector()
	d.SetSTTActive(true)
	d.SetTTSEnabled(true)

	iterations := 10000
	start := time.Now()

	for i := 0; i < iterations; i++ {
		_ = d.CurrentMode()
	}

	elapsed := time.Since(start)
	avgPerOp := elapsed / time.Duration(iterations)

	// Average operation should be well under 1ms
	assert.Less(t, avgPerOp, time.Millisecond,
		"Mode detection took %v per operation, expected < 1ms", avgPerOp)

	// Log actual performance
	t.Logf("Mode detection: %v per operation (%d iterations in %v)",
		avgPerOp, iterations, elapsed)
}

func TestModeDetectorPerformanceUnderContention(t *testing.T) {
	d := NewModeDetector()
	iterations := 1000

	// Start background writers
	done := make(chan struct{})
	go func() {
		for {
			select {
			case <-done:
				return
			default:
				d.SetSTTActive(true)
				d.SetSTTActive(false)
			}
		}
	}()

	// Measure read performance under contention
	start := time.Now()
	for i := 0; i < iterations; i++ {
		_ = d.CurrentMode()
	}
	elapsed := time.Since(start)

	close(done)

	avgPerOp := elapsed / time.Duration(iterations)
	assert.Less(t, avgPerOp, time.Millisecond,
		"Mode detection under contention took %v per operation", avgPerOp)

	t.Logf("Mode detection under contention: %v per operation", avgPerOp)
}

func TestIsVoiceModeIsTextMode(t *testing.T) {
	d := NewModeDetector()

	// Text mode
	assert.True(t, d.IsTextMode())
	assert.False(t, d.IsVoiceMode())

	// Voice mode
	d.SetExplicitMode(ModeVoice)
	assert.False(t, d.IsTextMode())
	assert.True(t, d.IsVoiceMode())
}

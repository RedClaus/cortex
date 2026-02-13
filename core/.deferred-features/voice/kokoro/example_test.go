package kokoro_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/normanking/cortex/internal/voice"
	"github.com/normanking/cortex/internal/voice/kokoro"
)

// Example demonstrates basic usage of the Kokoro TTS provider.
func Example() {
	// Create a new Kokoro provider with default settings
	provider := kokoro.NewProvider(kokoro.Config{
		BaseURL:      "http://localhost:8880",
		DefaultVoice: "af_bella",
		Timeout:      5 * time.Second,
	})

	ctx := context.Background()

	// Check if Kokoro is running
	if err := provider.Health(ctx); err != nil {
		log.Printf("Kokoro is not available: %v", err)
		return
	}

	// List available voices
	voices, err := provider.ListVoices(ctx)
	if err != nil {
		log.Fatalf("Failed to list voices: %v", err)
	}

	fmt.Printf("Available voices: %d\n", len(voices))
	for _, v := range voices {
		fmt.Printf("- %s (%s, %s)\n", v.Name, v.Gender, v.Language)
	}

	// Synthesize speech
	req := &voice.SynthesizeRequest{
		Text:    "Hello from Kokoro TTS!",
		VoiceID: "af_bella",
		Speed:   1.0,
	}

	resp, err := provider.Synthesize(ctx, req)
	if err != nil {
		log.Fatalf("Synthesis failed: %v", err)
	}

	fmt.Printf("Generated %d bytes of %s audio in %dms\n",
		len(resp.Audio), resp.Format, resp.ProcessedMs)
}

// ExampleProvider_Stream demonstrates streaming audio synthesis.
func ExampleProvider_Stream() {
	provider := kokoro.NewProvider(kokoro.Config{})
	ctx := context.Background()

	req := &voice.SynthesizeRequest{
		Text:    "This is a streaming example.",
		VoiceID: "am_adam",
		Speed:   1.0,
	}

	stream, err := provider.Stream(ctx, req)
	if err != nil {
		log.Fatalf("Stream failed: %v", err)
	}
	defer stream.Close()

	// Read audio chunks
	buffer := make([]byte, 4096)
	totalBytes := 0

	for {
		n, err := stream.Read(buffer)
		if n > 0 {
			totalBytes += n
			// Process audio chunk here (e.g., send to audio player)
		}
		if err != nil {
			break
		}
	}

	fmt.Printf("Streamed %d bytes of %s audio\n", totalBytes, stream.Format())
}

// ExampleProvider_Capabilities demonstrates querying provider features.
func ExampleProvider_Capabilities() {
	provider := kokoro.NewProvider(kokoro.Config{})
	caps := provider.Capabilities()

	fmt.Printf("Kokoro Capabilities:\n")
	fmt.Printf("- Streaming: %v\n", caps.SupportsStreaming)
	fmt.Printf("- Voice Cloning: %v\n", caps.SupportsCloning)
	fmt.Printf("- GPU Required: %v\n", caps.RequiresGPU)
	fmt.Printf("- Avg Latency: %dms\n", caps.AvgLatencyMs)
	fmt.Printf("- Max Text Length: %d characters\n", caps.MaxTextLength)
	fmt.Printf("- Supported Formats: %v\n", caps.SupportedFormats)
	fmt.Printf("- Languages: %v\n", caps.Languages)

	// Output:
	// Kokoro Capabilities:
	// - Streaming: true
	// - Voice Cloning: false
	// - GPU Required: false
	// - Avg Latency: 250ms
	// - Max Text Length: 2000 characters
	// - Supported Formats: [wav]
	// - Languages: [en]
}

// ExampleNewProvider demonstrates creating a provider with custom config.
func ExampleNewProvider() {
	// Create provider with custom configuration
	provider := kokoro.NewProvider(kokoro.Config{
		BaseURL:       "http://kokoro-server:9000",
		DefaultVoice:  "bf_emma", // British female voice
		Timeout:       10 * time.Second,
		MaxTextLength: 1500,
	})

	fmt.Printf("Provider: %s\n", provider.Name())
	fmt.Printf("Default Voice: %s\n", provider.GetDefaultVoice())

	// Output:
	// Provider: kokoro
	// Default Voice: bf_emma
}

// ExampleProvider_ValidateVoice demonstrates voice validation.
func ExampleProvider_ValidateVoice() {
	provider := kokoro.NewProvider(kokoro.Config{})

	// Validate a voice
	voice, err := provider.ValidateVoice("am_michael")
	if err != nil {
		log.Fatalf("Invalid voice: %v", err)
	}

	fmt.Printf("Voice: %s\n", voice.Name)
	fmt.Printf("Gender: %s\n", voice.Gender)
	fmt.Printf("Language: %s\n", voice.Language)

	// Output:
	// Voice: Michael (US Male)
	// Gender: male
	// Language: en
}

// ExamplePresetVoices shows all available preset voices.
func ExamplePresetVoices() {
	for _, voice := range kokoro.PresetVoices {
		fmt.Printf("%s: %s (%s, %s, %s)\n",
			voice.ID,
			voice.Name,
			voice.Gender,
			voice.Language,
			voice.Metadata["region"])
	}

	// Output:
	// af_bella: Bella (US Female) (female, en, us)
	// af_sarah: Sarah (US Female) (female, en, us)
	// am_adam: Adam (US Male) (male, en, us)
	// am_michael: Michael (US Male) (male, en, us)
	// bf_emma: Emma (British Female) (female, en, uk)
	// bm_george: George (British Male) (male, en, uk)
}

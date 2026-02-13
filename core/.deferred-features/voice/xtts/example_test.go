package xtts_test

import (
	"context"
	"fmt"
	"os"

	"github.com/normanking/cortex/internal/voice"
	"github.com/normanking/cortex/internal/voice/xtts"
)

// ExampleProvider_basic demonstrates basic XTTS usage
func ExampleProvider_basic() {
	// Create XTTS provider
	provider := xtts.NewProvider(xtts.Config{
		BaseURL: "http://localhost:5002",
	})

	// Create synthesis request
	req := &voice.SynthesizeRequest{
		Text:    "Hello from XTTS!",
		VoiceID: "default",
		Speed:   1.0,
	}

	// Synthesize speech
	resp, err := provider.Synthesize(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Synthesized %d bytes of %s audio\n", len(resp.Audio), resp.Format)
	fmt.Printf("Processing time: %dms\n", resp.ProcessedMs)
}

// ExampleProvider_streaming demonstrates streaming synthesis
func ExampleProvider_streaming() {
	provider := xtts.NewProvider(xtts.Config{
		BaseURL: "http://localhost:5002",
	})

	req := &voice.SynthesizeRequest{
		Text:    "This is a longer text that will be streamed in chunks.",
		VoiceID: "default",
	}

	stream, err := provider.Stream(context.Background(), req)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}
	defer stream.Close()

	fmt.Printf("Streaming audio format: %s at %d Hz\n", stream.Format(), stream.SampleRate())
}

// ExampleProvider_listVoices demonstrates listing available voices
func ExampleProvider_listVoices() {
	provider := xtts.NewProvider(xtts.Config{})

	voices, err := provider.ListVoices(context.Background())
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Available voices:\n")
	for _, v := range voices {
		clonedFlag := ""
		if v.IsCloned {
			clonedFlag = " (cloned)"
		}
		fmt.Printf("- %s [%s]%s\n", v.Name, v.Language, clonedFlag)
	}

	// Output:
	// Available voices:
	// - Default [en]
}

// ExampleProvider_capabilities demonstrates querying provider capabilities
func ExampleProvider_capabilities() {
	provider := xtts.NewProvider(xtts.Config{})

	caps := provider.Capabilities()

	fmt.Printf("Provider: %s\n", provider.Name())
	fmt.Printf("Streaming: %v\n", caps.SupportsStreaming)
	fmt.Printf("Voice Cloning: %v\n", caps.SupportsCloning)
	fmt.Printf("Languages: %d\n", len(caps.Languages))
	fmt.Printf("Max Text Length: %d chars\n", caps.MaxTextLength)
	fmt.Printf("Requires GPU: %v\n", caps.RequiresGPU)
	fmt.Printf("Avg Latency: %dms\n", caps.AvgLatencyMs)

	// Output:
	// Provider: xtts
	// Streaming: true
	// Voice Cloning: true
	// Languages: 17
	// Max Text Length: 5000 chars
	// Requires GPU: true
	// Avg Latency: 1500ms
}

// ExampleProvider_cloneVoice demonstrates voice cloning
func ExampleProvider_cloneVoice() {
	provider := xtts.NewProvider(xtts.Config{
		ClonedVoicesDir: "/tmp/voices",
	})

	// Note: This example assumes a reference audio file exists
	// In practice, this should be a 6-second+ audio sample
	referenceFile := "/tmp/reference.wav"

	// Create the reference file for demonstration
	_ = os.WriteFile(referenceFile, []byte("fake audio data"), 0644)

	cloned, err := provider.CloneVoice(
		context.Background(),
		"Custom Voice",
		referenceFile,
		"en",
	)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		return
	}

	fmt.Printf("Cloned voice ID: %s\n", cloned.ID)
	fmt.Printf("Voice name: %s\n", cloned.Name)
	fmt.Printf("Language: %s\n", cloned.Language)
}

// ExampleProvider_withClonedVoice demonstrates using a cloned voice
func ExampleProvider_withClonedVoice() {
	provider := xtts.NewProvider(xtts.Config{})

	// First, clone a voice (see ExampleProvider_cloneVoice)
	// Then use it for synthesis:
	req := &voice.SynthesizeRequest{
		Text:    "Speaking with a cloned voice!",
		VoiceID: "custom_voice", // ID from CloneVoice()
		Speed:   1.0,
	}

	_, err := provider.Synthesize(context.Background(), req)
	if err != nil {
		// This will fail if the voice doesn't exist
		fmt.Printf("Error: %v\n", err)
		return
	}
}

// ExampleProvider_healthCheck demonstrates health checking
func ExampleProvider_healthCheck() {
	provider := xtts.NewProvider(xtts.Config{
		BaseURL: "http://localhost:5002",
	})

	err := provider.Health(context.Background())
	if err != nil {
		fmt.Printf("XTTS is not available: %v\n", err)
		return
	}

	fmt.Println("XTTS is healthy and ready")
}

// ExampleProvider_multiLanguage demonstrates multi-language synthesis
func ExampleProvider_multiLanguage() {
	provider := xtts.NewProvider(xtts.Config{})

	// XTTS automatically detects language from voice or defaults to English
	// For best results with non-English text, clone a voice in that language

	examples := map[string]string{
		"en": "Hello, world!",
		"es": "¡Hola, mundo!",
		"fr": "Bonjour, le monde!",
		"de": "Hallo, Welt!",
		"ja": "こんにちは、世界！",
	}

	for lang, text := range examples {
		req := &voice.SynthesizeRequest{
			Text:    text,
			VoiceID: "default", // In production, use language-specific voices
		}

		_, err := provider.Synthesize(context.Background(), req)
		if err != nil {
			fmt.Printf("%s synthesis failed: %v\n", lang, err)
			continue
		}

		fmt.Printf("%s: synthesized successfully\n", lang)
	}
}

// ExampleProvider_speedControl demonstrates speech rate control
func ExampleProvider_speedControl() {
	provider := xtts.NewProvider(xtts.Config{})

	text := "This text will be spoken at different speeds."

	speeds := []float64{0.5, 1.0, 1.5, 2.0}

	for _, speed := range speeds {
		req := &voice.SynthesizeRequest{
			Text:    text,
			VoiceID: "default",
			Speed:   speed,
		}

		resp, err := provider.Synthesize(context.Background(), req)
		if err != nil {
			fmt.Printf("Speed %.1fx failed: %v\n", speed, err)
			continue
		}

		fmt.Printf("Speed %.1fx: %d bytes in %dms\n", speed, len(resp.Audio), resp.ProcessedMs)
	}
}

// ExampleProvider_errorHandling demonstrates proper error handling
func ExampleProvider_errorHandling() {
	provider := xtts.NewProvider(xtts.Config{
		MaxTextLength: 100,
	})

	// Text too long
	_, err := provider.Synthesize(context.Background(), &voice.SynthesizeRequest{
		Text:    string(make([]byte, 101)),
		VoiceID: "default",
	})
	if err == voice.ErrTextTooLong {
		fmt.Println("Correctly detected text too long")
	}

	// Invalid voice
	_, err = provider.Synthesize(context.Background(), &voice.SynthesizeRequest{
		Text:    "Hello",
		VoiceID: "nonexistent",
	})
	if err == voice.ErrVoiceNotFound {
		fmt.Println("Correctly detected voice not found")
	}

	// Empty text
	_, err = provider.Synthesize(context.Background(), &voice.SynthesizeRequest{
		Text:    "",
		VoiceID: "default",
	})
	if err != nil {
		fmt.Println("Correctly rejected empty text")
	}

	// Output:
	// Correctly detected text too long
	// Correctly rejected empty text
}

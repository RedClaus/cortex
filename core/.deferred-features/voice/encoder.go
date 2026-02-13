package voice

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"

	"github.com/rs/zerolog/log"
)

// AudioEncoder handles format conversion using ffmpeg.
type AudioEncoder struct {
	ffmpegPath string
	mu         sync.Mutex
}

// NewAudioEncoder creates a new encoder and verifies ffmpeg is available.
func NewAudioEncoder() (*AudioEncoder, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found in PATH: %w", err)
	}

	log.Debug().Str("path", ffmpegPath).Msg("ffmpeg found")

	return &AudioEncoder{
		ffmpegPath: ffmpegPath,
	}, nil
}

// EncodeToOpus converts WAV audio to Opus format for Smart Lane compression.
// Opus provides excellent compression (~64kbps) with low latency.
func (e *AudioEncoder) EncodeToOpus(wavData []byte) ([]byte, error) {
	return e.encode(wavData, []string{
		"-f", "wav",        // Input format
		"-i", "pipe:0",     // Read from stdin
		"-c:a", "libopus",  // Opus codec
		"-b:a", "64k",      // 64 kbps bitrate
		"-vbr", "on",       // Variable bitrate
		"-compression_level", "10", // Maximum compression
		"-frame_duration", "20", // 20ms frames for low latency
		"-application", "voip", // Optimize for voice
		"-f", "opus",       // Output format
		"pipe:1",           // Write to stdout
	})
}

// EncodeToMP3 converts WAV audio to MP3 format for fallback compatibility.
// MP3 is widely supported but less efficient than Opus.
func (e *AudioEncoder) EncodeToMP3(wavData []byte) ([]byte, error) {
	return e.encode(wavData, []string{
		"-f", "wav",        // Input format
		"-i", "pipe:0",     // Read from stdin
		"-c:a", "libmp3lame", // MP3 codec
		"-b:a", "128k",     // 128 kbps bitrate
		"-q:a", "2",        // High quality (0-9, lower is better)
		"-f", "mp3",        // Output format
		"pipe:1",           // Write to stdout
	})
}

// encode performs the actual encoding using ffmpeg.
func (e *AudioEncoder) encode(wavData []byte, args []string) ([]byte, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)

	// Setup pipes
	cmd.Stdin = bytes.NewReader(wavData)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// Run encoding
	if err := cmd.Run(); err != nil {
		log.Error().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("ffmpeg encoding failed")
		return nil, fmt.Errorf("ffmpeg encoding failed: %w", err)
	}

	encoded := stdout.Bytes()
	duration := time.Since(start)

	log.Debug().
		Int("input_size", len(wavData)).
		Int("output_size", len(encoded)).
		Float64("compression_ratio", float64(len(wavData))/float64(len(encoded))).
		Dur("duration", duration).
		Msg("audio encoded")

	return encoded, nil
}

// StreamEncoder provides real-time audio encoding for streaming scenarios.
type StreamEncoder struct {
	encoder    *AudioEncoder
	format     AudioFormat
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     bytes.Buffer
	cancelFunc context.CancelFunc
	closed     bool
	mu         sync.Mutex
}

// NewStreamEncoder creates a new streaming encoder.
// The encoder must be closed after use.
func (e *AudioEncoder) NewStreamEncoder(format AudioFormat) (*StreamEncoder, error) {
	ctx, cancel := context.WithCancel(context.Background())

	var args []string
	switch format {
	case FormatOpus:
		args = []string{
			"-f", "wav",
			"-i", "pipe:0",
			"-c:a", "libopus",
			"-b:a", "64k",
			"-vbr", "on",
			"-compression_level", "10",
			"-frame_duration", "20",
			"-application", "voip",
			"-f", "opus",
			"pipe:1",
		}
	case FormatMP3:
		args = []string{
			"-f", "wav",
			"-i", "pipe:0",
			"-c:a", "libmp3lame",
			"-b:a", "128k",
			"-q:a", "2",
			"-f", "mp3",
			"pipe:1",
		}
	default:
		cancel()
		return nil, fmt.Errorf("unsupported format: %s", format)
	}

	cmd := exec.CommandContext(ctx, e.ffmpegPath, args...)

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		stdin.Close()
		cancel()
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	se := &StreamEncoder{
		encoder:    e,
		format:     format,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		cancelFunc: cancel,
	}

	cmd.Stderr = &se.stderr

	if err := cmd.Start(); err != nil {
		stdin.Close()
		stdout.Close()
		cancel()
		return nil, fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	log.Debug().Str("format", string(format)).Msg("stream encoder started")

	return se, nil
}

// Write writes WAV audio data to the encoder.
func (se *StreamEncoder) Write(data []byte) (int, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.closed {
		return 0, fmt.Errorf("encoder is closed")
	}

	return se.stdin.Write(data)
}

// Read reads encoded audio data from the encoder.
func (se *StreamEncoder) Read(buf []byte) (int, error) {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.closed {
		return 0, io.EOF
	}

	return se.stdout.Read(buf)
}

// Close closes the encoder and releases resources.
func (se *StreamEncoder) Close() error {
	se.mu.Lock()
	defer se.mu.Unlock()

	if se.closed {
		return nil
	}

	se.closed = true

	// Close stdin to signal end of input
	if err := se.stdin.Close(); err != nil {
		log.Warn().Err(err).Msg("failed to close stdin")
	}

	// Wait for process to finish
	if err := se.cmd.Wait(); err != nil {
		log.Warn().
			Err(err).
			Str("stderr", se.stderr.String()).
			Msg("ffmpeg process finished with error")
	}

	// Close stdout
	if err := se.stdout.Close(); err != nil {
		log.Warn().Err(err).Msg("failed to close stdout")
	}

	// Cancel context
	se.cancelFunc()

	log.Debug().Msg("stream encoder closed")

	return nil
}

// EncodeStreamToOpus is a convenience function for encoding a stream to Opus.
func EncodeStreamToOpus(encoder *AudioEncoder, input io.Reader) ([]byte, error) {
	streamEncoder, err := encoder.NewStreamEncoder(FormatOpus)
	if err != nil {
		return nil, err
	}
	defer streamEncoder.Close()

	// Copy input to encoder
	go func() {
		if _, err := io.Copy(streamEncoder, input); err != nil {
			log.Warn().Err(err).Msg("failed to copy input to encoder")
		}
	}()

	// Read all encoded output
	var output bytes.Buffer
	if _, err := io.Copy(&output, streamEncoder); err != nil {
		return nil, fmt.Errorf("failed to read encoded output: %w", err)
	}

	return output.Bytes(), nil
}

// EncodeStreamToMP3 is a convenience function for encoding a stream to MP3.
func EncodeStreamToMP3(encoder *AudioEncoder, input io.Reader) ([]byte, error) {
	streamEncoder, err := encoder.NewStreamEncoder(FormatMP3)
	if err != nil {
		return nil, err
	}
	defer streamEncoder.Close()

	// Copy input to encoder
	go func() {
		if _, err := io.Copy(streamEncoder, input); err != nil {
			log.Warn().Err(err).Msg("failed to copy input to encoder")
		}
	}()

	// Read all encoded output
	var output bytes.Buffer
	if _, err := io.Copy(&output, streamEncoder); err != nil {
		return nil, fmt.Errorf("failed to read encoded output: %w", err)
	}

	return output.Bytes(), nil
}

// Probe returns information about an audio file using ffprobe.
type AudioInfo struct {
	Format       string        `json:"format"`
	Duration     time.Duration `json:"duration"`
	Bitrate      int           `json:"bitrate"`
	SampleRate   int           `json:"sample_rate"`
	Channels     int           `json:"channels"`
	CodecName    string        `json:"codec_name"`
}

// Probe analyzes audio data and returns metadata.
func (e *AudioEncoder) Probe(audioData []byte) (*AudioInfo, error) {
	ffprobePath, err := exec.LookPath("ffprobe")
	if err != nil {
		return nil, fmt.Errorf("ffprobe not found in PATH: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-i", "pipe:0")

	cmd.Stdin = bytes.NewReader(audioData)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		log.Error().
			Err(err).
			Str("stderr", stderr.String()).
			Msg("ffprobe failed")
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Note: In a real implementation, you would parse the JSON output
	// For now, return placeholder
	return &AudioInfo{
		Format: "unknown",
	}, nil
}

package vision

import (
	"sync"
	"sync/atomic"
	"time"
)

// Frame represents a single video frame
type Frame struct {
	Data      []byte    `json:"data"`       // Raw image bytes
	MimeType  string    `json:"mime_type"`  // "image/jpeg", "image/png"
	Timestamp time.Time `json:"timestamp"`
	Sequence  int64     `json:"sequence"`   // Frame sequence number
	Width     int       `json:"width,omitempty"`
	Height    int       `json:"height,omitempty"`
}

// FrameBuffer is a thread-safe ring buffer for video frames
type FrameBuffer struct {
	frames   []*Frame
	capacity int
	head     int // Write position
	tail     int // Read position
	count    int // Current frame count
	mu       sync.RWMutex
	sequence atomic.Int64

	// Stats
	droppedFrames atomic.Int64
	totalFrames   atomic.Int64
}

// FrameBufferConfig configures the frame buffer
type FrameBufferConfig struct {
	Capacity int // Number of frames to buffer (default: 30)
}

// FrameBufferStats contains buffer statistics
type FrameBufferStats struct {
	Capacity      int     `json:"capacity"`
	CurrentCount  int     `json:"current_count"`
	TotalReceived int64   `json:"total_received"`
	TotalDropped  int64   `json:"total_dropped"`
	DropRate      float64 `json:"drop_rate"` // percentage
}

// NewFrameBuffer creates a new ring buffer for frames
func NewFrameBuffer(config FrameBufferConfig) *FrameBuffer {
	capacity := config.Capacity
	if capacity <= 0 {
		capacity = 30 // Default: buffer 1 second at 30fps
	}

	return &FrameBuffer{
		frames:   make([]*Frame, capacity),
		capacity: capacity,
		head:     0,
		tail:     0,
		count:    0,
	}
}

// Push adds a frame to the buffer, dropping the oldest if full
func (b *FrameBuffer) Push(frame *Frame) (dropped bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Assign sequence number
	frame.Sequence = b.sequence.Add(1)
	b.totalFrames.Add(1)

	// Check if buffer is full
	if b.count == b.capacity {
		// Drop oldest frame
		b.tail = (b.tail + 1) % b.capacity
		b.droppedFrames.Add(1)
		dropped = true
	} else {
		b.count++
	}

	// Write new frame
	b.frames[b.head] = frame
	b.head = (b.head + 1) % b.capacity

	return dropped
}

// Pop removes and returns the oldest frame
func (b *FrameBuffer) Pop() *Frame {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 {
		return nil
	}

	frame := b.frames[b.tail]
	b.frames[b.tail] = nil // Help GC
	b.tail = (b.tail + 1) % b.capacity
	b.count--

	return frame
}

// Peek returns the oldest frame without removing it
func (b *FrameBuffer) Peek() *Frame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	return b.frames[b.tail]
}

// PeekLatest returns the newest frame without removing it
func (b *FrameBuffer) PeekLatest() *Frame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	// Head points to next write position, so go back one
	latestIdx := (b.head - 1 + b.capacity) % b.capacity
	return b.frames[latestIdx]
}

// PopN removes and returns up to N oldest frames
func (b *FrameBuffer) PopN(n int) []*Frame {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.count == 0 || n <= 0 {
		return nil
	}

	// Limit to available frames
	if n > b.count {
		n = b.count
	}

	result := make([]*Frame, n)
	for i := 0; i < n; i++ {
		result[i] = b.frames[b.tail]
		b.frames[b.tail] = nil // Help GC
		b.tail = (b.tail + 1) % b.capacity
		b.count--
	}

	return result
}

// Len returns the current number of frames in the buffer
func (b *FrameBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count
}

// Clear empties the buffer
func (b *FrameBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Help GC by clearing references
	for i := 0; i < b.capacity; i++ {
		b.frames[i] = nil
	}

	b.head = 0
	b.tail = 0
	b.count = 0
}

// IsFull returns true if the buffer is at capacity
func (b *FrameBuffer) IsFull() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.count == b.capacity
}

// Stats returns buffer statistics
func (b *FrameBuffer) Stats() FrameBufferStats {
	b.mu.RLock()
	currentCount := b.count
	b.mu.RUnlock()

	totalReceived := b.totalFrames.Load()
	totalDropped := b.droppedFrames.Load()

	var dropRate float64
	if totalReceived > 0 {
		dropRate = float64(totalDropped) / float64(totalReceived) * 100
	}

	return FrameBufferStats{
		Capacity:      b.capacity,
		CurrentCount:  currentCount,
		TotalReceived: totalReceived,
		TotalDropped:  totalDropped,
		DropRate:      dropRate,
	}
}

// GetSampledFrames returns evenly-spaced frames from the buffer
// Useful for sending representative frames to Vision Lobe
func (b *FrameBuffer) GetSampledFrames(count int) []*Frame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 || count <= 0 {
		return nil
	}

	// Limit to available frames
	if count > b.count {
		count = b.count
	}

	// If requesting all or more frames, return everything
	if count >= b.count {
		result := make([]*Frame, b.count)
		for i := 0; i < b.count; i++ {
			idx := (b.tail + i) % b.capacity
			result[i] = b.frames[idx]
		}
		return result
	}

	// Sample evenly across the buffer
	result := make([]*Frame, count)
	step := float64(b.count-1) / float64(count-1)

	for i := 0; i < count; i++ {
		offset := int(float64(i) * step)
		idx := (b.tail + offset) % b.capacity
		result[i] = b.frames[idx]
	}

	return result
}

// GetFramesSince returns all frames after the given timestamp
func (b *FrameBuffer) GetFramesSince(since time.Time) []*Frame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	var result []*Frame

	for i := 0; i < b.count; i++ {
		idx := (b.tail + i) % b.capacity
		frame := b.frames[idx]
		if frame.Timestamp.After(since) {
			result = append(result, frame)
		}
	}

	return result
}

// GetFrameRange returns frames within a specific time range
func (b *FrameBuffer) GetFrameRange(start, end time.Time) []*Frame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	var result []*Frame

	for i := 0; i < b.count; i++ {
		idx := (b.tail + i) % b.capacity
		frame := b.frames[idx]
		if !frame.Timestamp.Before(start) && !frame.Timestamp.After(end) {
			result = append(result, frame)
		}
	}

	return result
}

// GetOldestTimestamp returns the timestamp of the oldest frame in the buffer
func (b *FrameBuffer) GetOldestTimestamp() (time.Time, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return time.Time{}, false
	}

	return b.frames[b.tail].Timestamp, true
}

// GetNewestTimestamp returns the timestamp of the newest frame in the buffer
func (b *FrameBuffer) GetNewestTimestamp() (time.Time, bool) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return time.Time{}, false
	}

	latestIdx := (b.head - 1 + b.capacity) % b.capacity
	return b.frames[latestIdx].Timestamp, true
}

// GetAllFrames returns all frames in chronological order (oldest first)
// This creates a copy of the frame pointers, not the frame data
func (b *FrameBuffer) GetAllFrames() []*Frame {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if b.count == 0 {
		return nil
	}

	result := make([]*Frame, b.count)
	for i := 0; i < b.count; i++ {
		idx := (b.tail + i) % b.capacity
		result[i] = b.frames[idx]
	}

	return result
}

// Capacity returns the maximum capacity of the buffer
func (b *FrameBuffer) Capacity() int {
	return b.capacity
}

// AvailableSpace returns how many more frames can be added before dropping
func (b *FrameBuffer) AvailableSpace() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.capacity - b.count
}

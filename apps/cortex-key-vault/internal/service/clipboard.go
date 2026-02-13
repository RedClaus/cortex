package service

import (
	"context"
	"sync"
	"time"

	"golang.design/x/clipboard"
)

const (
	// DefaultClearTimeout is the default time before clipboard is cleared
	DefaultClearTimeout = 30 * time.Second
)

// ClipboardService handles secure clipboard operations
type ClipboardService struct {
	mu           sync.Mutex
	clearTimeout time.Duration
	lastCopied   string
	cancelFunc   context.CancelFunc
	initialized  bool
	initOnce     sync.Once
}

// NewClipboardService creates a new clipboard service
func NewClipboardService() *ClipboardService {
	return &ClipboardService{
		clearTimeout: DefaultClearTimeout,
	}
}

// init lazily initializes the clipboard
func (c *ClipboardService) init() {
	c.initOnce.Do(func() {
		// Initialize clipboard - this may fail silently on some systems
		err := clipboard.Init()
		c.initialized = (err == nil)
	})
}

// Copy copies text to clipboard and schedules auto-clear
func (c *ClipboardService) Copy(text string) {
	c.init()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return // Clipboard not available
	}

	// Cancel any pending clear operation
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}

	// Copy to clipboard
	clipboard.Write(clipboard.FmtText, []byte(text))
	c.lastCopied = text

	// Capture timeout value while holding lock
	timeout := c.clearTimeout

	// Create a new context for this clear operation
	ctx, cancel := context.WithCancel(context.Background())
	c.cancelFunc = cancel

	// Schedule clear with context
	go c.scheduleClear(ctx, text, timeout)
}

// scheduleClear waits for timeout then clears clipboard
func (c *ClipboardService) scheduleClear(ctx context.Context, copiedText string, timeout time.Duration) {
	select {
	case <-time.After(timeout):
		c.mu.Lock()
		defer c.mu.Unlock()

		if !c.initialized {
			return
		}

		// Only clear if this is still the active copy operation
		// and our copied content is still there
		if c.lastCopied == copiedText {
			current := string(clipboard.Read(clipboard.FmtText))
			if current == copiedText {
				clipboard.Write(clipboard.FmtText, []byte(""))
			}
			c.lastCopied = ""
		}
	case <-ctx.Done():
		// Clear was cancelled (new copy or manual clear)
		return
	}
}

// Clear immediately clears the clipboard
func (c *ClipboardService) Clear() {
	c.init()

	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return
	}

	// Cancel any pending clear
	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}

	clipboard.Write(clipboard.FmtText, []byte(""))
	c.lastCopied = ""
}

// SetClearTimeout sets the auto-clear timeout duration
func (c *ClipboardService) SetClearTimeout(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.clearTimeout = d
}

// GetClearTimeout returns the current clear timeout
func (c *ClipboardService) GetClearTimeout() time.Duration {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.clearTimeout
}

// Close cleans up the clipboard service
func (c *ClipboardService) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cancelFunc != nil {
		c.cancelFunc()
		c.cancelFunc = nil
	}
}

// IsAvailable returns whether clipboard is available
func (c *ClipboardService) IsAvailable() bool {
	c.init()
	return c.initialized
}

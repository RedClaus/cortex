// Package avatar manages the avatar's state and animations
package avatar

import (
	"sync"
	"time"
)

// EmotionState represents the avatar's emotional state
type EmotionState string

const (
	EmotionNeutral   EmotionState = "neutral"
	EmotionHappy     EmotionState = "happy"
	EmotionSad       EmotionState = "sad"
	EmotionThinking  EmotionState = "thinking"
	EmotionConfused  EmotionState = "confused"
	EmotionExcited   EmotionState = "excited"
	EmotionSurprised EmotionState = "surprised"
)

// MouthShape for lip-sync (visemes)
// Extended from 9 to 15 shapes for better 3D avatar lip-sync
type MouthShape string

const (
	// Basic shapes (original 9)
	MouthClosed MouthShape = "closed" // Rest, silence
	MouthAh     MouthShape = "ah"     // Open vowels: AA, AE, AH, AW, AY
	MouthOh     MouthShape = "oh"     // Rounded vowels: AO, OW
	MouthEe     MouthShape = "ee"     // Spread vowels: IY, EY
	MouthFV     MouthShape = "fv"     // Labiodental: F, V
	MouthTH     MouthShape = "th"     // Dental: TH, DH
	MouthMBP    MouthShape = "mbp"    // Bilabial: M, B, P
	MouthLNT    MouthShape = "lnt"    // Alveolar: L, N, T, D
	MouthWQ     MouthShape = "wq"     // Rounded: W, Q

	// Extended shapes (6 new for better 3D lip-sync)
	MouthOO MouthShape = "oo" // Tight round: UH, UW, OY
	MouthIH MouthShape = "ih" // Short spread: IH, EH
	MouthER MouthShape = "er" // R-colored: ER
	MouthCH MouthShape = "ch" // Affricates: CH, JH, SH, ZH
	MouthNG MouthShape = "ng" // Velars back: NG
	MouthK  MouthShape = "k"  // Velars open: K, G
)

// EyeState represents eye animation state
type EyeState string

const (
	EyeOpen   EyeState = "open"
	EyeClosed EyeState = "closed"
	EyeHalf   EyeState = "half"
	EyeWide   EyeState = "wide"
	EyeSquint EyeState = "squint"
)

// LookDirection represents gaze direction
type LookDirection string

const (
	LookCenter LookDirection = "center"
	LookLeft   LookDirection = "left"
	LookRight  LookDirection = "right"
	LookUp     LookDirection = "up"
	LookDown   LookDirection = "down"
)

// State represents the avatar's current state
type State struct {
	Emotion       EmotionState  `json:"emotion"`
	MouthShape    MouthShape    `json:"mouthShape"`
	EyeState      EyeState      `json:"eyeState"`
	LookDirection LookDirection `json:"lookDirection"`
	IsSpeaking    bool          `json:"isSpeaking"`
	IsListening   bool          `json:"isListening"`
	IsThinking    bool          `json:"isThinking"`
}

// Phoneme represents a phoneme for lip-sync
type Phoneme struct {
	Symbol  string `json:"symbol"`
	StartMs int    `json:"startMs"`
	EndMs   int    `json:"endMs"`
	Viseme  string `json:"viseme"`
}

// Controller manages avatar state transitions
type Controller struct {
	state        State
	lipSyncQueue []Phoneme
	mu           sync.RWMutex

	onStateChange func(State)

	// Animation timers
	blinkTicker *time.Ticker
	stopChan    chan struct{}
}

// NewController creates a new avatar controller
func NewController() *Controller {
	return &Controller{
		state: State{
			Emotion:       EmotionNeutral,
			MouthShape:    MouthClosed,
			EyeState:      EyeOpen,
			LookDirection: LookCenter,
		},
		stopChan: make(chan struct{}),
	}
}

// SetStateHandler sets the callback for state changes
func (c *Controller) SetStateHandler(handler func(State)) {
	c.onStateChange = handler
}

// Start begins animation loops
func (c *Controller) Start() {
	c.blinkTicker = time.NewTicker(4 * time.Second)

	go func() {
		for {
			select {
			case <-c.stopChan:
				return
			case <-c.blinkTicker.C:
				c.blink()
			}
		}
	}()
}

// Stop halts all animation loops
func (c *Controller) Stop() {
	close(c.stopChan)
	if c.blinkTicker != nil {
		c.blinkTicker.Stop()
	}
}

// GetState returns the current state
func (c *Controller) GetState() State {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.state
}

// SetEmotion transitions to a new emotional state
func (c *Controller) SetEmotion(emotion EmotionState) {
	c.mu.Lock()
	c.state.Emotion = emotion
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// SetMouthShape sets the current mouth shape
func (c *Controller) SetMouthShape(shape MouthShape) {
	c.mu.Lock()
	c.state.MouthShape = shape
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// StartSpeaking begins speaking animation with phonemes
func (c *Controller) StartSpeaking(phonemes []Phoneme) {
	c.mu.Lock()
	c.state.IsSpeaking = true
	c.state.IsListening = false
	c.state.IsThinking = false
	c.lipSyncQueue = phonemes
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)

	// Start lip-sync animation
	if len(phonemes) > 0 {
		go c.animateLipSync()
	}
}

// StopSpeaking ends speaking animation
func (c *Controller) StopSpeaking() {
	c.mu.Lock()
	c.state.IsSpeaking = false
	c.state.MouthShape = MouthClosed
	c.lipSyncQueue = nil
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// StartListening begins listening animation
func (c *Controller) StartListening() {
	c.mu.Lock()
	c.state.IsListening = true
	c.state.IsSpeaking = false
	c.state.IsThinking = false
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// StopListening ends listening animation
func (c *Controller) StopListening() {
	c.mu.Lock()
	c.state.IsListening = false
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// StartThinking begins thinking animation
func (c *Controller) StartThinking() {
	c.mu.Lock()
	c.state.IsThinking = true
	c.state.IsListening = false
	c.state.IsSpeaking = false
	c.state.Emotion = EmotionThinking
	c.state.LookDirection = LookUp
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// StopThinking ends thinking animation
func (c *Controller) StopThinking() {
	c.mu.Lock()
	c.state.IsThinking = false
	c.state.LookDirection = LookCenter
	if c.state.Emotion == EmotionThinking {
		c.state.Emotion = EmotionNeutral
	}
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// SetIdle returns to idle state
func (c *Controller) SetIdle() {
	c.mu.Lock()
	c.state.IsSpeaking = false
	c.state.IsListening = false
	c.state.IsThinking = false
	c.state.Emotion = EmotionNeutral
	c.state.MouthShape = MouthClosed
	c.state.LookDirection = LookCenter
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)
}

// blink performs a blink animation
func (c *Controller) blink() {
	c.mu.Lock()
	// Don't blink while speaking
	if c.state.IsSpeaking {
		c.mu.Unlock()
		return
	}
	c.state.EyeState = EyeClosed
	state := c.state
	c.mu.Unlock()

	c.notifyStateChange(state)

	// Open eyes after blink duration
	time.AfterFunc(150*time.Millisecond, func() {
		c.mu.Lock()
		c.state.EyeState = EyeOpen
		state := c.state
		c.mu.Unlock()
		c.notifyStateChange(state)
	})
}

// animateLipSync animates mouth shapes based on phonemes
func (c *Controller) animateLipSync() {
	c.mu.RLock()
	phonemes := make([]Phoneme, len(c.lipSyncQueue))
	copy(phonemes, c.lipSyncQueue)
	c.mu.RUnlock()

	startTime := time.Now()

	for _, phoneme := range phonemes {
		c.mu.RLock()
		speaking := c.state.IsSpeaking
		c.mu.RUnlock()

		if !speaking {
			return
		}

		// Wait until phoneme start time
		elapsed := time.Since(startTime).Milliseconds()
		waitMs := int64(phoneme.StartMs) - elapsed
		if waitMs > 0 {
			time.Sleep(time.Duration(waitMs) * time.Millisecond)
		}

		// Set mouth shape
		shape := PhonemeToViseme(phoneme.Symbol)
		c.SetMouthShape(shape)

		// Hold for phoneme duration
		durationMs := phoneme.EndMs - phoneme.StartMs
		if durationMs > 0 {
			time.Sleep(time.Duration(durationMs) * time.Millisecond)
		}
	}

	// Return to closed mouth
	c.SetMouthShape(MouthClosed)
}

// notifyStateChange sends state update to handler
func (c *Controller) notifyStateChange(state State) {
	if c.onStateChange != nil {
		c.onStateChange(state)
	}
}

// PhonemeToViseme maps a phoneme to a mouth shape
// Updated mapping for extended 15-viseme set for better 3D lip-sync
func PhonemeToViseme(phoneme string) MouthShape {
	mapping := map[string]MouthShape{
		// Silence
		"sil": MouthClosed,
		"sp":  MouthClosed,

		// Open vowels (ah) - wide open jaw
		"AA": MouthAh, "AE": MouthAh, "AH": MouthAh, "AW": MouthAh, "AY": MouthAh,

		// Rounded open vowels (oh) - open with lip rounding
		"AO": MouthOh, "OW": MouthOh,

		// Tight rounded vowels (oo) - pursed lips
		"UH": MouthOO, "UW": MouthOO, "OY": MouthOO,

		// Spread vowels (ee) - wide smile
		"IY": MouthEe, "EY": MouthEe,

		// Short spread vowels (ih) - slight spread
		"IH": MouthIH, "EH": MouthIH,

		// R-colored vowels (er) - slight pucker with tongue
		"ER": MouthER,

		// Labial consonants (lips together)
		"M": MouthMBP, "B": MouthMBP, "P": MouthMBP,

		// Labiodental (teeth on lip)
		"F": MouthFV, "V": MouthFV,

		// Dental (tongue between teeth)
		"TH": MouthTH, "DH": MouthTH,

		// Alveolar (tongue behind teeth)
		"L": MouthLNT, "N": MouthLNT, "T": MouthLNT, "D": MouthLNT, "S": MouthLNT, "Z": MouthLNT,

		// Rounded consonants
		"W": MouthWQ, "R": MouthWQ, "Y": MouthEe,

		// Affricates and fricatives (ch) - rounded with slight opening
		"CH": MouthCH, "JH": MouthCH, "SH": MouthCH, "ZH": MouthCH,

		// Velar nasal (ng) - back of mouth
		"NG": MouthNG,

		// Velar stops (k) - open for release
		"K": MouthK, "G": MouthK,

		// Glottal
		"HH": MouthAh,
	}

	if shape, ok := mapping[phoneme]; ok {
		return shape
	}
	return MouthClosed
}

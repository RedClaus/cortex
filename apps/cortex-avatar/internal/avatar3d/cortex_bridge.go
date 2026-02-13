package avatar3d

import (
	"sync"
	"time"
)

type CognitiveMode string

const (
	ModeIdle      CognitiveMode = "idle"
	ModeListening CognitiveMode = "listening"
	ModeThinking  CognitiveMode = "thinking"
	ModeSpeaking  CognitiveMode = "speaking"
	ModeAttentive CognitiveMode = "attentive"
)

type CortexState struct {
	Timestamp time.Time

	Valence float32 // -1 (negative) to +1 (positive)
	Arousal float32 // 0 (calm) to 1 (activated)

	AttentionLevel float32
	Confidence     float32
	ProcessingLoad float32
	Mode           CognitiveMode

	GazeTarget *GazePoint
	IsSpeaking bool
	Visemes    []Viseme
}

type GazePoint struct {
	X float32
	Y float32
}

type Viseme struct {
	Shape    VisemeShape
	Weight   float32
	Duration float32
	Offset   float32
}

type VisemeShape string

const (
	VisemeSil VisemeShape = "sil"
	VisemePP  VisemeShape = "PP"
	VisemeFF  VisemeShape = "FF"
	VisemeTH  VisemeShape = "TH"
	VisemeDD  VisemeShape = "DD"
	VisemeKK  VisemeShape = "kk"
	VisemeCH  VisemeShape = "CH"
	VisemeSS  VisemeShape = "SS"
	VisemeNN  VisemeShape = "nn"
	VisemeRR  VisemeShape = "RR"
	VisemeAA  VisemeShape = "aa"
	VisemeE   VisemeShape = "E"
	VisemeI   VisemeShape = "I"
	VisemeO   VisemeShape = "O"
	VisemeU   VisemeShape = "U"
)

type CortexBridge struct {
	mu sync.RWMutex

	currentState CortexState
	stateBuffer  []CortexState
	bufferSize   int

	onStateChange func(CortexState)
}

func NewCortexBridge() *CortexBridge {
	return &CortexBridge{
		bufferSize: 10,
		currentState: CortexState{
			Mode:           ModeIdle,
			Valence:        0,
			Arousal:        0.3,
			AttentionLevel: 0.5,
			Confidence:     0.7,
		},
		stateBuffer: make([]CortexState, 0, 10),
	}
}

func (b *CortexBridge) SetOnStateChange(fn func(CortexState)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.onStateChange = fn
}

func (b *CortexBridge) UpdateState(state CortexState) {
	b.mu.Lock()
	state.Timestamp = time.Now()
	b.stateBuffer = append(b.stateBuffer, state)
	if len(b.stateBuffer) > b.bufferSize {
		b.stateBuffer = b.stateBuffer[1:]
	}
	b.currentState = state
	callback := b.onStateChange
	b.mu.Unlock()

	if callback != nil {
		callback(state)
	}
}

func (b *CortexBridge) GetState() CortexState {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.currentState
}

func (b *CortexBridge) SetMode(mode CognitiveMode) {
	b.mu.Lock()
	b.currentState.Mode = mode
	b.currentState.Timestamp = time.Now()
	b.mu.Unlock()
}

func (b *CortexBridge) SetValence(v float32) {
	b.mu.Lock()
	b.currentState.Valence = clamp(v, -1, 1)
	b.currentState.Timestamp = time.Now()
	b.mu.Unlock()
}

func (b *CortexBridge) SetArousal(a float32) {
	b.mu.Lock()
	b.currentState.Arousal = clamp(a, 0, 1)
	b.currentState.Timestamp = time.Now()
	b.mu.Unlock()
}

func (b *CortexBridge) SetSpeaking(speaking bool) {
	b.mu.Lock()
	b.currentState.IsSpeaking = speaking
	b.currentState.Timestamp = time.Now()
	b.mu.Unlock()
}

func (b *CortexBridge) SetVisemes(visemes []Viseme) {
	b.mu.Lock()
	b.currentState.Visemes = visemes
	b.currentState.Timestamp = time.Now()
	b.mu.Unlock()
}

func (b *CortexBridge) SetGazeTarget(x, y float32) {
	b.mu.Lock()
	b.currentState.GazeTarget = &GazePoint{X: x, Y: y}
	b.currentState.Timestamp = time.Now()
	b.mu.Unlock()
}

func (b *CortexBridge) GetInterpolatedState(targetTime time.Time) CortexState {
	b.mu.RLock()
	defer b.mu.RUnlock()

	if len(b.stateBuffer) < 2 {
		return b.currentState
	}

	var before, after *CortexState
	for i := len(b.stateBuffer) - 1; i >= 0; i-- {
		if b.stateBuffer[i].Timestamp.Before(targetTime) {
			before = &b.stateBuffer[i]
			if i+1 < len(b.stateBuffer) {
				after = &b.stateBuffer[i+1]
			}
			break
		}
	}

	if before == nil || after == nil {
		return b.currentState
	}

	totalDur := after.Timestamp.Sub(before.Timestamp).Seconds()
	if totalDur <= 0 {
		return b.currentState
	}

	elapsed := targetTime.Sub(before.Timestamp).Seconds()
	t := float32(elapsed / totalDur)

	return interpolateCortexState(*before, *after, t)
}

func interpolateCortexState(a, b CortexState, t float32) CortexState {
	t = clamp(t, 0, 1)

	result := CortexState{
		Timestamp:      b.Timestamp,
		Valence:        lerp(a.Valence, b.Valence, t),
		Arousal:        lerp(a.Arousal, b.Arousal, t),
		AttentionLevel: lerp(a.AttentionLevel, b.AttentionLevel, t),
		Confidence:     lerp(a.Confidence, b.Confidence, t),
		ProcessingLoad: lerp(a.ProcessingLoad, b.ProcessingLoad, t),
		Mode:           b.Mode,
		IsSpeaking:     b.IsSpeaking,
		Visemes:        b.Visemes,
	}

	if a.GazeTarget != nil && b.GazeTarget != nil {
		result.GazeTarget = &GazePoint{
			X: lerp(a.GazeTarget.X, b.GazeTarget.X, t),
			Y: lerp(a.GazeTarget.Y, b.GazeTarget.Y, t),
		}
	} else {
		result.GazeTarget = b.GazeTarget
	}

	return result
}

func lerp(a, b, t float32) float32 {
	return a + (b-a)*t
}

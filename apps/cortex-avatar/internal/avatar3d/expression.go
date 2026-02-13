package avatar3d

import (
	"math"
	"sync"
	"time"
)

type ExpressionPreset struct {
	Name    string
	Weights BlendshapeWeights
}

var (
	PresetNeutral = ExpressionPreset{
		Name:    "neutral",
		Weights: NewBlendshapeWeights(),
	}

	PresetAttentive = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(BrowInnerUp, 0.15)
		w.Set(EyeWideLeft, 0.1)
		w.Set(EyeWideRight, 0.1)
		w.Set(MouthSmileLeft, 0.05)
		w.Set(MouthSmileRight, 0.05)
		return ExpressionPreset{Name: "attentive", Weights: w}
	}()

	PresetThinking = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(BrowInnerUp, 0.25)
		w.Set(EyeLookUpLeft, 0.3)
		w.Set(EyeLookUpRight, 0.3)
		w.Set(MouthPressLeft, 0.1)
		w.Set(MouthPressRight, 0.1)
		return ExpressionPreset{Name: "thinking", Weights: w}
	}()

	PresetConcerned = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(BrowInnerUp, 0.35)
		w.Set(BrowDownLeft, 0.2)
		w.Set(BrowDownRight, 0.2)
		w.Set(MouthFrownLeft, 0.15)
		w.Set(MouthFrownRight, 0.15)
		return ExpressionPreset{Name: "concerned", Weights: w}
	}()

	PresetConfident = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(MouthSmileLeft, 0.2)
		w.Set(MouthSmileRight, 0.2)
		w.Set(CheekSquintLeft, 0.1)
		w.Set(CheekSquintRight, 0.1)
		w.Set(EyeSquintLeft, 0.05)
		w.Set(EyeSquintRight, 0.05)
		return ExpressionPreset{Name: "confident", Weights: w}
	}()

	PresetSurprised = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(BrowInnerUp, 0.4)
		w.Set(BrowOuterUpLeft, 0.3)
		w.Set(BrowOuterUpRight, 0.3)
		w.Set(EyeWideLeft, 0.4)
		w.Set(EyeWideRight, 0.4)
		w.Set(JawOpen, 0.2)
		return ExpressionPreset{Name: "surprised", Weights: w}
	}()

	PresetHappy = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(MouthSmileLeft, 0.4)
		w.Set(MouthSmileRight, 0.4)
		w.Set(CheekSquintLeft, 0.25)
		w.Set(CheekSquintRight, 0.25)
		w.Set(EyeSquintLeft, 0.15)
		w.Set(EyeSquintRight, 0.15)
		return ExpressionPreset{Name: "happy", Weights: w}
	}()

	PresetSad = func() ExpressionPreset {
		w := NewBlendshapeWeights()
		w.Set(BrowInnerUp, 0.4)
		w.Set(BrowDownLeft, 0.1)
		w.Set(BrowDownRight, 0.1)
		w.Set(MouthFrownLeft, 0.25)
		w.Set(MouthFrownRight, 0.25)
		w.Set(EyeSquintLeft, 0.1)
		w.Set(EyeSquintRight, 0.1)
		return ExpressionPreset{Name: "sad", Weights: w}
	}()
)

type InterpolationMode int

const (
	InterpLinear InterpolationMode = iota
	InterpEaseInOut
	InterpEaseIn
	InterpEaseOut
	InterpSpring
)

const (
	TransitionFast   = 150 * time.Millisecond
	TransitionNormal = 300 * time.Millisecond
	TransitionSlow   = 500 * time.Millisecond
)

type BlendTransition struct {
	From      BlendshapeWeights
	To        BlendshapeWeights
	StartTime time.Time
	Duration  time.Duration
	Mode      InterpolationMode
}

func (t *BlendTransition) Interpolate(now time.Time) BlendshapeWeights {
	elapsed := now.Sub(t.StartTime)
	if elapsed >= t.Duration {
		return t.To
	}

	progress := float32(elapsed) / float32(t.Duration)

	switch t.Mode {
	case InterpEaseInOut:
		progress = easeInOutCubic(progress)
	case InterpEaseIn:
		progress = easeInCubic(progress)
	case InterpEaseOut:
		progress = easeOutCubic(progress)
	case InterpSpring:
		progress = springInterpolation(progress, 0.3, 8.0)
	}

	return t.From.Lerp(&t.To, progress)
}

func (t *BlendTransition) IsComplete(now time.Time) bool {
	return now.Sub(t.StartTime) >= t.Duration
}

type ExpressionController struct {
	mu sync.RWMutex

	currentWeights BlendshapeWeights
	transition     *BlendTransition
	layers         map[string]*ExpressionLayer
}

type ExpressionLayer struct {
	Name      string
	Weights   BlendshapeWeights
	Intensity float32
	BlendMode BlendMode
}

type BlendMode int

const (
	BlendModeOverride BlendMode = iota
	BlendModeAdditive
	BlendModeMultiply
)

func NewExpressionController() *ExpressionController {
	return &ExpressionController{
		currentWeights: NewBlendshapeWeights(),
		layers:         make(map[string]*ExpressionLayer),
	}
}

func (ec *ExpressionController) GetCurrentWeights() BlendshapeWeights {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.currentWeights
}

func (ec *ExpressionController) TransitionTo(target BlendshapeWeights, duration time.Duration, mode InterpolationMode) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.transition = &BlendTransition{
		From:      ec.currentWeights,
		To:        target,
		StartTime: time.Now(),
		Duration:  duration,
		Mode:      mode,
	}
}

func (ec *ExpressionController) TransitionToPreset(preset ExpressionPreset, duration time.Duration) {
	ec.TransitionTo(preset.Weights, duration, InterpEaseInOut)
}

func (ec *ExpressionController) SetImmediate(weights BlendshapeWeights) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.currentWeights = weights
	ec.transition = nil
}

func (ec *ExpressionController) AddLayer(name string, weights BlendshapeWeights, intensity float32, mode BlendMode) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	ec.layers[name] = &ExpressionLayer{
		Name:      name,
		Weights:   weights,
		Intensity: clamp(intensity, 0, 1),
		BlendMode: mode,
	}
}

func (ec *ExpressionController) RemoveLayer(name string) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	delete(ec.layers, name)
}

func (ec *ExpressionController) SetLayerIntensity(name string, intensity float32) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	if layer, ok := ec.layers[name]; ok {
		layer.Intensity = clamp(intensity, 0, 1)
	}
}

func (ec *ExpressionController) Update(dt float32) BlendshapeWeights {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	now := time.Now()

	if ec.transition != nil {
		ec.currentWeights = ec.transition.Interpolate(now)
		if ec.transition.IsComplete(now) {
			ec.transition = nil
		}
	}

	result := ec.currentWeights
	for _, layer := range ec.layers {
		result = ec.applyLayer(result, layer)
	}

	return result
}

func (ec *ExpressionController) IsTransitioning() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.transition != nil
}

func (ec *ExpressionController) applyLayer(base BlendshapeWeights, layer *ExpressionLayer) BlendshapeWeights {
	scaled := layer.Weights.Scale(layer.Intensity)

	switch layer.BlendMode {
	case BlendModeAdditive:
		return base.Add(&scaled)
	case BlendModeMultiply:
		var result BlendshapeWeights
		for i := range base {
			result[i] = clamp(base[i]*scaled[i], 0, 1)
		}
		return result
	default:
		return base.Lerp(&scaled, layer.Intensity)
	}
}

func easeInOutCubic(t float32) float32 {
	if t < 0.5 {
		return 4 * t * t * t
	}
	return 1 - float32(math.Pow(float64(-2*t+2), 3))/2
}

func easeInCubic(t float32) float32 {
	return t * t * t
}

func easeOutCubic(t float32) float32 {
	return 1 - float32(math.Pow(float64(1-t), 3))
}

func springInterpolation(t, damping, frequency float32) float32 {
	decay := float32(math.Exp(float64(-damping * t * frequency)))
	oscillation := float32(math.Cos(float64(frequency * t * (1 - damping))))
	return 1 - decay*oscillation
}

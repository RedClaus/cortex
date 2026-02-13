package avatar3d

import (
	"math"
	"math/rand"
	"sync"
	"time"
)

type EyeController struct {
	mu sync.RWMutex

	gazeTarget    GazeTarget
	currentGaze   GazeTarget
	gazeSmoothing float32

	blinkState    BlinkState
	nextBlinkTime time.Time
	blinkProgress float32
	blinkDuration float32
	minBlinkGap   time.Duration
	maxBlinkGap   time.Duration

	saccadeEnabled   bool
	saccadeAmplitude float32
	nextSaccadeTime  time.Time
	saccadeOffset    GazeTarget
}

type GazeTarget struct {
	X float32 // -1 (left) to +1 (right)
	Y float32 // -1 (down) to +1 (up)
}

type BlinkState int

const (
	BlinkStateOpen BlinkState = iota
	BlinkStateClosing
	BlinkStateClosed
	BlinkStateOpening
)

func NewEyeController() *EyeController {
	return &EyeController{
		gazeSmoothing:    8.0,
		blinkDuration:    0.15,
		minBlinkGap:      2 * time.Second,
		maxBlinkGap:      5 * time.Second,
		nextBlinkTime:    time.Now().Add(randomDuration(2*time.Second, 4*time.Second)),
		saccadeEnabled:   true,
		saccadeAmplitude: 0.05,
		nextSaccadeTime:  time.Now().Add(randomDuration(500*time.Millisecond, 2*time.Second)),
	}
}

func (ec *EyeController) SetGazeTarget(target GazeTarget) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.gazeTarget = target
}

func (ec *EyeController) LookAt(x, y float32) {
	ec.SetGazeTarget(GazeTarget{X: clamp(x, -1, 1), Y: clamp(y, -1, 1)})
}

func (ec *EyeController) LookAtCamera() {
	ec.SetGazeTarget(GazeTarget{X: 0, Y: 0})
}

func (ec *EyeController) TriggerBlink() {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	if ec.blinkState == BlinkStateOpen {
		ec.blinkState = BlinkStateClosing
		ec.blinkProgress = 0
	}
}

func (ec *EyeController) SetBlinkRate(minGap, maxGap time.Duration) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.minBlinkGap = minGap
	ec.maxBlinkGap = maxGap
}

func (ec *EyeController) SetSaccadeEnabled(enabled bool) {
	ec.mu.Lock()
	defer ec.mu.Unlock()
	ec.saccadeEnabled = enabled
}

func (ec *EyeController) Update(dt float32, weights *BlendshapeWeights) {
	ec.mu.Lock()
	defer ec.mu.Unlock()

	now := time.Now()

	ec.updateGaze(dt)
	ec.updateBlink(dt, now)
	ec.updateSaccade(now)
	ec.applyToWeights(weights)
}

func (ec *EyeController) updateGaze(dt float32) {
	lerpFactor := 1.0 - float32(math.Exp(float64(-ec.gazeSmoothing*dt)))

	target := ec.gazeTarget
	if ec.saccadeEnabled {
		target.X += ec.saccadeOffset.X
		target.Y += ec.saccadeOffset.Y
	}

	ec.currentGaze.X += (target.X - ec.currentGaze.X) * lerpFactor
	ec.currentGaze.Y += (target.Y - ec.currentGaze.Y) * lerpFactor
}

func (ec *EyeController) updateBlink(dt float32, now time.Time) {
	switch ec.blinkState {
	case BlinkStateOpen:
		if now.After(ec.nextBlinkTime) {
			ec.blinkState = BlinkStateClosing
			ec.blinkProgress = 0
		}

	case BlinkStateClosing:
		ec.blinkProgress += dt / (ec.blinkDuration * 0.4)
		if ec.blinkProgress >= 1.0 {
			ec.blinkProgress = 1.0
			ec.blinkState = BlinkStateClosed
		}

	case BlinkStateClosed:
		ec.blinkProgress += dt / (ec.blinkDuration * 0.1)
		if ec.blinkProgress >= 1.1 {
			ec.blinkState = BlinkStateOpening
			ec.blinkProgress = 1.0
		}

	case BlinkStateOpening:
		ec.blinkProgress -= dt / (ec.blinkDuration * 0.5)
		if ec.blinkProgress <= 0 {
			ec.blinkProgress = 0
			ec.blinkState = BlinkStateOpen
			ec.nextBlinkTime = now.Add(randomDuration(ec.minBlinkGap, ec.maxBlinkGap))
		}
	}
}

func (ec *EyeController) updateSaccade(now time.Time) {
	if !ec.saccadeEnabled {
		ec.saccadeOffset = GazeTarget{}
		return
	}

	if now.After(ec.nextSaccadeTime) {
		ec.saccadeOffset.X = (rand.Float32()*2 - 1) * ec.saccadeAmplitude
		ec.saccadeOffset.Y = (rand.Float32()*2 - 1) * ec.saccadeAmplitude * 0.5
		ec.nextSaccadeTime = now.Add(randomDuration(300*time.Millisecond, 1500*time.Millisecond))
	}
}

func (ec *EyeController) applyToWeights(weights *BlendshapeWeights) {
	gazeX := ec.currentGaze.X
	gazeY := ec.currentGaze.Y

	if gazeX > 0 {
		weights.Set(EyeLookOutLeft, gazeX*0.8)
		weights.Set(EyeLookInRight, gazeX*0.8)
		weights.Set(EyeLookOutRight, 0)
		weights.Set(EyeLookInLeft, 0)
	} else {
		weights.Set(EyeLookOutRight, -gazeX*0.8)
		weights.Set(EyeLookInLeft, -gazeX*0.8)
		weights.Set(EyeLookOutLeft, 0)
		weights.Set(EyeLookInRight, 0)
	}

	if gazeY > 0 {
		weights.Set(EyeLookUpLeft, gazeY*0.6)
		weights.Set(EyeLookUpRight, gazeY*0.6)
		weights.Set(EyeLookDownLeft, 0)
		weights.Set(EyeLookDownRight, 0)
	} else {
		weights.Set(EyeLookDownLeft, -gazeY*0.6)
		weights.Set(EyeLookDownRight, -gazeY*0.6)
		weights.Set(EyeLookUpLeft, 0)
		weights.Set(EyeLookUpRight, 0)
	}

	blinkAmount := ec.getBlinkAmount()
	weights.Set(EyeBlinkLeft, blinkAmount)
	weights.Set(EyeBlinkRight, blinkAmount)
}

func (ec *EyeController) getBlinkAmount() float32 {
	switch ec.blinkState {
	case BlinkStateClosing:
		return easeOutQuad(ec.blinkProgress)
	case BlinkStateClosed:
		return 1.0
	case BlinkStateOpening:
		return easeInQuad(ec.blinkProgress)
	default:
		return 0
	}
}

func (ec *EyeController) GetCurrentGaze() GazeTarget {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.currentGaze
}

func (ec *EyeController) IsBlinking() bool {
	ec.mu.RLock()
	defer ec.mu.RUnlock()
	return ec.blinkState != BlinkStateOpen
}

func easeOutQuad(t float32) float32 {
	return t * (2 - t)
}

func easeInQuad(t float32) float32 {
	return t * t
}

func randomDuration(min, max time.Duration) time.Duration {
	return min + time.Duration(rand.Float64()*float64(max-min))
}

package avatar3d

import (
	"math"
	"math/rand"
	"sync"
)

type IdleAnimator struct {
	mu sync.RWMutex

	enabled   bool
	intensity float32
	time      float32

	breathingRate      float32
	breathingAmplitude float32

	microMovementRate      float32
	microMovementAmplitude float32

	headSwayRate      float32
	headSwayAmplitude float32

	noiseOffsets [8]float32
}

func NewIdleAnimator() *IdleAnimator {
	ia := &IdleAnimator{
		enabled:                true,
		intensity:              1.0,
		breathingRate:          0.2,
		breathingAmplitude:     0.03,
		microMovementRate:      0.5,
		microMovementAmplitude: 0.02,
		headSwayRate:           0.1,
		headSwayAmplitude:      0.015,
	}

	for i := range ia.noiseOffsets {
		ia.noiseOffsets[i] = rand.Float32() * 100
	}

	return ia
}

func (ia *IdleAnimator) SetEnabled(enabled bool) {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	ia.enabled = enabled
}

func (ia *IdleAnimator) SetIntensity(intensity float32) {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	ia.intensity = clamp(intensity, 0, 1)
}

func (ia *IdleAnimator) Update(dt float32, weights *BlendshapeWeights) {
	ia.mu.Lock()
	defer ia.mu.Unlock()

	if !ia.enabled || ia.intensity <= 0 {
		return
	}

	ia.time += dt

	ia.applyBreathing(weights)
	ia.applyMicroMovements(weights)
	ia.applyHeadSway(weights)
}

func (ia *IdleAnimator) applyBreathing(weights *BlendshapeWeights) {
	breathPhase := ia.time * ia.breathingRate * 2 * math.Pi
	breathValue := float32(math.Sin(float64(breathPhase)))*0.5 + 0.5
	breathValue *= ia.breathingAmplitude * ia.intensity

	jawOpen := weights.Get(JawOpen)
	weights.Set(JawOpen, clamp(jawOpen+breathValue*0.3, 0, 1))

	noseSneer := breathValue * 0.2
	weights.Set(NoseSneerLeft, clamp(weights.Get(NoseSneerLeft)+noseSneer, 0, 1))
	weights.Set(NoseSneerRight, clamp(weights.Get(NoseSneerRight)+noseSneer, 0, 1))
}

func (ia *IdleAnimator) applyMicroMovements(weights *BlendshapeWeights) {
	amp := ia.microMovementAmplitude * ia.intensity

	browNoise := ia.perlinNoise(ia.time*ia.microMovementRate, ia.noiseOffsets[0])
	weights.Set(BrowInnerUp, clamp(weights.Get(BrowInnerUp)+browNoise*amp, 0, 1))

	mouthNoise := ia.perlinNoise(ia.time*ia.microMovementRate*0.7, ia.noiseOffsets[1])
	weights.Set(MouthPressLeft, clamp(weights.Get(MouthPressLeft)+mouthNoise*amp*0.5, 0, 1))
	weights.Set(MouthPressRight, clamp(weights.Get(MouthPressRight)+mouthNoise*amp*0.5, 0, 1))

	cheekNoise := ia.perlinNoise(ia.time*ia.microMovementRate*0.5, ia.noiseOffsets[2])
	weights.Set(CheekSquintLeft, clamp(weights.Get(CheekSquintLeft)+cheekNoise*amp*0.3, 0, 1))
	weights.Set(CheekSquintRight, clamp(weights.Get(CheekSquintRight)+cheekNoise*amp*0.3, 0, 1))
}

func (ia *IdleAnimator) applyHeadSway(weights *BlendshapeWeights) {
	amp := ia.headSwayAmplitude * ia.intensity

	swayX := ia.perlinNoise(ia.time*ia.headSwayRate, ia.noiseOffsets[3])
	swayY := ia.perlinNoise(ia.time*ia.headSwayRate*0.8, ia.noiseOffsets[4])

	if swayX > 0 {
		weights.Set(EyeLookOutLeft, clamp(weights.Get(EyeLookOutLeft)+swayX*amp, 0, 1))
		weights.Set(EyeLookInRight, clamp(weights.Get(EyeLookInRight)+swayX*amp, 0, 1))
	} else {
		weights.Set(EyeLookOutRight, clamp(weights.Get(EyeLookOutRight)-swayX*amp, 0, 1))
		weights.Set(EyeLookInLeft, clamp(weights.Get(EyeLookInLeft)-swayX*amp, 0, 1))
	}

	browAsymmetry := ia.perlinNoise(ia.time*ia.headSwayRate*0.6, ia.noiseOffsets[5])
	if browAsymmetry > 0 {
		weights.Set(BrowOuterUpLeft, clamp(weights.Get(BrowOuterUpLeft)+browAsymmetry*amp*2, 0, 1))
	} else {
		weights.Set(BrowOuterUpRight, clamp(weights.Get(BrowOuterUpRight)-browAsymmetry*amp*2, 0, 1))
	}

	_ = swayY
}

func (ia *IdleAnimator) perlinNoise(t, offset float32) float32 {
	t += offset

	n1 := float32(math.Sin(float64(t * 1.0)))
	n2 := float32(math.Sin(float64(t*2.3+1.7))) * 0.5
	n3 := float32(math.Sin(float64(t*4.1+3.2))) * 0.25

	return (n1 + n2 + n3) / 1.75
}

func (ia *IdleAnimator) Reset() {
	ia.mu.Lock()
	defer ia.mu.Unlock()
	ia.time = 0
}

func (ia *IdleAnimator) GetTime() float32 {
	ia.mu.RLock()
	defer ia.mu.RUnlock()
	return ia.time
}

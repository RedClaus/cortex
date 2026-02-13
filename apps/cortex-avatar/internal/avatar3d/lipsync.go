package avatar3d

import (
	"sync"
	"time"
)

type LipSyncController struct {
	mu sync.RWMutex

	visemeQueue   []TimedViseme
	currentViseme VisemeShape
	currentWeight float32

	smoothing float32
	startTime time.Time
}

type TimedViseme struct {
	Shape    VisemeShape
	Weight   float32
	StartAt  time.Time
	Duration time.Duration
}

var visemeToBlendshapes = map[VisemeShape][]BlendshapeMapping{
	VisemeSil: {},
	VisemePP:  {{MouthClose, 0.8}, {MouthPucker, 0.3}},
	VisemeFF:  {{MouthFunnel, 0.5}, {MouthLowerDownLeft, 0.2}, {MouthLowerDownRight, 0.2}},
	VisemeTH:  {{MouthFunnel, 0.3}, {TongueOut, 0.4}},
	VisemeDD:  {{JawOpen, 0.2}, {MouthUpperUpLeft, 0.2}, {MouthUpperUpRight, 0.2}},
	VisemeKK:  {{JawOpen, 0.25}, {MouthStretchLeft, 0.2}, {MouthStretchRight, 0.2}},
	VisemeCH:  {{MouthFunnel, 0.4}, {MouthPucker, 0.3}},
	VisemeSS:  {{MouthStretchLeft, 0.3}, {MouthStretchRight, 0.3}},
	VisemeNN:  {{JawOpen, 0.15}, {MouthClose, 0.3}},
	VisemeRR:  {{MouthPucker, 0.4}, {MouthFunnel, 0.2}},
	VisemeAA:  {{JawOpen, 0.6}, {MouthStretchLeft, 0.2}, {MouthStretchRight, 0.2}},
	VisemeE:   {{JawOpen, 0.3}, {MouthSmileLeft, 0.3}, {MouthSmileRight, 0.3}},
	VisemeI:   {{JawOpen, 0.2}, {MouthSmileLeft, 0.4}, {MouthSmileRight, 0.4}},
	VisemeO:   {{JawOpen, 0.4}, {MouthFunnel, 0.5}, {MouthPucker, 0.3}},
	VisemeU:   {{JawOpen, 0.25}, {MouthPucker, 0.6}, {MouthFunnel, 0.4}},
}

type BlendshapeMapping struct {
	Index  BlendshapeIndex
	Weight float32
}

func NewLipSyncController() *LipSyncController {
	return &LipSyncController{
		visemeQueue:   make([]TimedViseme, 0),
		currentViseme: VisemeSil,
		currentWeight: 0,
		smoothing:     12.0,
		startTime:     time.Now(),
	}
}

func (l *LipSyncController) QueueVisemes(visemes []Viseme) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	l.visemeQueue = l.visemeQueue[:0]

	for _, v := range visemes {
		l.visemeQueue = append(l.visemeQueue, TimedViseme{
			Shape:    v.Shape,
			Weight:   v.Weight,
			StartAt:  now.Add(time.Duration(v.Offset * float32(time.Second))),
			Duration: time.Duration(v.Duration * float32(time.Second)),
		})
	}
}

func (l *LipSyncController) SetVisemeImmediate(shape VisemeShape, weight float32) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.currentViseme = shape
	l.currentWeight = weight
}

func (l *LipSyncController) Update(dt float32, weights *BlendshapeWeights) {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()

	var activeViseme *TimedViseme
	for i := range l.visemeQueue {
		v := &l.visemeQueue[i]
		if now.After(v.StartAt) && now.Before(v.StartAt.Add(v.Duration)) {
			activeViseme = v
			break
		}
	}

	var targetShape VisemeShape = l.currentViseme
	var targetWeight float32 = 0

	if activeViseme != nil {
		targetShape = activeViseme.Shape
		elapsed := now.Sub(activeViseme.StartAt).Seconds()
		duration := activeViseme.Duration.Seconds()
		progress := float32(elapsed / duration)

		envelope := l.computeEnvelope(progress)
		targetWeight = activeViseme.Weight * envelope
	}

	lerpFactor := 1.0 - exp32(-l.smoothing*dt)
	l.currentWeight += (targetWeight - l.currentWeight) * lerpFactor
	
	// Only switch viseme shape if we have a new target or significant weight
	if activeViseme != nil {
		l.currentViseme = targetShape
	}

	l.cleanExpiredVisemes(now)

	l.applyToWeights(weights)
}

func (l *LipSyncController) computeEnvelope(progress float32) float32 {
	attackTime := float32(0.1)
	releaseTime := float32(0.2)

	if progress < attackTime {
		return progress / attackTime
	}
	if progress > 1.0-releaseTime {
		return (1.0 - progress) / releaseTime
	}
	return 1.0
}

func (l *LipSyncController) applyToWeights(weights *BlendshapeWeights) {
	if l.currentWeight < 0.01 {
		return
	}

	mappings, ok := visemeToBlendshapes[l.currentViseme]
	if !ok {
		return
	}

	for _, m := range mappings {
		current := weights.Get(m.Index)
		target := m.Weight * l.currentWeight
		weights.Set(m.Index, clamp(current+target, 0, 1))
	}
}

func (l *LipSyncController) cleanExpiredVisemes(now time.Time) {
	validIdx := 0
	for i := range l.visemeQueue {
		endTime := l.visemeQueue[i].StartAt.Add(l.visemeQueue[i].Duration)
		if now.Before(endTime) {
			l.visemeQueue[validIdx] = l.visemeQueue[i]
			validIdx++
		}
	}
	l.visemeQueue = l.visemeQueue[:validIdx]
}

func (l *LipSyncController) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.visemeQueue = l.visemeQueue[:0]
	l.currentViseme = VisemeSil
	l.currentWeight = 0
}

func (l *LipSyncController) IsSpeaking() bool {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.currentWeight > 0.05
}

func exp32(x float32) float32 {
	if x < -10 {
		return 0
	}
	if x > 10 {
		return 22026.47
	}
	sum := float32(1.0)
	term := float32(1.0)
	for i := 1; i < 12; i++ {
		term *= x / float32(i)
		sum += term
	}
	return sum
}

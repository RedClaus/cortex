package avatar3d

type BlendshapeIndex int

const (
	BrowDownLeft BlendshapeIndex = iota
	BrowDownRight
	BrowInnerUp
	BrowOuterUpLeft
	BrowOuterUpRight
	CheekPuff
	CheekSquintLeft
	CheekSquintRight
	EyeBlinkLeft
	EyeBlinkRight
	EyeLookDownLeft
	EyeLookDownRight
	EyeLookInLeft
	EyeLookInRight
	EyeLookOutLeft
	EyeLookOutRight
	EyeLookUpLeft
	EyeLookUpRight
	EyeSquintLeft
	EyeSquintRight
	EyeWideLeft
	EyeWideRight
	JawForward
	JawLeft
	JawOpen
	JawRight
	MouthClose
	MouthDimpleLeft
	MouthDimpleRight
	MouthFrownLeft
	MouthFrownRight
	MouthFunnel
	MouthLeft
	MouthLowerDownLeft
	MouthLowerDownRight
	MouthPressLeft
	MouthPressRight
	MouthPucker
	MouthRight
	MouthRollLower
	MouthRollUpper
	MouthShrugLower
	MouthShrugUpper
	MouthSmileLeft
	MouthSmileRight
	MouthStretchLeft
	MouthStretchRight
	MouthUpperUpLeft
	MouthUpperUpRight
	NoseSneerLeft
	NoseSneerRight
	TongueOut
	BlendshapeCount
)

var BlendshapeNames = [BlendshapeCount]string{
	"browDownLeft",
	"browDownRight",
	"browInnerUp",
	"browOuterUpLeft",
	"browOuterUpRight",
	"cheekPuff",
	"cheekSquintLeft",
	"cheekSquintRight",
	"eyeBlinkLeft",
	"eyeBlinkRight",
	"eyeLookDownLeft",
	"eyeLookDownRight",
	"eyeLookInLeft",
	"eyeLookInRight",
	"eyeLookOutLeft",
	"eyeLookOutRight",
	"eyeLookUpLeft",
	"eyeLookUpRight",
	"eyeSquintLeft",
	"eyeSquintRight",
	"eyeWideLeft",
	"eyeWideRight",
	"jawForward",
	"jawLeft",
	"jawOpen",
	"jawRight",
	"mouthClose",
	"mouthDimpleLeft",
	"mouthDimpleRight",
	"mouthFrownLeft",
	"mouthFrownRight",
	"mouthFunnel",
	"mouthLeft",
	"mouthLowerDownLeft",
	"mouthLowerDownRight",
	"mouthPressLeft",
	"mouthPressRight",
	"mouthPucker",
	"mouthRight",
	"mouthRollLower",
	"mouthRollUpper",
	"mouthShrugLower",
	"mouthShrugUpper",
	"mouthSmileLeft",
	"mouthSmileRight",
	"mouthStretchLeft",
	"mouthStretchRight",
	"mouthUpperUpLeft",
	"mouthUpperUpRight",
	"noseSneerLeft",
	"noseSneerRight",
	"tongueOut",
}

type BlendshapeWeights [BlendshapeCount]float32

func NewBlendshapeWeights() BlendshapeWeights {
	return BlendshapeWeights{}
}

func (w *BlendshapeWeights) Set(idx BlendshapeIndex, value float32) {
	if value < 0 {
		value = 0
	}
	if value > 1 {
		value = 1
	}
	w[idx] = value
}

func (w *BlendshapeWeights) Get(idx BlendshapeIndex) float32 {
	return w[idx]
}

func (w *BlendshapeWeights) Reset() {
	for i := range w {
		w[i] = 0
	}
}

func (w *BlendshapeWeights) Lerp(target *BlendshapeWeights, t float32) BlendshapeWeights {
	if t <= 0 {
		return *w
	}
	if t >= 1 {
		return *target
	}

	var result BlendshapeWeights
	for i := range w {
		result[i] = w[i] + (target[i]-w[i])*t
	}
	return result
}

func (w *BlendshapeWeights) Add(other *BlendshapeWeights) BlendshapeWeights {
	var result BlendshapeWeights
	for i := range w {
		result[i] = clamp(w[i]+other[i], 0, 1)
	}
	return result
}

func (w *BlendshapeWeights) Scale(factor float32) BlendshapeWeights {
	var result BlendshapeWeights
	for i := range w {
		result[i] = clamp(w[i]*factor, 0, 1)
	}
	return result
}

func (w *BlendshapeWeights) ToSlice() []float32 {
	return w[:]
}

func clamp(v, min, max float32) float32 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func BlendshapeIndexFromName(name string) BlendshapeIndex {
	for i, n := range BlendshapeNames {
		if n == name {
			return BlendshapeIndex(i)
		}
	}
	return -1
}

package avatar3d

type StateMapper struct {
	lastMode CognitiveMode
}

func NewStateMapper() *StateMapper {
	return &StateMapper{
		lastMode: ModeIdle,
	}
}

func (m *StateMapper) MapToExpression(state CortexState) BlendshapeWeights {
	var target BlendshapeWeights

	switch state.Mode {
	case ModeIdle:
		target = PresetNeutral.Weights

	case ModeListening:
		target = PresetNeutral.Weights.Lerp(&PresetAttentive.Weights, 0.6)
		target.Set(BrowOuterUpLeft, 0.05)

	case ModeThinking:
		target = PresetThinking.Weights
		intensity := 0.5 + state.ProcessingLoad*0.5
		target = PresetNeutral.Weights.Lerp(&target, intensity)

	case ModeSpeaking:
		target = PresetNeutral.Weights.Lerp(&PresetConfident.Weights, state.Confidence)

	case ModeAttentive:
		target = PresetAttentive.Weights.Scale(state.AttentionLevel)

	default:
		target = PresetNeutral.Weights
	}

	target = m.applyEmotionalModulation(target, state.Valence, state.Arousal)
	target = m.applyConfidenceModulation(target, state.Confidence)

	m.lastMode = state.Mode

	return target
}

func (m *StateMapper) applyEmotionalModulation(base BlendshapeWeights, valence, arousal float32) BlendshapeWeights {
	result := base

	if valence > 0 {
		smileAmount := valence * 0.3 * (0.5 + arousal*0.5)
		result.Set(MouthSmileLeft, clamp(result.Get(MouthSmileLeft)+smileAmount, 0, 1))
		result.Set(MouthSmileRight, clamp(result.Get(MouthSmileRight)+smileAmount, 0, 1))
		result.Set(CheekSquintLeft, clamp(result.Get(CheekSquintLeft)+smileAmount*0.3, 0, 1))
		result.Set(CheekSquintRight, clamp(result.Get(CheekSquintRight)+smileAmount*0.3, 0, 1))
	}

	if valence < 0 {
		concernAmount := -valence * 0.25
		result.Set(BrowInnerUp, clamp(result.Get(BrowInnerUp)+concernAmount, 0, 1))
		result.Set(MouthFrownLeft, clamp(result.Get(MouthFrownLeft)+concernAmount*0.5, 0, 1))
		result.Set(MouthFrownRight, clamp(result.Get(MouthFrownRight)+concernAmount*0.5, 0, 1))
	}

	if arousal > 0.5 {
		arousalEffect := (arousal - 0.5) * 2 * 0.2
		result.Set(EyeWideLeft, clamp(result.Get(EyeWideLeft)+arousalEffect, 0, 1))
		result.Set(EyeWideRight, clamp(result.Get(EyeWideRight)+arousalEffect, 0, 1))
		result.Set(BrowOuterUpLeft, clamp(result.Get(BrowOuterUpLeft)+arousalEffect*0.5, 0, 1))
		result.Set(BrowOuterUpRight, clamp(result.Get(BrowOuterUpRight)+arousalEffect*0.5, 0, 1))
	}

	if arousal < 0.3 {
		relaxEffect := (0.3 - arousal) / 0.3 * 0.15
		result.Set(EyeSquintLeft, clamp(result.Get(EyeSquintLeft)+relaxEffect, 0, 1))
		result.Set(EyeSquintRight, clamp(result.Get(EyeSquintRight)+relaxEffect, 0, 1))
	}

	return result
}

func (m *StateMapper) applyConfidenceModulation(base BlendshapeWeights, confidence float32) BlendshapeWeights {
	result := base

	if confidence < 0.5 {
		pressAmount := (0.5 - confidence) * 0.4
		result.Set(MouthPressLeft, clamp(result.Get(MouthPressLeft)+pressAmount, 0, 1))
		result.Set(MouthPressRight, clamp(result.Get(MouthPressRight)+pressAmount, 0, 1))
	}

	if confidence > 0.7 {
		confidentAmount := (confidence - 0.7) / 0.3 * 0.15
		result.Set(MouthSmileLeft, clamp(result.Get(MouthSmileLeft)+confidentAmount, 0, 1))
		result.Set(MouthSmileRight, clamp(result.Get(MouthSmileRight)+confidentAmount, 0, 1))
	}

	return result
}

func (m *StateMapper) MapGaze(state CortexState) GazeTarget {
	if state.GazeTarget != nil {
		return GazeTarget{X: state.GazeTarget.X, Y: state.GazeTarget.Y}
	}

	switch state.Mode {
	case ModeThinking:
		return GazeTarget{X: -0.2, Y: 0.3}
	case ModeListening:
		return GazeTarget{X: 0, Y: 0}
	case ModeSpeaking:
		return GazeTarget{X: 0, Y: 0}
	default:
		return GazeTarget{X: 0, Y: 0}
	}
}

func (m *StateMapper) GetModeTransition() (from, to CognitiveMode) {
	return m.lastMode, m.lastMode
}

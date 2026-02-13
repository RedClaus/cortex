package tests

import (
	"testing"

	"github.com/normanking/cortexavatar/internal/avatar3d"
)

func TestBlendshapeWeights(t *testing.T) {
	weights := avatar3d.NewBlendshapeWeights()

	if weights.Get(avatar3d.BrowInnerUp) != 0.0 {
		t.Error("New weights should be zero")
	}

	weights.Set(avatar3d.BrowInnerUp, 0.5)
	if weights.Get(avatar3d.BrowInnerUp) != 0.5 {
		t.Errorf("Expected 0.5, got %f", weights.Get(avatar3d.BrowInnerUp))
	}

	weights.Set(avatar3d.BrowInnerUp, 1.5)
	if weights.Get(avatar3d.BrowInnerUp) != 1.0 {
		t.Error("Weight should be clamped to 1.0")
	}

	weights.Set(avatar3d.BrowInnerUp, -0.5)
	if weights.Get(avatar3d.BrowInnerUp) != 0.0 {
		t.Error("Weight should be clamped to 0.0")
	}
}

func TestBlendshapeWeightsLerp(t *testing.T) {
	a := avatar3d.NewBlendshapeWeights()
	b := avatar3d.NewBlendshapeWeights()

	a.Set(avatar3d.MouthSmileLeft, 0.0)
	b.Set(avatar3d.MouthSmileLeft, 1.0)

	result := a.Lerp(&b, 0.5)
	if result.Get(avatar3d.MouthSmileLeft) != 0.5 {
		t.Errorf("Expected 0.5, got %f", result.Get(avatar3d.MouthSmileLeft))
	}
}

func TestExpressionController(t *testing.T) {
	ctrl := avatar3d.NewExpressionController()

	target := avatar3d.NewBlendshapeWeights()
	target.Set(avatar3d.EyeBlinkLeft, 1.0)
	target.Set(avatar3d.EyeBlinkRight, 1.0)

	ctrl.SetImmediate(target)

	weights := ctrl.GetCurrentWeights()
	if weights.Get(avatar3d.EyeBlinkLeft) != 1.0 {
		t.Errorf("SetImmediate should set weight to 1.0, got %f", weights.Get(avatar3d.EyeBlinkLeft))
	}
}

func TestExpressionPresets(t *testing.T) {
	presets := []avatar3d.ExpressionPreset{
		avatar3d.PresetNeutral,
		avatar3d.PresetHappy,
		avatar3d.PresetSad,
		avatar3d.PresetSurprised,
		avatar3d.PresetThinking,
		avatar3d.PresetAttentive,
		avatar3d.PresetConcerned,
		avatar3d.PresetConfident,
	}

	for _, preset := range presets {
		if preset.Name == "" {
			t.Errorf("Preset has empty name")
		}
	}
}

func TestEyeController(t *testing.T) {
	ctrl := avatar3d.NewEyeController()

	ctrl.LookAt(0.5, 0.3)

	for i := 0; i < 60; i++ {
		weights := avatar3d.NewBlendshapeWeights()
		ctrl.Update(0.016, &weights)
	}

	gaze := ctrl.GetCurrentGaze()
	if gaze.X < 0.4 || gaze.X > 0.6 {
		t.Errorf("Gaze X should be near 0.5, got %f", gaze.X)
	}
}

func TestEyeControllerBlink(t *testing.T) {
	ctrl := avatar3d.NewEyeController()
	ctrl.TriggerBlink()

	if !ctrl.IsBlinking() {
		t.Error("Should be blinking after TriggerBlink")
	}

	for i := 0; i < 30; i++ {
		weights := avatar3d.NewBlendshapeWeights()
		ctrl.Update(0.016, &weights)
	}
}

func TestIdleAnimator(t *testing.T) {
	idle := avatar3d.NewIdleAnimator()

	weights := avatar3d.NewBlendshapeWeights()
	for i := 0; i < 300; i++ {
		idle.Update(0.016, &weights)
	}
}

func TestLipSyncController(t *testing.T) {
	lipSync := avatar3d.NewLipSyncController()

	visemes := []avatar3d.Viseme{
		{Shape: avatar3d.VisemeAA, Weight: 0.8, Duration: 0.15, Offset: 0.0},
		{Shape: avatar3d.VisemeE, Weight: 0.7, Duration: 0.12, Offset: 0.15},
		{Shape: avatar3d.VisemeO, Weight: 0.9, Duration: 0.18, Offset: 0.27},
	}

	lipSync.QueueVisemes(visemes)

	weights := avatar3d.NewBlendshapeWeights()
	for i := 0; i < 60; i++ {
		lipSync.Update(0.016, &weights)
	}
}

func TestStateMapper(t *testing.T) {
	mapper := avatar3d.NewStateMapper()

	state := avatar3d.CortexState{
		Mode:           avatar3d.ModeThinking,
		Valence:        0.5,
		Arousal:        0.3,
		AttentionLevel: 0.7,
		Confidence:     0.8,
	}

	weights := mapper.MapToExpression(state)
	if weights.Get(avatar3d.BrowInnerUp) == 0.0 &&
		weights.Get(avatar3d.MouthSmileLeft) == 0.0 &&
		weights.Get(avatar3d.EyeLookUpLeft) == 0.0 {
		t.Error("MapToExpression should produce some non-zero weights for thinking mode")
	}
}

func TestCortexBridge(t *testing.T) {
	bridge := avatar3d.NewCortexBridge()

	called := false
	bridge.SetOnStateChange(func(state avatar3d.CortexState) {
		called = true
	})

	state := avatar3d.CortexState{
		Mode: avatar3d.ModeListening,
	}
	bridge.UpdateState(state)

	if !called {
		t.Error("OnStateChange callback was not called")
	}
}

func TestCognitiveModes(t *testing.T) {
	modes := []avatar3d.CognitiveMode{
		avatar3d.ModeIdle,
		avatar3d.ModeListening,
		avatar3d.ModeThinking,
		avatar3d.ModeSpeaking,
		avatar3d.ModeAttentive,
	}

	mapper := avatar3d.NewStateMapper()

	for _, mode := range modes {
		state := avatar3d.CortexState{Mode: mode}
		_ = mapper.MapToExpression(state)
	}
}

func TestAvatar(t *testing.T) {
	avatar := avatar3d.NewAvatar(avatar3d.AvatarHannah)

	if avatar.ID != avatar3d.AvatarHannah {
		t.Error("Avatar ID mismatch")
	}

	weights := avatar3d.NewBlendshapeWeights()
	weights.Set(avatar3d.MouthSmileLeft, 0.5)
	avatar.SetBlendshapeWeights(weights)

	avatar.Update(0.016)

	currentWeights := avatar.GetBlendshapeWeights()
	if currentWeights.Get(avatar3d.MouthSmileLeft) == 0.0 {
		t.Error("Avatar should have some smile weight after update")
	}
}

func TestVisemeShapes(t *testing.T) {
	shapes := []avatar3d.VisemeShape{
		avatar3d.VisemeSil,
		avatar3d.VisemePP,
		avatar3d.VisemeFF,
		avatar3d.VisemeTH,
		avatar3d.VisemeDD,
		avatar3d.VisemeKK,
		avatar3d.VisemeCH,
		avatar3d.VisemeSS,
		avatar3d.VisemeNN,
		avatar3d.VisemeRR,
		avatar3d.VisemeAA,
		avatar3d.VisemeE,
		avatar3d.VisemeI,
		avatar3d.VisemeO,
		avatar3d.VisemeU,
	}

	lipSync := avatar3d.NewLipSyncController()

	for _, shape := range shapes {
		viseme := avatar3d.Viseme{Shape: shape, Weight: 1.0, Duration: 0.1, Offset: 0.0}
		lipSync.QueueVisemes([]avatar3d.Viseme{viseme})
		weights := avatar3d.NewBlendshapeWeights()
		lipSync.Update(0.05, &weights)
	}
}

func TestBlendshapeWeightsScale(t *testing.T) {
	weights := avatar3d.NewBlendshapeWeights()
	weights.Set(avatar3d.MouthSmileLeft, 1.0)
	weights.Set(avatar3d.MouthSmileRight, 1.0)

	scaled := weights.Scale(0.5)
	if scaled.Get(avatar3d.MouthSmileLeft) != 0.5 {
		t.Errorf("Expected 0.5 after scaling, got %f", scaled.Get(avatar3d.MouthSmileLeft))
	}
}

func TestBlendshapeWeightsAdd(t *testing.T) {
	a := avatar3d.NewBlendshapeWeights()
	b := avatar3d.NewBlendshapeWeights()

	a.Set(avatar3d.BrowInnerUp, 0.3)
	b.Set(avatar3d.BrowInnerUp, 0.4)

	result := a.Add(&b)
	sum := result.Get(avatar3d.BrowInnerUp)
	if sum < 0.69 || sum > 0.71 {
		t.Errorf("Expected ~0.7 after add, got %f", sum)
	}

	a.Set(avatar3d.EyeWideLeft, 0.8)
	b.Set(avatar3d.EyeWideLeft, 0.5)

	result = a.Add(&b)
	if result.Get(avatar3d.EyeWideLeft) != 1.0 {
		t.Error("Add should clamp to 1.0")
	}
}

func TestExpressionControllerLayers(t *testing.T) {
	ctrl := avatar3d.NewExpressionController()

	baseWeights := avatar3d.NewBlendshapeWeights()
	baseWeights.Set(avatar3d.MouthSmileLeft, 0.5)
	ctrl.SetImmediate(baseWeights)

	layerWeights := avatar3d.NewBlendshapeWeights()
	layerWeights.Set(avatar3d.MouthSmileLeft, 0.3)
	ctrl.AddLayer("extra", layerWeights, 1.0, avatar3d.BlendModeAdditive)

	result := ctrl.Update(0.016)
	if result.Get(avatar3d.MouthSmileLeft) < 0.5 {
		t.Error("Layer should add to base weight")
	}

	ctrl.RemoveLayer("extra")
}

func TestCortexBridgeState(t *testing.T) {
	bridge := avatar3d.NewCortexBridge()

	bridge.SetMode(avatar3d.ModeThinking)
	bridge.SetValence(0.5)
	bridge.SetArousal(0.7)

	state := bridge.GetState()
	if state.Mode != avatar3d.ModeThinking {
		t.Error("Mode should be thinking")
	}
	if state.Valence != 0.5 {
		t.Errorf("Valence should be 0.5, got %f", state.Valence)
	}
	if state.Arousal != 0.7 {
		t.Errorf("Arousal should be 0.7, got %f", state.Arousal)
	}
}

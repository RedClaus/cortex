// internal/renderer/lighting.go
//
// Light definitions and studio lighting setup for avatar rendering
package renderer

import (
	"fmt"

	"github.com/go-gl/mathgl/mgl32"
)

// LightType defines the type of light source
type LightType int

const (
	LightTypePoint LightType = iota
	LightTypeDirectional
	LightTypeArea
	LightTypeSpot
)

// Light represents a light source
type Light struct {
	Type      LightType
	Position  mgl32.Vec3
	Direction mgl32.Vec3
	Color     mgl32.Vec3
	Intensity float32
	AreaSize  mgl32.Vec2 // For area lights
	SpotAngle float32    // For spot lights
}

// LightingRig represents a collection of lights for a scene
type LightingRig struct {
	Lights       []Light
	AmbientColor mgl32.Vec3
}

func NewStudioLighting() *LightingRig {
	return &LightingRig{
		Lights: []Light{
			{
				Type:      LightTypePoint,
				Position:  mgl32.Vec3{1.5, 1.0, 1.5},
				Color:     mgl32.Vec3{1.0, 0.98, 0.95},
				Intensity: 8.0,
			},
			{
				Type:      LightTypePoint,
				Position:  mgl32.Vec3{-1.2, 0.5, 1.2},
				Color:     mgl32.Vec3{0.95, 0.97, 1.0},
				Intensity: 4.0,
			},
			{
				Type:      LightTypePoint,
				Position:  mgl32.Vec3{0, 1.0, -1.0},
				Color:     mgl32.Vec3{1.0, 0.95, 0.9},
				Intensity: 3.0,
			},
		},
		AmbientColor: mgl32.Vec3{0.15, 0.15, 0.18},
	}
}

// NewDramaticLighting creates a more dramatic, high-contrast lighting setup
func NewDramaticLighting() *LightingRig {
	return &LightingRig{
		Lights: []Light{
			// Strong key light from one side
			{
				Type:      LightTypeArea,
				Position:  mgl32.Vec3{2.5, 1.8, 1.5},
				Color:     mgl32.Vec3{1.0, 0.95, 0.85},
				Intensity: 1.2,
				AreaSize:  mgl32.Vec2{0.5, 0.5},
			},
			// Very weak fill
			{
				Type:      LightTypeArea,
				Position:  mgl32.Vec3{-1.5, 0.3, 1.5},
				Color:     mgl32.Vec3{0.8, 0.85, 1.0},
				Intensity: 0.15,
				AreaSize:  mgl32.Vec2{0.8, 0.8},
			},
			// Strong rim for separation
			{
				Type:      LightTypeArea,
				Position:  mgl32.Vec3{-0.5, 2.0, -2.0},
				Color:     mgl32.Vec3{1.0, 0.9, 0.8},
				Intensity: 0.5,
				AreaSize:  mgl32.Vec2{0.3, 0.3},
			},
		},
		AmbientColor: mgl32.Vec3{0.02, 0.02, 0.03},
	}
}

// NewSoftLighting creates soft, even lighting for a friendly appearance
func NewSoftLighting() *LightingRig {
	return &LightingRig{
		Lights: []Light{
			// Large soft key
			{
				Type:      LightTypeArea,
				Position:  mgl32.Vec3{1.0, 1.0, 2.0},
				Color:     mgl32.Vec3{1.0, 1.0, 1.0},
				Intensity: 0.8,
				AreaSize:  mgl32.Vec2{1.5, 1.5},
			},
			// Large fill
			{
				Type:      LightTypeArea,
				Position:  mgl32.Vec3{-1.5, 0.5, 1.5},
				Color:     mgl32.Vec3{1.0, 1.0, 1.0},
				Intensity: 0.5,
				AreaSize:  mgl32.Vec2{1.2, 1.2},
			},
			// Soft rim
			{
				Type:      LightTypeArea,
				Position:  mgl32.Vec3{0, 1.5, -1.5},
				Color:     mgl32.Vec3{1.0, 1.0, 1.0},
				Intensity: 0.3,
				AreaSize:  mgl32.Vec2{1.0, 1.0},
			},
		},
		AmbientColor: mgl32.Vec3{0.05, 0.05, 0.05},
	}
}

// SetLightUniforms sets light uniforms on a shader
func (rig *LightingRig) SetLightUniforms(s *Shader) {
	for i, light := range rig.Lights {
		prefix := fmt.Sprintf("uLights[%d].", i)
		s.SetVec3(prefix+"position", light.Position)
		s.SetVec3(prefix+"color", light.Color)
		s.SetFloat(prefix+"intensity", light.Intensity)
		s.SetInt(prefix+"type", int32(light.Type))

		if light.Type == LightTypeArea {
			s.SetVec2(prefix+"areaSize", light.AreaSize)
		}
		if light.Type == LightTypeDirectional {
			s.SetVec3(prefix+"direction", light.Direction)
		}
	}
	s.SetInt("uLightCount", int32(len(rig.Lights)))
	s.SetVec3("uAmbientColor", rig.AmbientColor)
}

// Temperature presets for color temperature adjustment
var (
	ColorTemp2700K = mgl32.Vec3{1.0, 0.76, 0.55} // Warm incandescent
	ColorTemp3200K = mgl32.Vec3{1.0, 0.84, 0.67} // Tungsten
	ColorTemp4000K = mgl32.Vec3{1.0, 0.91, 0.80} // Neutral warm
	ColorTemp5000K = mgl32.Vec3{1.0, 0.96, 0.91} // Daylight
	ColorTemp5600K = mgl32.Vec3{1.0, 0.98, 0.95} // Daylight balanced
	ColorTemp6500K = mgl32.Vec3{0.95, 0.97, 1.0} // Cool daylight
	ColorTemp7500K = mgl32.Vec3{0.90, 0.94, 1.0} // Overcast
)

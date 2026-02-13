// internal/renderer/camera.go
//
// Camera system with perspective projection for avatar viewing
package renderer

import (
	"math"

	"github.com/go-gl/mathgl/mgl32"
)

// Camera represents a 3D camera
type Camera struct {
	Position mgl32.Vec3
	Target   mgl32.Vec3
	Up       mgl32.Vec3

	// Projection parameters
	FOV         float32
	AspectRatio float32
	NearPlane   float32
	FarPlane    float32

	// Cached matrices
	viewMatrix       mgl32.Mat4
	projectionMatrix mgl32.Mat4
	dirty            bool
}

// NewCamera creates a new camera
func NewCamera(position, target, up mgl32.Vec3, fov, aspect, near, far float32) *Camera {
	c := &Camera{
		Position:    position,
		Target:      target,
		Up:          up,
		FOV:         fov,
		AspectRatio: aspect,
		NearPlane:   near,
		FarPlane:    far,
		dirty:       true,
	}
	c.updateMatrices()
	return c
}

// NewConversationCamera creates a camera optimized for avatar conversation view
// Uses a 24mm equivalent FOV for natural portrait framing
func NewConversationCamera(aspect float32) *Camera {
	return NewCamera(
		mgl32.Vec3{0, 0.1, 1.2}, // Slightly elevated, 1.2m away
		mgl32.Vec3{0, 0, 0},     // Looking at origin (face center)
		mgl32.Vec3{0, 1, 0},     // Up vector
		24.0,                    // 24mm equivalent FOV (portrait lens)
		aspect,
		0.1, 10.0, // Near/far planes
	)
}

// ViewMatrix returns the view matrix
func (c *Camera) ViewMatrix() mgl32.Mat4 {
	if c.dirty {
		c.updateMatrices()
	}
	return c.viewMatrix
}

// ProjectionMatrix returns the projection matrix
func (c *Camera) ProjectionMatrix() mgl32.Mat4 {
	if c.dirty {
		c.updateMatrices()
	}
	return c.projectionMatrix
}

func (c *Camera) updateMatrices() {
	c.viewMatrix = mgl32.LookAtV(c.Position, c.Target, c.Up)
	c.projectionMatrix = mgl32.Perspective(
		mgl32.DegToRad(c.FOV),
		c.AspectRatio,
		c.NearPlane,
		c.FarPlane,
	)
	c.dirty = false
}

// SetPosition updates camera position
func (c *Camera) SetPosition(pos mgl32.Vec3) {
	c.Position = pos
	c.dirty = true
}

// SetTarget updates camera target
func (c *Camera) SetTarget(target mgl32.Vec3) {
	c.Target = target
	c.dirty = true
}

// SetFOV updates field of view
func (c *Camera) SetFOV(fov float32) {
	c.FOV = fov
	c.dirty = true
}

// SetAspectRatio updates aspect ratio
func (c *Camera) SetAspectRatio(aspect float32) {
	c.AspectRatio = aspect
	c.dirty = true
}

// Forward returns the camera's forward direction
func (c *Camera) Forward() mgl32.Vec3 {
	return c.Target.Sub(c.Position).Normalize()
}

// Right returns the camera's right direction
func (c *Camera) Right() mgl32.Vec3 {
	return c.Forward().Cross(c.Up).Normalize()
}

// Orbit rotates the camera around the target
func (c *Camera) Orbit(deltaYaw, deltaPitch float32) {
	// Convert to radians
	yawRad := float64(mgl32.DegToRad(deltaYaw))
	pitchRad := float64(mgl32.DegToRad(deltaPitch))

	// Get current position relative to target
	relPos := c.Position.Sub(c.Target)
	distance := relPos.Len()

	// Convert to spherical coordinates
	theta := math.Atan2(float64(relPos.X()), float64(relPos.Z()))
	phi := math.Acos(float64(relPos.Y()) / float64(distance))

	// Apply rotation
	theta += yawRad
	phi += pitchRad

	// Clamp pitch to avoid gimbal lock
	phi = math.Max(0.1, math.Min(math.Pi-0.1, phi))

	// Convert back to Cartesian
	newPos := mgl32.Vec3{
		float32(math.Sin(phi) * math.Sin(theta)),
		float32(math.Cos(phi)),
		float32(math.Sin(phi) * math.Cos(theta)),
	}.Mul(distance)

	c.Position = c.Target.Add(newPos)
	c.dirty = true
}

// Zoom moves camera toward/away from target
func (c *Camera) Zoom(delta float32) {
	direction := c.Target.Sub(c.Position).Normalize()
	c.Position = c.Position.Add(direction.Mul(delta))

	// Prevent getting too close
	if c.Position.Sub(c.Target).Len() < 0.1 {
		c.Position = c.Target.Add(direction.Mul(-0.1))
	}

	c.dirty = true
}

// Pan moves both position and target
func (c *Camera) Pan(deltaX, deltaY float32) {
	right := c.Right()
	up := c.Up

	offset := right.Mul(deltaX).Add(up.Mul(deltaY))

	c.Position = c.Position.Add(offset)
	c.Target = c.Target.Add(offset)
	c.dirty = true
}

// =============================================================================
// ORBIT CONTROLLER
// =============================================================================

// OrbitController provides mouse-based orbit camera control
type OrbitController struct {
	camera *Camera

	// Sensitivity
	OrbitSensitivity float32
	ZoomSensitivity  float32
	PanSensitivity   float32

	// State
	lastMouseX float32
	lastMouseY float32
	isOrbiting bool
	isPanning  bool
}

// NewOrbitController creates an orbit controller for a camera
func NewOrbitController(camera *Camera) *OrbitController {
	return &OrbitController{
		camera:           camera,
		OrbitSensitivity: 0.5,
		ZoomSensitivity:  0.1,
		PanSensitivity:   0.01,
	}
}

// ProcessMouse handles mouse input
func (oc *OrbitController) ProcessMouse(x, y float32, leftButton, rightButton, middleButton bool) {
	deltaX := x - oc.lastMouseX
	deltaY := y - oc.lastMouseY

	if leftButton {
		if !oc.isOrbiting {
			oc.isOrbiting = true
		} else {
			oc.camera.Orbit(deltaX*oc.OrbitSensitivity, deltaY*oc.OrbitSensitivity)
		}
	} else {
		oc.isOrbiting = false
	}

	if middleButton || rightButton {
		if !oc.isPanning {
			oc.isPanning = true
		} else {
			oc.camera.Pan(-deltaX*oc.PanSensitivity, deltaY*oc.PanSensitivity)
		}
	} else {
		oc.isPanning = false
	}

	oc.lastMouseX = x
	oc.lastMouseY = y
}

// ProcessScroll handles scroll wheel for zoom
func (oc *OrbitController) ProcessScroll(delta float32) {
	oc.camera.Zoom(delta * oc.ZoomSensitivity)
}

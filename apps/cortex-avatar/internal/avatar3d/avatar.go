package avatar3d

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/normanking/cortexavatar/internal/renderer"
)

type AvatarID string

const (
	AvatarHannah AvatarID = "hannah"
	AvatarHenry  AvatarID = "henry"
)

type Avatar struct {
	ID AvatarID

	headMesh *renderer.Mesh

	position mgl32.Vec3
	rotation mgl32.Vec3
	scale    float32

	currentWeights BlendshapeWeights
	targetWeights  BlendshapeWeights

	smoothingFactor float32
}

func (a *Avatar) GetMesh() *renderer.Mesh {
	return a.headMesh
}

func NewAvatar(id AvatarID) *Avatar {
	return &Avatar{
		ID:              id,
		position:        mgl32.Vec3{0, 0, 0},
		rotation:        mgl32.Vec3{0, 0, 0},
		scale:           1.0,
		currentWeights:  NewBlendshapeWeights(),
		targetWeights:   NewBlendshapeWeights(),
		smoothingFactor: 0.15,
	}
}

func (a *Avatar) LoadMesh(path string) error {
	mesh, err := renderer.LoadMeshFromGLTF(path)
	if err != nil {
		return err
	}
	a.headMesh = mesh

	// Build mapping from internal BlendshapeIndex to GLTF MorphTarget index
	mapping := make([]int, BlendshapeCount)
	for i := range mapping {
		mapping[i] = -1
	}

	for i, name := range BlendshapeNames {
		// Simple direct match first
		for ti, target := range mesh.MorphTargets {
			if target.Name == name {
				mapping[i] = ti
				break
			}
		}
		// Try case-insensitive or partial match if needed?
		// For now, strict name matching as per specification.
	}

	mesh.TargetMapping = mapping
	return nil
}

func (a *Avatar) SetPlaceholderMesh() {
	a.headMesh = renderer.NewSphereMesh(32, 16)
}

func (a *Avatar) SetBlendshapeWeight(idx BlendshapeIndex, weight float32) {
	a.targetWeights.Set(idx, weight)
}

func (a *Avatar) SetBlendshapeWeights(weights BlendshapeWeights) {
	a.targetWeights = weights
}

func (a *Avatar) GetBlendshapeWeights() BlendshapeWeights {
	return a.currentWeights
}

func (a *Avatar) Update(dt float32) {
	smoothing := a.smoothingFactor
	if dt > 0 {
		smoothing = 1.0 - pow(1.0-a.smoothingFactor, dt*60)
	}

	a.currentWeights = a.currentWeights.Lerp(&a.targetWeights, smoothing)

	if a.headMesh != nil {
		a.headMesh.ApplyMorphWeights(a.currentWeights.ToSlice())
	}
}

func (a *Avatar) Draw(r *renderer.Renderer, shader *renderer.Shader) {
	if a.headMesh == nil {
		return
	}

	model := mgl32.Translate3D(a.position[0], a.position[1], a.position[2])
	model = model.Mul4(mgl32.HomogRotate3DX(a.rotation[0]))
	model = model.Mul4(mgl32.HomogRotate3DY(a.rotation[1]))
	model = model.Mul4(mgl32.HomogRotate3DZ(a.rotation[2]))
	model = model.Mul4(mgl32.Scale3D(a.scale, a.scale, a.scale))

	shader.SetMat4("uModel", model)

	a.headMesh.Draw()
}

func (a *Avatar) SetPosition(pos mgl32.Vec3) {
	a.position = pos
}

func (a *Avatar) SetRotation(rot mgl32.Vec3) {
	a.rotation = rot
}

func (a *Avatar) SetScale(s float32) {
	a.scale = s
}

func (a *Avatar) Delete() {
	if a.headMesh != nil {
		a.headMesh.Delete()
	}
}

func pow(base, exp float32) float32 {
	if exp == 0 {
		return 1
	}
	result := base
	for i := 1; i < int(exp); i++ {
		result *= base
	}
	return result
}

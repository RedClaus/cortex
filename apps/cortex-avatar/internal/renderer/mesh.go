package renderer

import (
	"fmt"
	"os"
	"unsafe"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/qmuntal/gltf"
)

type Vertex struct {
	Position mgl32.Vec3
	Normal   mgl32.Vec3
	TexCoord mgl32.Vec2
	Tangent  mgl32.Vec3
}

type Mesh struct {
	VAO           uint32
	VBO           uint32
	EBO           uint32
	VertexCount   int32
	IndexCount    int32
	HasIndices    bool
	BaseVertices  []Vertex
	MorphTargets  []MorphTarget
	CurrentDeltas []mgl32.Vec3
	TargetMapping []int
	AlbedoTexture uint32
}

type MorphTarget struct {
	Name           string
	PositionDeltas []mgl32.Vec3
	NormalDeltas   []mgl32.Vec3
}

func LoadMeshFromGLTF(path string) (*Mesh, error) {
	doc, err := gltf.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open gltf: %w", err)
	}

	if len(doc.Meshes) == 0 {
		return nil, fmt.Errorf("no meshes in file")
	}

	gltfMesh := doc.Meshes[0]
	if len(gltfMesh.Primitives) == 0 {
		return nil, fmt.Errorf("no primitives in mesh")
	}

	prim := gltfMesh.Primitives[0]

	mesh := &Mesh{}

	// Load Texture if available
	if prim.Material != nil {
		material := doc.Materials[*prim.Material]
		if material.PBRMetallicRoughness != nil && material.PBRMetallicRoughness.BaseColorTexture != nil {
			texInfo := material.PBRMetallicRoughness.BaseColorTexture
			if texInfo != nil {
				texture := doc.Textures[texInfo.Index]
				if texture.Source != nil {
					image := doc.Images[*texture.Source]
					if image.BufferView != nil {
						bufferView := doc.BufferViews[*image.BufferView]
						buffer := doc.Buffers[bufferView.Buffer]

						data, err := getBufferData(doc, buffer)
						if err == nil {
							// Offset and length within the buffer
							offset := int(bufferView.ByteOffset)
							length := int(bufferView.ByteLength)

							if offset+length <= len(data) {
								imageData := data[offset : offset+length]
								texID, err := CreateTextureFromBytes(imageData)
								if err == nil {
									mesh.AlbedoTexture = texID
									fmt.Printf("Loaded Albedo Texture: ID %d\n", texID)
								} else {
									fmt.Printf("Failed to create texture: %v\n", err)
								}
							}
						}
					}
				}
			}
		}
	}

	posIdx := uint32(prim.Attributes[gltf.POSITION])
	positions, err := readAccessorVec3(doc, posIdx)
	if err != nil {
		return nil, fmt.Errorf("read positions: %w", err)
	}

	var normals []mgl32.Vec3
	if normIdxVal, ok := prim.Attributes[gltf.NORMAL]; ok {
		normals, err = readAccessorVec3(doc, uint32(normIdxVal))
		if err != nil {
			normals = make([]mgl32.Vec3, len(positions))
		}
	} else {
		normals = make([]mgl32.Vec3, len(positions))
	}

	var texCoords []mgl32.Vec2
	if tcIdx, ok := prim.Attributes[gltf.TEXCOORD_0]; ok {
		texCoords, err = readAccessorVec2(doc, uint32(tcIdx))
		if err != nil {
			texCoords = make([]mgl32.Vec2, len(positions))
		}
	} else {
		texCoords = make([]mgl32.Vec2, len(positions))
	}

	var tangents []mgl32.Vec3
	if tanIdx, ok := prim.Attributes[gltf.TANGENT]; ok {
		tangents, err = readAccessorVec3(doc, uint32(tanIdx))
		if err != nil {
			tangents = make([]mgl32.Vec3, len(positions))
		}
	} else {
		tangents = make([]mgl32.Vec3, len(positions))
	}

	mesh.BaseVertices = make([]Vertex, len(positions))
	for i := range positions {
		mesh.BaseVertices[i] = Vertex{
			Position: positions[i],
			Normal:   normals[i],
			TexCoord: texCoords[i],
			Tangent:  tangents[i],
		}
	}

	for i, target := range prim.Targets {
		mt := MorphTarget{Name: fmt.Sprintf("target_%d", i)}

		if posIdx, ok := target[gltf.POSITION]; ok {
			mt.PositionDeltas, _ = readAccessorVec3(doc, uint32(posIdx))
		}
		if normIdx, ok := target[gltf.NORMAL]; ok {
			mt.NormalDeltas, _ = readAccessorVec3(doc, uint32(normIdx))
		}

		mesh.MorphTargets = append(mesh.MorphTargets, mt)
	}

	// Attempt to read target names from extras
	if extras, ok := gltfMesh.Extras.(map[string]interface{}); ok {
		if targetNames, ok := extras["targetNames"].([]interface{}); ok {
			for i, name := range targetNames {
				if i < len(mesh.MorphTargets) {
					if strName, ok := name.(string); ok {
						mesh.MorphTargets[i].Name = strName
					}
				}
			}
		}
	}

	var indices []uint32
	if prim.Indices != nil {
		indices, err = readAccessorIndices(doc, uint32(*prim.Indices))
		if err != nil {
			return nil, fmt.Errorf("read indices: %w", err)
		}
		mesh.HasIndices = true
		mesh.IndexCount = int32(len(indices))
	}

	mesh.VertexCount = int32(len(mesh.BaseVertices))
	mesh.CurrentDeltas = make([]mgl32.Vec3, len(mesh.BaseVertices))

	mesh.uploadToGPU(indices)

	return mesh, nil
}

func (m *Mesh) uploadToGPU(indices []uint32) {
	gl.GenVertexArrays(1, &m.VAO)
	gl.GenBuffers(1, &m.VBO)

	gl.BindVertexArray(m.VAO)
	gl.BindBuffer(gl.ARRAY_BUFFER, m.VBO)

	vertexData := make([]float32, 0, len(m.BaseVertices)*11)
	for _, v := range m.BaseVertices {
		vertexData = append(vertexData, v.Position[0], v.Position[1], v.Position[2])
		vertexData = append(vertexData, v.Normal[0], v.Normal[1], v.Normal[2])
		vertexData = append(vertexData, v.TexCoord[0], v.TexCoord[1])
		vertexData = append(vertexData, v.Tangent[0], v.Tangent[1], v.Tangent[2])
	}

	gl.BufferData(gl.ARRAY_BUFFER, len(vertexData)*4, gl.Ptr(vertexData), gl.DYNAMIC_DRAW)

	stride := int32(11 * 4)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(2, 2, gl.FLOAT, false, stride, 6*4)
	gl.EnableVertexAttribArray(2)
	gl.VertexAttribPointerWithOffset(3, 3, gl.FLOAT, false, stride, 8*4)
	gl.EnableVertexAttribArray(3)

	if m.HasIndices && len(indices) > 0 {
		gl.GenBuffers(1, &m.EBO)
		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, m.EBO)
		gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(indices)*4, gl.Ptr(indices), gl.STATIC_DRAW)
	}

	gl.BindVertexArray(0)
}

func (m *Mesh) ApplyMorphWeights(weights []float32) {
	for i := range m.CurrentDeltas {
		m.CurrentDeltas[i] = mgl32.Vec3{0, 0, 0}
	}

	// Use mapping if available
	if len(m.TargetMapping) > 0 {
		for i, targetIdx := range m.TargetMapping {
			if i >= len(weights) || targetIdx < 0 || targetIdx >= len(m.MorphTargets) {
				continue
			}
			weight := weights[i]
			if weight < 0.001 {
				continue
			}

			target := m.MorphTargets[targetIdx]
			for vi, delta := range target.PositionDeltas {
				if vi < len(m.CurrentDeltas) {
					m.CurrentDeltas[vi] = m.CurrentDeltas[vi].Add(delta.Mul(weight))
				}
			}
		}
	} else {
		// Fallback to direct mapping
		for ti, target := range m.MorphTargets {
			if ti >= len(weights) {
				break
			}
			weight := weights[ti]
			if weight < 0.001 {
				continue
			}

			for vi, delta := range target.PositionDeltas {
				if vi < len(m.CurrentDeltas) {
					m.CurrentDeltas[vi] = m.CurrentDeltas[vi].Add(delta.Mul(weight))
				}
			}
		}
	}

	m.updateVertexBuffer()
}

func (m *Mesh) updateVertexBuffer() {
	vertexData := make([]float32, 0, len(m.BaseVertices)*11)
	for i, v := range m.BaseVertices {
		pos := v.Position.Add(m.CurrentDeltas[i])
		vertexData = append(vertexData, pos[0], pos[1], pos[2])
		vertexData = append(vertexData, v.Normal[0], v.Normal[1], v.Normal[2])
		vertexData = append(vertexData, v.TexCoord[0], v.TexCoord[1])
		vertexData = append(vertexData, v.Tangent[0], v.Tangent[1], v.Tangent[2])
	}

	gl.BindBuffer(gl.ARRAY_BUFFER, m.VBO)
	gl.BufferSubData(gl.ARRAY_BUFFER, 0, len(vertexData)*4, gl.Ptr(vertexData))
}

func (m *Mesh) Draw() {
	gl.BindVertexArray(m.VAO)
	if m.HasIndices {
		gl.DrawElements(gl.TRIANGLES, m.IndexCount, gl.UNSIGNED_INT, nil)
	} else {
		gl.DrawArrays(gl.TRIANGLES, 0, m.VertexCount)
	}
	gl.BindVertexArray(0)
}

func (m *Mesh) Delete() {
	gl.DeleteVertexArrays(1, &m.VAO)
	gl.DeleteBuffers(1, &m.VBO)
	if m.HasIndices {
		gl.DeleteBuffers(1, &m.EBO)
	}
}

func readAccessorVec3(doc *gltf.Document, accessorIdx uint32) ([]mgl32.Vec3, error) {
	accessor := doc.Accessors[accessorIdx]
	bufferView := doc.BufferViews[*accessor.BufferView]
	buffer := doc.Buffers[bufferView.Buffer]

	data, err := getBufferData(doc, buffer)
	if err != nil {
		return nil, err
	}

	offset := int(bufferView.ByteOffset) + int(accessor.ByteOffset)
	count := int(accessor.Count)

	result := make([]mgl32.Vec3, count)
	stride := int(bufferView.ByteStride)
	if stride == 0 {
		stride = 12
	}

	for i := 0; i < count; i++ {
		idx := offset + i*stride
		floats := (*[3]float32)(unsafe.Pointer(&data[idx]))
		result[i] = mgl32.Vec3{floats[0], floats[1], floats[2]}
	}

	return result, nil
}

func readAccessorVec2(doc *gltf.Document, accessorIdx uint32) ([]mgl32.Vec2, error) {
	accessor := doc.Accessors[accessorIdx]
	bufferView := doc.BufferViews[*accessor.BufferView]
	buffer := doc.Buffers[bufferView.Buffer]

	data, err := getBufferData(doc, buffer)
	if err != nil {
		return nil, err
	}

	offset := int(bufferView.ByteOffset) + int(accessor.ByteOffset)
	count := int(accessor.Count)

	result := make([]mgl32.Vec2, count)
	stride := int(bufferView.ByteStride)
	if stride == 0 {
		stride = 8
	}

	for i := 0; i < count; i++ {
		idx := offset + i*stride
		floats := (*[2]float32)(unsafe.Pointer(&data[idx]))
		result[i] = mgl32.Vec2{floats[0], floats[1]}
	}

	return result, nil
}

func readAccessorIndices(doc *gltf.Document, accessorIdx uint32) ([]uint32, error) {
	accessor := doc.Accessors[accessorIdx]
	bufferView := doc.BufferViews[*accessor.BufferView]
	buffer := doc.Buffers[bufferView.Buffer]

	data, err := getBufferData(doc, buffer)
	if err != nil {
		return nil, err
	}

	offset := int(bufferView.ByteOffset) + int(accessor.ByteOffset)
	count := int(accessor.Count)

	result := make([]uint32, count)

	switch accessor.ComponentType {
	case gltf.ComponentUbyte:
		for i := 0; i < count; i++ {
			result[i] = uint32(data[offset+i])
		}
	case gltf.ComponentUshort:
		for i := 0; i < count; i++ {
			idx := offset + i*2
			result[i] = uint32(*(*uint16)(unsafe.Pointer(&data[idx])))
		}
	case gltf.ComponentUint:
		for i := 0; i < count; i++ {
			idx := offset + i*4
			result[i] = *(*uint32)(unsafe.Pointer(&data[idx]))
		}
	}

	return result, nil
}

func getBufferData(doc *gltf.Document, buffer *gltf.Buffer) ([]byte, error) {
	// If URI is empty, the data is likely in the binary chunk (GLB)
	if buffer.URI == "" {
		if len(buffer.Data) > 0 {
			return buffer.Data, nil
		}
		return nil, fmt.Errorf("buffer has no URI and no embedded data")
	}

	if len(buffer.URI) > 5 && buffer.URI[:5] == "data:" {
		return nil, fmt.Errorf("data URI not supported")
	}

	data, err := os.ReadFile(buffer.URI)
	if err != nil {
		return nil, fmt.Errorf("read buffer file: %w", err)
	}

	return data, nil
}

func NewSphereMesh(segments, rings int) *Mesh {
	var vertices []Vertex
	var indices []uint32

	for y := 0; y <= rings; y++ {
		for x := 0; x <= segments; x++ {
			xSeg := float32(x) / float32(segments)
			ySeg := float32(y) / float32(rings)

			xPos := float32(cos(2.0*PI*xSeg) * sin(PI*ySeg))
			yPos := float32(cos(PI * ySeg))
			zPos := float32(sin(2.0*PI*xSeg) * sin(PI*ySeg))

			vertices = append(vertices, Vertex{
				Position: mgl32.Vec3{xPos * 0.15, yPos * 0.15, zPos * 0.15},
				Normal:   mgl32.Vec3{xPos, yPos, zPos},
				TexCoord: mgl32.Vec2{xSeg, ySeg},
				Tangent:  mgl32.Vec3{1, 0, 0},
			})
		}
	}

	for y := 0; y < rings; y++ {
		for x := 0; x < segments; x++ {
			first := uint32(y*(segments+1) + x)
			second := first + uint32(segments+1)

			indices = append(indices, first, second, first+1)
			indices = append(indices, second, second+1, first+1)
		}
	}

	mesh := &Mesh{
		BaseVertices:  vertices,
		HasIndices:    true,
		VertexCount:   int32(len(vertices)),
		IndexCount:    int32(len(indices)),
		CurrentDeltas: make([]mgl32.Vec3, len(vertices)),
	}

	mesh.uploadToGPU(indices)

	return mesh
}

const PI = 3.14159265359

func sin(x float32) float32 {
	return float32(sinApprox(float64(x)))
}

func cos(x float32) float32 {
	return float32(sinApprox(float64(x) + PI/2))
}

func sinApprox(x float64) float64 {
	const pi2 = PI * 2
	for x < 0 {
		x += pi2
	}
	for x >= pi2 {
		x -= pi2
	}
	if x < PI {
		return 16 * x * (PI - x) / (5*PI*PI - 4*x*(PI-x))
	}
	x -= PI
	return -16 * x * (PI - x) / (5*PI*PI - 4*x*(PI-x))
}

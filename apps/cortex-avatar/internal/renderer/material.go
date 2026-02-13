package renderer

import (
	"bytes"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"os"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

type MaterialType int

const (
	MaterialTypeSkin MaterialType = iota
	MaterialTypeEye
	MaterialTypeHair
	MaterialTypeGeneric
)

type Material struct {
	Type MaterialType
	Name string

	Albedo    mgl32.Vec3
	Roughness float32
	Metallic  float32

	SSSWidth     float32
	SSSColor     mgl32.Vec3
	SSSIntensity float32
	SkinTint     mgl32.Vec3

	IrisOffset mgl32.Vec2
	IOR        float32
	PupilSize  float32

	HairColor mgl32.Vec3

	AlbedoTex    uint32
	NormalTex    uint32
	RoughnessTex uint32
	SSSTex       uint32
	AOTex        uint32
	SkinLUTTex   uint32
	IrisTex      uint32
	HairTex      uint32

	hasAlbedo    bool
	hasNormal    bool
	hasRoughness bool
	hasSSS       bool
	hasAO        bool
	hasSkinLUT   bool
	hasIris      bool
	hasHair      bool
}

func NewSkinMaterial(name string) *Material {
	m := &Material{
		Type:         MaterialTypeSkin,
		Name:         name,
		Albedo:       mgl32.Vec3{0.8, 0.6, 0.5},
		Roughness:    0.4,
		Metallic:     0.0,
		SSSWidth:     0.5,
		SSSColor:     mgl32.Vec3{1.0, 0.4, 0.3},
		SSSIntensity: 0.5,
		SkinTint:     mgl32.Vec3{1.0, 1.0, 1.0},
	}
	m.createDefaultTextures()
	return m
}

func NewEyeMaterial(name string) *Material {
	m := &Material{
		Type:       MaterialTypeEye,
		Name:       name,
		Albedo:     mgl32.Vec3{1.0, 1.0, 1.0},
		Roughness:  0.1,
		Metallic:   0.0,
		IrisOffset: mgl32.Vec2{0.0, 0.0},
		IOR:        1.376, // Human cornea index of refraction - physics constant
		PupilSize:  0.3,
	}
	m.createDefaultTextures()
	return m
}

func NewHairMaterial(name string) *Material {
	m := &Material{
		Type:      MaterialTypeHair,
		Name:      name,
		Albedo:    mgl32.Vec3{0.3, 0.2, 0.15},
		Roughness: 0.5,
		Metallic:  0.0,
		HairColor: mgl32.Vec3{0.3, 0.2, 0.15},
	}
	m.createDefaultTextures()
	return m
}

func NewGenericMaterial(name string) *Material {
	m := &Material{
		Type:      MaterialTypeGeneric,
		Name:      name,
		Albedo:    mgl32.Vec3{0.8, 0.8, 0.8},
		Roughness: 0.5,
		Metallic:  0.0,
	}
	m.createDefaultTextures()
	return m
}

func (m *Material) createDefaultTextures() {
	m.AlbedoTex = createSolidTexture(255, 255, 255, 255)
	m.hasAlbedo = true

	m.NormalTex = createSolidTexture(128, 128, 255, 255) // Tangent-space up: (0.5, 0.5, 1.0) encoded
	m.hasNormal = true

	m.RoughnessTex = createSolidTexture(128, 128, 128, 255)
	m.hasRoughness = true

	m.SSSTex = createSolidTexture(255, 255, 255, 255)
	m.hasSSS = true

	m.AOTex = createSolidTexture(255, 255, 255, 255)
	m.hasAO = true
}

func (m *Material) LoadTexture(slot string, path string) error {
	tex, err := loadTextureFromFile(path)
	if err != nil {
		return err
	}

	switch slot {
	case "albedo":
		m.AlbedoTex = tex
		m.hasAlbedo = true
	case "normal":
		m.NormalTex = tex
		m.hasNormal = true
	case "roughness":
		m.RoughnessTex = tex
		m.hasRoughness = true
	case "sss":
		m.SSSTex = tex
		m.hasSSS = true
	case "ao":
		m.AOTex = tex
		m.hasAO = true
	case "skinlut":
		m.SkinLUTTex = tex
		m.hasSkinLUT = true
	case "iris":
		m.IrisTex = tex
		m.hasIris = true
	case "hair":
		m.HairTex = tex
		m.hasHair = true
	}

	return nil
}

func (m *Material) SetAlbedoTexture(tex uint32) {
	m.AlbedoTex = tex
	m.hasAlbedo = true
}

func (m *Material) Bind() {
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, m.AlbedoTex)

	gl.ActiveTexture(gl.TEXTURE1)
	gl.BindTexture(gl.TEXTURE_2D, m.NormalTex)

	gl.ActiveTexture(gl.TEXTURE2)
	gl.BindTexture(gl.TEXTURE_2D, m.RoughnessTex)

	gl.ActiveTexture(gl.TEXTURE3)
	gl.BindTexture(gl.TEXTURE_2D, m.SSSTex)

	gl.ActiveTexture(gl.TEXTURE4)
	gl.BindTexture(gl.TEXTURE_2D, m.AOTex)

	gl.ActiveTexture(gl.TEXTURE5)
	gl.BindTexture(gl.TEXTURE_2D, m.SkinLUTTex)

	if m.Type == MaterialTypeEye && m.hasIris {
		gl.ActiveTexture(gl.TEXTURE6)
		gl.BindTexture(gl.TEXTURE_2D, m.IrisTex)
	}

	if m.Type == MaterialTypeHair && m.hasHair {
		gl.ActiveTexture(gl.TEXTURE6)
		gl.BindTexture(gl.TEXTURE_2D, m.HairTex)
	}
}

func (m *Material) SetUniforms(shader *Shader) {
	shader.SetInt("uAlbedo", 0)
	shader.SetInt("uNormal", 1)
	shader.SetInt("uRoughness", 2)
	shader.SetInt("uSSS", 3)
	shader.SetInt("uAO", 4)
	shader.SetInt("uSkinLUT", 5)

	switch m.Type {
	case MaterialTypeSkin:
		shader.SetFloat("uSSSWidth", m.SSSWidth)
		shader.SetVec3("uSSSColor", m.SSSColor)
		shader.SetFloat("uSSSIntensity", m.SSSIntensity)
		shader.SetFloat("uSkinSmoothness", 1.0-m.Roughness)
		shader.SetVec3("uSkinTint", m.SkinTint)

	case MaterialTypeEye:
		shader.SetInt("uIrisTexture", 6)
		shader.SetVec2("uIrisOffset", m.IrisOffset)
		shader.SetFloat("uIOR", m.IOR)
		shader.SetFloat("uPupilSize", m.PupilSize)

	case MaterialTypeHair:
		shader.SetInt("uHairTexture", 6)
		shader.SetVec3("uHairColor", m.HairColor)
	}
}

func (m *Material) Destroy() {
	if m.hasAlbedo {
		gl.DeleteTextures(1, &m.AlbedoTex)
	}
	if m.hasNormal {
		gl.DeleteTextures(1, &m.NormalTex)
	}
	if m.hasRoughness {
		gl.DeleteTextures(1, &m.RoughnessTex)
	}
	if m.hasSSS {
		gl.DeleteTextures(1, &m.SSSTex)
	}
	if m.hasAO {
		gl.DeleteTextures(1, &m.AOTex)
	}
	if m.hasSkinLUT {
		gl.DeleteTextures(1, &m.SkinLUTTex)
	}
	if m.hasIris {
		gl.DeleteTextures(1, &m.IrisTex)
	}
	if m.hasHair {
		gl.DeleteTextures(1, &m.HairTex)
	}
}

func createSolidTexture(r, g, b, a uint8) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)

	data := []uint8{r, g, b, a}
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA, 1, 1, 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(data))

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)

	return tex
}

func loadTextureFromFile(path string) (uint32, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return 0, err
	}

	return createTextureFromImage(img)
}

func CreateTextureFromBytes(data []byte) (uint32, error) {
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return 0, err
	}
	return createTextureFromImage(img)
}

func createTextureFromImage(img image.Image) (uint32, error) {
	rgba := image.NewRGBA(img.Bounds())
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{}, draw.Src)

	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA,
		int32(rgba.Bounds().Dx()), int32(rgba.Bounds().Dy()),
		0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(rgba.Pix))

	gl.GenerateMipmap(gl.TEXTURE_2D)

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR_MIPMAP_LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.REPEAT)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.REPEAT)

	return tex, nil
}

// GenerateSkinLUT creates pre-integrated skin scattering lookup table.
// Based on Penner & Borshukov "GPU Pro 2" technique for real-time SSS.
func GenerateSkinLUT(width, height int) uint32 {
	var tex uint32
	gl.GenTextures(1, &tex)
	gl.BindTexture(gl.TEXTURE_2D, tex)

	data := make([]float32, width*height*3)

	for y := 0; y < height; y++ {
		curvature := float32(y) / float32(height-1)

		for x := 0; x < width; x++ {
			ndotl := float32(x)/float32(width-1)*2.0 - 1.0
			idx := (y*width + x) * 3

			diffuse := integrateDiffuseScattering(ndotl, curvature)

			data[idx+0] = diffuse[0]
			data[idx+1] = diffuse[1]
			data[idx+2] = diffuse[2]
		}
	}

	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGB16F,
		int32(width), int32(height),
		0, gl.RGB, gl.FLOAT, gl.Ptr(data))

	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	return tex
}

// integrateDiffuseScattering computes skin diffusion using Gaussian sum approximation.
// Variance/weight values from Jensen et al. skin scattering measurements.
// Red light scatters most through skin, blue least (biological property).
func integrateDiffuseScattering(ndotl, curvature float32) [3]float32 {
	var result [3]float32

	variances := [3]float32{0.0064, 0.0484, 0.187}
	weights := [3]float32{0.233, 0.455, 0.649}

	for c := 0; c < 3; c++ {
		wrap := 0.5 + curvature*0.5
		d := clamp((ndotl+wrap)/(1.0+wrap), 0, 1)

		scatter := gaussian(1.0-d, variances[c])

		result[c] = d + scatter*weights[c]*curvature
		result[c] = clamp(result[c], 0, 1)
	}

	return result
}

func gaussian(x, variance float32) float32 {
	return exp(-x*x/(2.0*variance)) / sqrt(2.0*3.14159*variance)
}

func exp(x float32) float32 {
	if x < -10 {
		return 0
	}
	if x > 10 {
		return 22026.47
	}
	return float32(expf64(float64(x)))
}

func sqrt(x float32) float32 {
	return float32(sqrtf64(float64(x)))
}

func expf64(x float64) float64 {
	sum := 1.0
	term := 1.0
	for i := 1; i < 20; i++ {
		term *= x / float64(i)
		sum += term
	}
	return sum
}

func sqrtf64(x float64) float64 {
	if x <= 0 {
		return 0
	}
	guess := x / 2
	for i := 0; i < 10; i++ {
		guess = (guess + x/guess) / 2
	}
	return guess
}

func clamp(x, min, max float32) float32 {
	if x < min {
		return min
	}
	if x > max {
		return max
	}
	return x
}

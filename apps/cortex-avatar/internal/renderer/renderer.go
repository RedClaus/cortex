package renderer

import (
	"fmt"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

type Config struct {
	Width         int
	Height        int
	Title         string
	VSync         bool
	MSAA          int
	TransparentBG bool
	HDR           bool
}

func DefaultConfig() Config {
	return Config{
		Width:         1280,
		Height:        720,
		Title:         "Cortex Avatar 3D",
		VSync:         true,
		MSAA:          4,
		TransparentBG: false,
		HDR:           true,
	}
}

type Renderer struct {
	window *glfw.Window
	config Config

	skinShader  *Shader
	eyeShader   *Shader
	hairShader  *Shader
	postShader  *Shader
	basicShader *Shader

	camera      *Camera
	lightingRig *LightingRig

	hdrFBO      uint32
	hdrColorTex uint32
	hdrDepthTex uint32

	projectionMatrix mgl32.Mat4
	viewMatrix       mgl32.Mat4

	drawCalls int
	triangles int

	cubeVAO uint32
	cubeVBO uint32

	quadVAO uint32

	fbWidth  int
	fbHeight int
}

func New(cfg Config) (*Renderer, error) {
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Resizable, glfw.False)

	if cfg.MSAA > 0 {
		glfw.WindowHint(glfw.Samples, cfg.MSAA)
	}

	if cfg.TransparentBG {
		glfw.WindowHint(glfw.TransparentFramebuffer, glfw.True)
	}

	window, err := glfw.CreateWindow(cfg.Width, cfg.Height, cfg.Title, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create window: %w", err)
	}
	window.MakeContextCurrent()

	if err := gl.Init(); err != nil {
		return nil, fmt.Errorf("gl init: %w", err)
	}

	if cfg.VSync {
		glfw.SwapInterval(1)
	} else {
		glfw.SwapInterval(0)
	}

	r := &Renderer{
		window: window,
		config: cfg,
	}

	fbW, fbH := window.GetFramebufferSize()
	r.fbWidth = fbW
	r.fbHeight = fbH

	if err := r.initShaders(); err != nil {
		return nil, fmt.Errorf("init shaders: %w", err)
	}

	r.initCamera()
	r.initLighting()
	r.initCube()
	r.initQuad()

	// Adjust camera to look at head height
	cam := r.Camera()
	cam.SetPosition(mgl32.Vec3{0, 1.5, 2.0})
	cam.SetTarget(mgl32.Vec3{0, 1.5, 0})

	if cfg.HDR {
		r.initHDRFramebuffer()
	}

	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.Enable(gl.CULL_FACE)
	gl.CullFace(gl.BACK)

	if cfg.MSAA > 0 {
		gl.Enable(gl.MULTISAMPLE)
	}

	return r, nil
}

func (r *Renderer) initShaders() error {
	var err error

	r.skinShader, err = NewShaderFromFiles("assets/shaders/skin.vert", "assets/shaders/skin.frag")
	if err != nil {
		r.skinShader, err = NewShaderFromSource(skinVertSrc, skinFragSrc)
		if err != nil {
			return fmt.Errorf("skin shader: %w", err)
		}
	}

	r.eyeShader, err = NewShaderFromSource(skinVertSrc, eyeFragSrc)
	if err != nil {
		return fmt.Errorf("eye shader: %w", err)
	}

	r.hairShader, err = NewShaderFromSource(skinVertSrc, hairFragSrc)
	if err != nil {
		return fmt.Errorf("hair shader: %w", err)
	}

	r.postShader, err = NewShaderFromSource(postVertSrc, postFragSrc)
	if err != nil {
		return fmt.Errorf("post shader: %w", err)
	}

	r.basicShader, err = NewShaderFromSource(basicVertSrc, basicFragSrc)
	if err != nil {
		return fmt.Errorf("basic shader: %w", err)
	}

	return nil
}

func (r *Renderer) initCamera() {
	r.camera = NewConversationCamera(float32(r.config.Width) / float32(r.config.Height))
}

func (r *Renderer) initLighting() {
	r.lightingRig = NewStudioLighting()
}

func (r *Renderer) initHDRFramebuffer() {
	gl.GenFramebuffers(1, &r.hdrFBO)
	gl.BindFramebuffer(gl.FRAMEBUFFER, r.hdrFBO)

	gl.GenTextures(1, &r.hdrColorTex)
	gl.BindTexture(gl.TEXTURE_2D, r.hdrColorTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F,
		int32(r.fbWidth), int32(r.fbHeight),
		0, gl.RGBA, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, r.hdrColorTex, 0)

	gl.GenTextures(1, &r.hdrDepthTex)
	gl.BindTexture(gl.TEXTURE_2D, r.hdrDepthTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.DEPTH_COMPONENT24,
		int32(r.fbWidth), int32(r.fbHeight),
		0, gl.DEPTH_COMPONENT, gl.FLOAT, nil)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, r.hdrDepthTex, 0)

	if gl.CheckFramebufferStatus(gl.FRAMEBUFFER) != gl.FRAMEBUFFER_COMPLETE {
		panic("HDR framebuffer incomplete")
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

func (r *Renderer) initCube() {
	vertices := []float32{
		// positions          // normals           // colors
		-0.5, -0.5, -0.5, 0, 0, -1, 1, 0, 0,
		0.5, -0.5, -0.5, 0, 0, -1, 1, 0, 0,
		0.5, 0.5, -0.5, 0, 0, -1, 1, 0, 0,
		0.5, 0.5, -0.5, 0, 0, -1, 1, 0, 0,
		-0.5, 0.5, -0.5, 0, 0, -1, 1, 0, 0,
		-0.5, -0.5, -0.5, 0, 0, -1, 1, 0, 0,

		-0.5, -0.5, 0.5, 0, 0, 1, 0, 1, 0,
		0.5, -0.5, 0.5, 0, 0, 1, 0, 1, 0,
		0.5, 0.5, 0.5, 0, 0, 1, 0, 1, 0,
		0.5, 0.5, 0.5, 0, 0, 1, 0, 1, 0,
		-0.5, 0.5, 0.5, 0, 0, 1, 0, 1, 0,
		-0.5, -0.5, 0.5, 0, 0, 1, 0, 1, 0,

		-0.5, 0.5, 0.5, -1, 0, 0, 0, 0, 1,
		-0.5, 0.5, -0.5, -1, 0, 0, 0, 0, 1,
		-0.5, -0.5, -0.5, -1, 0, 0, 0, 0, 1,
		-0.5, -0.5, -0.5, -1, 0, 0, 0, 0, 1,
		-0.5, -0.5, 0.5, -1, 0, 0, 0, 0, 1,
		-0.5, 0.5, 0.5, -1, 0, 0, 0, 0, 1,

		0.5, 0.5, 0.5, 1, 0, 0, 1, 1, 0,
		0.5, 0.5, -0.5, 1, 0, 0, 1, 1, 0,
		0.5, -0.5, -0.5, 1, 0, 0, 1, 1, 0,
		0.5, -0.5, -0.5, 1, 0, 0, 1, 1, 0,
		0.5, -0.5, 0.5, 1, 0, 0, 1, 1, 0,
		0.5, 0.5, 0.5, 1, 0, 0, 1, 1, 0,

		-0.5, -0.5, -0.5, 0, -1, 0, 0, 1, 1,
		0.5, -0.5, -0.5, 0, -1, 0, 0, 1, 1,
		0.5, -0.5, 0.5, 0, -1, 0, 0, 1, 1,
		0.5, -0.5, 0.5, 0, -1, 0, 0, 1, 1,
		-0.5, -0.5, 0.5, 0, -1, 0, 0, 1, 1,
		-0.5, -0.5, -0.5, 0, -1, 0, 0, 1, 1,

		-0.5, 0.5, -0.5, 0, 1, 0, 1, 0, 1,
		0.5, 0.5, -0.5, 0, 1, 0, 1, 0, 1,
		0.5, 0.5, 0.5, 0, 1, 0, 1, 0, 1,
		0.5, 0.5, 0.5, 0, 1, 0, 1, 0, 1,
		-0.5, 0.5, 0.5, 0, 1, 0, 1, 0, 1,
		-0.5, 0.5, -0.5, 0, 1, 0, 1, 0, 1,
	}

	gl.GenVertexArrays(1, &r.cubeVAO)
	gl.GenBuffers(1, &r.cubeVBO)

	gl.BindVertexArray(r.cubeVAO)

	gl.BindBuffer(gl.ARRAY_BUFFER, r.cubeVBO)
	gl.BufferData(gl.ARRAY_BUFFER, len(vertices)*4, gl.Ptr(vertices), gl.STATIC_DRAW)

	stride := int32(9 * 4)
	gl.VertexAttribPointerWithOffset(0, 3, gl.FLOAT, false, stride, 0)
	gl.EnableVertexAttribArray(0)
	gl.VertexAttribPointerWithOffset(1, 3, gl.FLOAT, false, stride, 3*4)
	gl.EnableVertexAttribArray(1)
	gl.VertexAttribPointerWithOffset(2, 3, gl.FLOAT, false, stride, 6*4)
	gl.EnableVertexAttribArray(2)

	gl.BindVertexArray(0)
}

func (r *Renderer) initQuad() {
	gl.GenVertexArrays(1, &r.quadVAO)
}

func (r *Renderer) BeginFrame() {
	r.drawCalls = 0
	r.triangles = 0

	if r.config.HDR {
		gl.BindFramebuffer(gl.FRAMEBUFFER, r.hdrFBO)
	}

	if r.config.TransparentBG {
		gl.ClearColor(0, 0, 0, 0)
	} else {
		gl.ClearColor(0.1, 0.1, 0.12, 1.0)
	}
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)

	r.projectionMatrix = r.camera.ProjectionMatrix()
	r.viewMatrix = r.camera.ViewMatrix()
}

func (r *Renderer) EndFrame() {
	if r.config.HDR {
		gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
		gl.Clear(gl.COLOR_BUFFER_BIT)
		gl.Disable(gl.DEPTH_TEST)

		r.postShader.Use()
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, r.hdrColorTex)
		r.postShader.SetInt("uHDRBuffer", 0)
		r.postShader.SetFloat("uExposure", 1.2)

		r.drawFullscreenQuad()

		gl.Enable(gl.DEPTH_TEST)
	}
}

func (r *Renderer) Present() {
	r.window.SwapBuffers()
	glfw.PollEvents()
}

func (r *Renderer) ShouldClose() bool {
	return r.window.ShouldClose()
}

func (r *Renderer) DrawTestCube(rotation float32) {
	r.basicShader.Use()

	model := mgl32.HomogRotate3D(rotation, mgl32.Vec3{0.5, 1.0, 0.0}.Normalize())
	model = model.Mul4(mgl32.Scale3D(0.3, 0.3, 0.3))

	r.basicShader.SetMat4("uModel", model)
	r.basicShader.SetMat4("uView", r.viewMatrix)
	r.basicShader.SetMat4("uProjection", r.projectionMatrix)
	r.basicShader.SetVec3("uCameraPos", r.camera.Position)
	r.basicShader.SetVec3("uAmbientColor", mgl32.Vec3{0.1, 0.1, 0.12})

	r.lightingRig.SetLightUniforms(r.basicShader)

	gl.BindVertexArray(r.cubeVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 36)
	gl.BindVertexArray(0)

	r.drawCalls++
	r.triangles += 12
}

func (r *Renderer) drawFullscreenQuad() {
	gl.BindVertexArray(r.quadVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 3)
	gl.BindVertexArray(0)
}

func (r *Renderer) GetStats() (drawCalls, triangles int) {
	return r.drawCalls, r.triangles
}

func (r *Renderer) UseSkinShader() *Shader {
	r.skinShader.Use()
	r.skinShader.SetMat4("uProjection", r.projectionMatrix)
	r.skinShader.SetMat4("uView", r.viewMatrix)
	r.skinShader.SetVec3("uCameraPos", r.camera.Position)
	r.lightingRig.SetLightUniforms(r.skinShader)
	return r.skinShader
}

func (r *Renderer) UseEyeShader() *Shader {
	r.eyeShader.Use()
	r.eyeShader.SetMat4("uProjection", r.projectionMatrix)
	r.eyeShader.SetMat4("uView", r.viewMatrix)
	r.eyeShader.SetVec3("uCameraPos", r.camera.Position)
	return r.eyeShader
}

func (r *Renderer) UseHairShader() *Shader {
	r.hairShader.Use()
	r.hairShader.SetMat4("uProjection", r.projectionMatrix)
	r.hairShader.SetMat4("uView", r.viewMatrix)
	r.hairShader.SetVec3("uCameraPos", r.camera.Position)
	return r.hairShader
}

func (r *Renderer) SkinShader() *Shader {
	return r.skinShader
}

func (r *Renderer) SetModelMatrix(model mgl32.Mat4) {
	r.skinShader.SetMat4("uModel", model)
}

func (r *Renderer) Camera() *Camera {
	return r.camera
}

func (r *Renderer) Shutdown() {
	if r.config.HDR {
		gl.DeleteFramebuffers(1, &r.hdrFBO)
		gl.DeleteTextures(1, &r.hdrColorTex)
		gl.DeleteTextures(1, &r.hdrDepthTex)
	}

	gl.DeleteVertexArrays(1, &r.cubeVAO)
	gl.DeleteBuffers(1, &r.cubeVBO)
	gl.DeleteVertexArrays(1, &r.quadVAO)

	r.skinShader.Delete()
	if r.postShader != r.skinShader {
		r.postShader.Delete()
	}

	r.window.Destroy()
}

var skinVertSrc = `#version 410 core

layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aTexCoord;
layout(location = 3) in vec3 aTangent;

out vec3 vPosition;
out vec3 vNormal;
out vec2 vTexCoord;
out vec3 vTangent;
out vec3 vBitangent;

uniform mat4 uModel;
uniform mat4 uView;
uniform mat4 uProjection;

void main() {
    vec4 worldPos = uModel * vec4(aPosition, 1.0);
    vPosition = worldPos.xyz;
    
    mat3 normalMatrix = transpose(inverse(mat3(uModel)));
    vNormal = normalize(normalMatrix * aNormal);
    vTangent = normalize(normalMatrix * aTangent);
    vBitangent = cross(vNormal, vTangent);
    
    vTexCoord = aTexCoord;
    
    gl_Position = uProjection * uView * worldPos;
}
` + "\x00"

var skinFragSrc = `#version 410 core

in vec3 vPosition;
in vec3 vNormal;
in vec2 vTexCoord;
in vec3 vTangent;
in vec3 vBitangent;

out vec4 FragColor;

uniform sampler2D uAlbedo;
uniform sampler2D uNormal;
uniform sampler2D uRoughness;
uniform sampler2D uSSS;

uniform vec3 uCameraPos;

uniform float uSSSWidth;
uniform vec3 uSSSColor;
uniform float uSSSIntensity;

struct Light {
    vec3 position;
    vec3 color;
    float intensity;
    int type;
    vec2 areaSize;
    vec3 direction;
};

#define MAX_LIGHTS 4
uniform Light uLights[MAX_LIGHTS];
uniform int uLightCount;

const float PI = 3.14159265359;

float DistributionGGX(vec3 N, vec3 H, float roughness) {
    float a = roughness * roughness;
    float a2 = a * a;
    float NdotH = max(dot(N, H), 0.0);
    float NdotH2 = NdotH * NdotH;
    
    float num = a2;
    float denom = (NdotH2 * (a2 - 1.0) + 1.0);
    denom = PI * denom * denom;
    
    return num / denom;
}

float GeometrySchlickGGX(float NdotV, float roughness) {
    float r = (roughness + 1.0);
    float k = (r * r) / 8.0;
    return NdotV / (NdotV * (1.0 - k) + k);
}

float GeometrySmith(vec3 N, vec3 V, vec3 L, float roughness) {
    float NdotV = max(dot(N, V), 0.0);
    float NdotL = max(dot(N, L), 0.0);
    return GeometrySchlickGGX(NdotV, roughness) * GeometrySchlickGGX(NdotL, roughness);
}

vec3 fresnelSchlick(float cosTheta, vec3 F0) {
    return F0 + (1.0 - F0) * pow(clamp(1.0 - cosTheta, 0.0, 1.0), 5.0);
}

vec3 subsurfaceScattering(vec3 N, vec3 L, vec3 V, vec3 lightColor, float thickness) {
    vec3 scatterDir = normalize(L + N * uSSSWidth);
    float scatter = pow(clamp(dot(V, -scatterDir), 0.0, 1.0), 2.0);
    return uSSSColor * lightColor * scatter * (1.0 - thickness) * uSSSIntensity;
}

void main() {
    vec3 albedo = pow(texture(uAlbedo, vTexCoord).rgb, vec3(2.2));
    float roughness = texture(uRoughness, vTexCoord).r;
    float sssThickness = texture(uSSS, vTexCoord).r;
    
    vec3 tangentNormal = texture(uNormal, vTexCoord).xyz * 2.0 - 1.0;
    mat3 TBN = mat3(normalize(vTangent), normalize(vBitangent), normalize(vNormal));
    vec3 N = normalize(TBN * tangentNormal);
    
    vec3 V = normalize(uCameraPos - vPosition);
    
    vec3 F0 = vec3(0.028, 0.026, 0.024);
    
    vec3 Lo = vec3(0.0);
    vec3 sss = vec3(0.0);
    
    for (int i = 0; i < uLightCount && i < MAX_LIGHTS; i++) {
        vec3 L = normalize(uLights[i].position - vPosition);
        vec3 H = normalize(V + L);
        float distance = length(uLights[i].position - vPosition);
        float attenuation = 1.0 / (distance * distance);
        vec3 radiance = uLights[i].color * uLights[i].intensity * attenuation;
        
        float NDF = DistributionGGX(N, H, roughness);
        float G = GeometrySmith(N, V, L, roughness);
        vec3 F = fresnelSchlick(max(dot(H, V), 0.0), F0);
        
        vec3 numerator = NDF * G * F;
        float denominator = 4.0 * max(dot(N, V), 0.0) * max(dot(N, L), 0.0) + 0.0001;
        vec3 specular = numerator / denominator;
        
        vec3 kD = vec3(1.0) - F;
        
        float NdotL = max(dot(N, L), 0.0);
        Lo += (kD * albedo / PI + specular) * radiance * NdotL;
        
        sss += subsurfaceScattering(N, L, V, uLights[i].color, sssThickness) 
               * uLights[i].intensity * attenuation;
    }
    
    vec3 ambient = vec3(0.03) * albedo;
    
    vec3 color = ambient + Lo + sss * albedo * 0.5;
    
    FragColor = vec4(color, 1.0);
}
` + "\x00"

var eyeFragSrc = `#version 410 core

in vec3 vPosition;
in vec3 vNormal;
in vec2 vTexCoord;

out vec4 FragColor;

uniform sampler2D uIrisTexture;
uniform vec3 uCameraPos;
uniform vec2 uIrisOffset;
uniform float uIOR;
uniform float uPupilSize;

void main() {
    vec3 N = normalize(vNormal);
    vec3 V = normalize(uCameraPos - vPosition);
    
    vec2 uv = vTexCoord + uIrisOffset * 0.1;
    vec3 irisColor = texture(uIrisTexture, uv).rgb;
    
    float fresnel = pow(1.0 - max(dot(N, V), 0.0), 3.0);
    
    vec3 color = mix(irisColor, vec3(1.0), fresnel * 0.3);
    
    FragColor = vec4(color, 1.0);
}
` + "\x00"

var hairFragSrc = `#version 410 core

in vec3 vPosition;
in vec3 vNormal;
in vec2 vTexCoord;
in vec3 vTangent;

out vec4 FragColor;

uniform sampler2D uHairTexture;
uniform vec3 uCameraPos;
uniform vec3 uHairColor;

float kajiyaKay(vec3 T, vec3 H, float exponent) {
    float TdotH = dot(T, H);
    float sinTH = sqrt(1.0 - TdotH * TdotH);
    return pow(sinTH, exponent);
}

void main() {
    vec3 N = normalize(vNormal);
    vec3 T = normalize(vTangent);
    vec3 V = normalize(uCameraPos - vPosition);
    
    vec3 L = normalize(vec3(2.0, 1.5, 2.0));
    vec3 H = normalize(V + L);
    
    float NdotL = max(dot(N, L), 0.0);
    
    float spec1 = kajiyaKay(T, H, 80.0);
    float spec2 = kajiyaKay(T, H, 20.0);
    
    vec3 hairAlbedo = texture(uHairTexture, vTexCoord).rgb * uHairColor;
    
    vec3 diffuse = hairAlbedo * NdotL;
    vec3 specular = vec3(0.5) * (spec1 * 0.5 + spec2 * 0.2);
    
    vec3 color = diffuse + specular;
    
    float alpha = texture(uHairTexture, vTexCoord).a;
    
    FragColor = vec4(color, alpha);
}
` + "\x00"

var basicVertSrc = `#version 410 core

layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec3 aColor;

out vec3 vPosition;
out vec3 vNormal;
out vec3 vColor;

uniform mat4 uModel;
uniform mat4 uView;
uniform mat4 uProjection;

void main() {
    vec4 worldPos = uModel * vec4(aPosition, 1.0);
    vPosition = worldPos.xyz;
    
    mat3 normalMatrix = transpose(inverse(mat3(uModel)));
    vNormal = normalize(normalMatrix * aNormal);
    vColor = aColor;
    
    gl_Position = uProjection * uView * worldPos;
}
` + "\x00"

var basicFragSrc = `#version 410 core

in vec3 vPosition;
in vec3 vNormal;
in vec3 vColor;

out vec4 FragColor;

uniform vec3 uCameraPos;

struct Light {
    vec3 position;
    vec3 color;
    float intensity;
    int type;
    vec2 areaSize;
    vec3 direction;
};

#define MAX_LIGHTS 4
uniform Light uLights[MAX_LIGHTS];
uniform int uLightCount;
uniform vec3 uAmbientColor;

void main() {
    vec3 N = normalize(vNormal);
    vec3 V = normalize(uCameraPos - vPosition);
    
    vec3 Lo = vec3(0.0);
    
    for (int i = 0; i < uLightCount && i < MAX_LIGHTS; i++) {
        vec3 L = normalize(uLights[i].position - vPosition);
        float distance = length(uLights[i].position - vPosition);
        float attenuation = 1.0 / (distance * distance);
        
        float NdotL = max(dot(N, L), 0.0);
        vec3 diffuse = vColor * NdotL;
        
        vec3 H = normalize(V + L);
        float spec = pow(max(dot(N, H), 0.0), 32.0);
        vec3 specular = vec3(0.3) * spec;
        
        vec3 radiance = uLights[i].color * uLights[i].intensity * attenuation;
        Lo += (diffuse + specular) * radiance;
    }
    
    vec3 ambient = uAmbientColor * vColor;
    vec3 color = ambient + Lo;
    
    FragColor = vec4(color, 1.0);
}
` + "\x00"

var postVertSrc = `#version 410 core

out vec2 vTexCoord;

void main() {
    vec2 positions[3] = vec2[](
        vec2(-1.0, -1.0),
        vec2(3.0, -1.0),
        vec2(-1.0, 3.0)
    );
    
    gl_Position = vec4(positions[gl_VertexID], 0.0, 1.0);
    vTexCoord = (positions[gl_VertexID] + 1.0) * 0.5;
}
` + "\x00"

var postFragSrc = `#version 410 core

in vec2 vTexCoord;
out vec4 FragColor;

uniform sampler2D uHDRBuffer;
uniform float uExposure;

vec3 ACESFilm(vec3 x) {
    float a = 2.51;
    float b = 0.03;
    float c = 2.43;
    float d = 0.59;
    float e = 0.14;
    return clamp((x*(a*x+b))/(x*(c*x+d)+e), 0.0, 1.0);
}

void main() {
    vec3 hdrColor = texture(uHDRBuffer, vTexCoord).rgb;
    vec3 mapped = vec3(1.0) - exp(-hdrColor * uExposure);
    mapped = ACESFilm(mapped);
    mapped = pow(mapped, vec3(1.0/2.2));
    FragColor = vec4(mapped, 1.0);
}
` + "\x00"

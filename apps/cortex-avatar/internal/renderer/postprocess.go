package renderer

import (
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

type PostProcessConfig struct {
	Exposure          float32
	Gamma             float32
	EnableBloom       bool
	BloomIntensity    float32
	BloomThreshold    float32
	EnableFXAA        bool
	EnableVignette    bool
	VignetteIntensity float32
}

func DefaultPostProcessConfig() PostProcessConfig {
	return PostProcessConfig{
		Exposure:          1.0,
		Gamma:             2.2,
		EnableBloom:       false,
		BloomIntensity:    0.3,
		BloomThreshold:    1.0,
		EnableFXAA:        true,
		EnableVignette:    false,
		VignetteIntensity: 0.3,
	}
}

type PostProcessor struct {
	config PostProcessConfig

	hdrFBO      uint32
	hdrColorTex uint32
	hdrDepthTex uint32

	bloomFBO [2]uint32
	bloomTex [2]uint32

	postShader  *Shader
	bloomShader *Shader
	blurShader  *Shader

	quadVAO uint32

	width  int
	height int
}

func NewPostProcessor(width, height int, config PostProcessConfig) (*PostProcessor, error) {
	pp := &PostProcessor{
		config: config,
		width:  width,
		height: height,
	}

	pp.initHDRFramebuffer()
	if config.EnableBloom {
		pp.initBloomFramebuffers()
	}

	var err error
	pp.postShader, err = NewShaderFromSource(postProcessVertSrc, postProcessFragSrc)
	if err != nil {
		return nil, err
	}

	if config.EnableBloom {
		pp.blurShader, err = NewShaderFromSource(postProcessVertSrc, gaussianBlurFragSrc)
		if err != nil {
			return nil, err
		}
		pp.bloomShader, err = NewShaderFromSource(postProcessVertSrc, bloomExtractFragSrc)
		if err != nil {
			return nil, err
		}
	}

	gl.GenVertexArrays(1, &pp.quadVAO)

	return pp, nil
}

func (pp *PostProcessor) initHDRFramebuffer() {
	gl.GenFramebuffers(1, &pp.hdrFBO)
	gl.BindFramebuffer(gl.FRAMEBUFFER, pp.hdrFBO)

	gl.GenTextures(1, &pp.hdrColorTex)
	gl.BindTexture(gl.TEXTURE_2D, pp.hdrColorTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F,
		int32(pp.width), int32(pp.height),
		0, gl.RGBA, gl.FLOAT, nil)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, pp.hdrColorTex, 0)

	gl.GenTextures(1, &pp.hdrDepthTex)
	gl.BindTexture(gl.TEXTURE_2D, pp.hdrDepthTex)
	gl.TexImage2D(gl.TEXTURE_2D, 0, gl.DEPTH_COMPONENT24,
		int32(pp.width), int32(pp.height),
		0, gl.DEPTH_COMPONENT, gl.FLOAT, nil)
	gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.DEPTH_ATTACHMENT, gl.TEXTURE_2D, pp.hdrDepthTex, 0)

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

func (pp *PostProcessor) initBloomFramebuffers() {
	for i := 0; i < 2; i++ {
		gl.GenFramebuffers(1, &pp.bloomFBO[i])
		gl.BindFramebuffer(gl.FRAMEBUFFER, pp.bloomFBO[i])

		gl.GenTextures(1, &pp.bloomTex[i])
		gl.BindTexture(gl.TEXTURE_2D, pp.bloomTex[i])
		gl.TexImage2D(gl.TEXTURE_2D, 0, gl.RGBA16F,
			int32(pp.width/2), int32(pp.height/2),
			0, gl.RGBA, gl.FLOAT, nil)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, gl.LINEAR)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
		gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)
		gl.FramebufferTexture2D(gl.FRAMEBUFFER, gl.COLOR_ATTACHMENT0, gl.TEXTURE_2D, pp.bloomTex[i], 0)
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
}

func (pp *PostProcessor) BeginHDRPass() {
	gl.BindFramebuffer(gl.FRAMEBUFFER, pp.hdrFBO)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
}

func (pp *PostProcessor) EndHDRPass() {
	var bloomTex uint32 = 0

	if pp.config.EnableBloom {
		bloomTex = pp.renderBloom()
	}

	gl.BindFramebuffer(gl.FRAMEBUFFER, 0)
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.Disable(gl.DEPTH_TEST)

	pp.postShader.Use()

	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, pp.hdrColorTex)
	pp.postShader.SetInt("uHDRBuffer", 0)

	if pp.config.EnableBloom && bloomTex != 0 {
		gl.ActiveTexture(gl.TEXTURE1)
		gl.BindTexture(gl.TEXTURE_2D, bloomTex)
		pp.postShader.SetInt("uBloomBuffer", 1)
	}

	pp.postShader.SetFloat("uExposure", pp.config.Exposure)
	pp.postShader.SetFloat("uGamma", pp.config.Gamma)
	pp.postShader.SetBool("uEnableBloom", pp.config.EnableBloom)
	pp.postShader.SetFloat("uBloomIntensity", pp.config.BloomIntensity)
	pp.postShader.SetBool("uEnableFXAA", pp.config.EnableFXAA)
	pp.postShader.SetBool("uEnableVignette", pp.config.EnableVignette)
	pp.postShader.SetFloat("uVignetteIntensity", pp.config.VignetteIntensity)
	pp.postShader.SetVec2("uResolution", mgl32.Vec2{float32(pp.width), float32(pp.height)})

	pp.drawFullscreen()

	gl.Enable(gl.DEPTH_TEST)
}

func (pp *PostProcessor) renderBloom() uint32 {
	gl.Disable(gl.DEPTH_TEST)
	gl.Viewport(0, 0, int32(pp.width/2), int32(pp.height/2))

	gl.BindFramebuffer(gl.FRAMEBUFFER, pp.bloomFBO[0])
	pp.bloomShader.Use()
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, pp.hdrColorTex)
	pp.bloomShader.SetInt("uImage", 0)
	pp.bloomShader.SetFloat("uThreshold", pp.config.BloomThreshold)
	pp.drawFullscreen()

	horizontal := true
	passes := 6
	for i := 0; i < passes; i++ {
		targetFBO := 1
		sourceTex := pp.bloomTex[0]
		if horizontal {
			targetFBO = 1
			sourceTex = pp.bloomTex[0]
		} else {
			targetFBO = 0
			sourceTex = pp.bloomTex[1]
		}

		gl.BindFramebuffer(gl.FRAMEBUFFER, pp.bloomFBO[targetFBO])
		pp.blurShader.Use()
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, sourceTex)
		pp.blurShader.SetInt("uImage", 0)
		pp.blurShader.SetBool("uHorizontal", horizontal)
		pp.drawFullscreen()

		horizontal = !horizontal
	}

	gl.Viewport(0, 0, int32(pp.width), int32(pp.height))
	gl.Enable(gl.DEPTH_TEST)

	if horizontal {
		return pp.bloomTex[0]
	}
	return pp.bloomTex[1]
}

func (pp *PostProcessor) drawFullscreen() {
	gl.BindVertexArray(pp.quadVAO)
	gl.DrawArrays(gl.TRIANGLES, 0, 3)
	gl.BindVertexArray(0)
}

func (pp *PostProcessor) SetExposure(exposure float32) {
	pp.config.Exposure = exposure
}

func (pp *PostProcessor) SetBloomIntensity(intensity float32) {
	pp.config.BloomIntensity = intensity
}

func (pp *PostProcessor) ToggleFXAA(enabled bool) {
	pp.config.EnableFXAA = enabled
}

func (pp *PostProcessor) ToggleVignette(enabled bool) {
	pp.config.EnableVignette = enabled
}

func (pp *PostProcessor) Destroy() {
	gl.DeleteFramebuffers(1, &pp.hdrFBO)
	gl.DeleteTextures(1, &pp.hdrColorTex)
	gl.DeleteTextures(1, &pp.hdrDepthTex)

	if pp.config.EnableBloom {
		gl.DeleteFramebuffers(2, &pp.bloomFBO[0])
		gl.DeleteTextures(2, &pp.bloomTex[0])
	}

	gl.DeleteVertexArrays(1, &pp.quadVAO)

	pp.postShader.Delete()
	if pp.bloomShader != nil {
		pp.bloomShader.Delete()
	}
	if pp.blurShader != nil {
		pp.blurShader.Delete()
	}
}

var postProcessVertSrc = `#version 410 core

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

var postProcessFragSrc = `#version 410 core

in vec2 vTexCoord;
out vec4 FragColor;

uniform sampler2D uHDRBuffer;
uniform sampler2D uBloomBuffer;

uniform float uExposure;
uniform float uGamma;
uniform bool uEnableBloom;
uniform float uBloomIntensity;
uniform bool uEnableFXAA;
uniform bool uEnableVignette;
uniform float uVignetteIntensity;
uniform vec2 uResolution;

vec3 ACESFilm(vec3 x) {
    float a = 2.51;
    float b = 0.03;
    float c = 2.43;
    float d = 0.59;
    float e = 0.14;
    return clamp((x * (a * x + b)) / (x * (c * x + d) + e), 0.0, 1.0);
}

vec3 FXAA(sampler2D tex, vec2 uv, vec2 resolution) {
    vec2 texelSize = 1.0 / resolution;
    
    vec3 rgbNW = texture(tex, uv + vec2(-1.0, -1.0) * texelSize).rgb;
    vec3 rgbNE = texture(tex, uv + vec2(1.0, -1.0) * texelSize).rgb;
    vec3 rgbSW = texture(tex, uv + vec2(-1.0, 1.0) * texelSize).rgb;
    vec3 rgbSE = texture(tex, uv + vec2(1.0, 1.0) * texelSize).rgb;
    vec3 rgbM = texture(tex, uv).rgb;
    
    vec3 luma = vec3(0.299, 0.587, 0.114);
    float lumaNW = dot(rgbNW, luma);
    float lumaNE = dot(rgbNE, luma);
    float lumaSW = dot(rgbSW, luma);
    float lumaSE = dot(rgbSE, luma);
    float lumaM = dot(rgbM, luma);
    
    float lumaMin = min(lumaM, min(min(lumaNW, lumaNE), min(lumaSW, lumaSE)));
    float lumaMax = max(lumaM, max(max(lumaNW, lumaNE), max(lumaSW, lumaSE)));
    
    vec2 dir;
    dir.x = -((lumaNW + lumaNE) - (lumaSW + lumaSE));
    dir.y = ((lumaNW + lumaSW) - (lumaNE + lumaSE));
    
    float dirReduce = max((lumaNW + lumaNE + lumaSW + lumaSE) * 0.0625, 1.0/128.0);
    float rcpDirMin = 1.0 / (min(abs(dir.x), abs(dir.y)) + dirReduce);
    
    dir = min(vec2(8.0, 8.0), max(vec2(-8.0, -8.0), dir * rcpDirMin)) * texelSize;
    
    vec3 rgbA = 0.5 * (
        texture(tex, uv + dir * (1.0/3.0 - 0.5)).rgb +
        texture(tex, uv + dir * (2.0/3.0 - 0.5)).rgb
    );
    
    vec3 rgbB = rgbA * 0.5 + 0.25 * (
        texture(tex, uv + dir * -0.5).rgb +
        texture(tex, uv + dir * 0.5).rgb
    );
    
    float lumaB = dot(rgbB, luma);
    
    if (lumaB < lumaMin || lumaB > lumaMax) {
        return rgbA;
    }
    return rgbB;
}

float vignette(vec2 uv, float intensity) {
    vec2 center = uv - 0.5;
    float dist = length(center);
    return 1.0 - smoothstep(0.3, 0.8, dist) * intensity;
}

void main() {
    vec3 hdrColor;
    
    if (uEnableFXAA) {
        hdrColor = FXAA(uHDRBuffer, vTexCoord, uResolution);
    } else {
        hdrColor = texture(uHDRBuffer, vTexCoord).rgb;
    }
    
    if (uEnableBloom) {
        vec3 bloom = texture(uBloomBuffer, vTexCoord).rgb;
        hdrColor += bloom * uBloomIntensity;
    }
    
    vec3 exposedColor = hdrColor * uExposure;
    vec3 mapped = ACESFilm(exposedColor);
    mapped = pow(mapped, vec3(1.0 / uGamma));
    
    if (uEnableVignette) {
        mapped *= vignette(vTexCoord, uVignetteIntensity);
    }
    
    FragColor = vec4(mapped, 1.0);
}
` + "\x00"

var bloomExtractFragSrc = `#version 410 core

in vec2 vTexCoord;
out vec4 FragColor;

uniform sampler2D uImage;
uniform float uThreshold;

void main() {
    vec3 color = texture(uImage, vTexCoord).rgb;
    float brightness = dot(color, vec3(0.2126, 0.7152, 0.0722));
    
    if (brightness > uThreshold) {
        FragColor = vec4(color, 1.0);
    } else {
        FragColor = vec4(0.0, 0.0, 0.0, 1.0);
    }
}
` + "\x00"

var gaussianBlurFragSrc = `#version 410 core

in vec2 vTexCoord;
out vec4 FragColor;

uniform sampler2D uImage;
uniform bool uHorizontal;

const float weight[5] = float[](0.227027, 0.1945946, 0.1216216, 0.054054, 0.016216);

void main() {
    vec2 texOffset = 1.0 / textureSize(uImage, 0);
    vec3 result = texture(uImage, vTexCoord).rgb * weight[0];
    
    if (uHorizontal) {
        for (int i = 1; i < 5; ++i) {
            result += texture(uImage, vTexCoord + vec2(texOffset.x * float(i), 0.0)).rgb * weight[i];
            result += texture(uImage, vTexCoord - vec2(texOffset.x * float(i), 0.0)).rgb * weight[i];
        }
    } else {
        for (int i = 1; i < 5; ++i) {
            result += texture(uImage, vTexCoord + vec2(0.0, texOffset.y * float(i))).rgb * weight[i];
            result += texture(uImage, vTexCoord - vec2(0.0, texOffset.y * float(i))).rgb * weight[i];
        }
    }
    
    FragColor = vec4(result, 1.0);
}
` + "\x00"

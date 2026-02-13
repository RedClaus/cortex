#version 410 core
//
// postprocess.frag - Post-Processing Effects
//
// Features:
// - ACES Filmic Tone Mapping
// - Exposure control
// - Optional bloom
// - FXAA anti-aliasing
// - Vignette
//

in vec2 vTexCoord;
out vec4 FragColor;

// ============================================================================
// TEXTURES
// ============================================================================

uniform sampler2D uHDRBuffer;         // Main HDR render target
uniform sampler2D uBloomBuffer;       // Bloom texture (if enabled)

// ============================================================================
// UNIFORMS
// ============================================================================

uniform float uExposure;              // Exposure value (default: 1.0)
uniform float uGamma;                 // Gamma correction (default: 2.2)
uniform bool uEnableBloom;            // Enable bloom effect
uniform float uBloomIntensity;        // Bloom strength (default: 0.3)
uniform bool uEnableFXAA;             // Enable FXAA
uniform bool uEnableVignette;         // Enable vignette
uniform float uVignetteIntensity;     // Vignette strength (default: 0.3)
uniform vec2 uResolution;             // Screen resolution

// ============================================================================
// TONE MAPPING
// ============================================================================

// ACES Filmic Tone Mapping
vec3 ACESFilm(vec3 x) {
    float a = 2.51;
    float b = 0.03;
    float c = 2.43;
    float d = 0.59;
    float e = 0.14;
    return clamp((x * (a * x + b)) / (x * (c * x + d) + e), 0.0, 1.0);
}

// Alternative: Reinhard tone mapping
vec3 ReinhardTonemap(vec3 x) {
    return x / (x + vec3(1.0));
}

// Alternative: Uncharted 2 tone mapping
vec3 Uncharted2Tonemap(vec3 x) {
    float A = 0.15;
    float B = 0.50;
    float C = 0.10;
    float D = 0.20;
    float E = 0.02;
    float F = 0.30;
    return ((x * (A * x + C * B) + D * E) / (x * (A * x + B) + D * F)) - E / F;
}

// ============================================================================
// FXAA
// ============================================================================

vec3 FXAA(sampler2D tex, vec2 uv, vec2 resolution) {
    vec2 texelSize = 1.0 / resolution;
    
    // Sample neighbors
    vec3 rgbNW = texture(tex, uv + vec2(-1.0, -1.0) * texelSize).rgb;
    vec3 rgbNE = texture(tex, uv + vec2(1.0, -1.0) * texelSize).rgb;
    vec3 rgbSW = texture(tex, uv + vec2(-1.0, 1.0) * texelSize).rgb;
    vec3 rgbSE = texture(tex, uv + vec2(1.0, 1.0) * texelSize).rgb;
    vec3 rgbM = texture(tex, uv).rgb;
    
    // Luminance calculation
    vec3 luma = vec3(0.299, 0.587, 0.114);
    float lumaNW = dot(rgbNW, luma);
    float lumaNE = dot(rgbNE, luma);
    float lumaSW = dot(rgbSW, luma);
    float lumaSE = dot(rgbSE, luma);
    float lumaM = dot(rgbM, luma);
    
    // Edge detection
    float lumaMin = min(lumaM, min(min(lumaNW, lumaNE), min(lumaSW, lumaSE)));
    float lumaMax = max(lumaM, max(max(lumaNW, lumaNE), max(lumaSW, lumaSE)));
    
    vec2 dir;
    dir.x = -((lumaNW + lumaNE) - (lumaSW + lumaSE));
    dir.y = ((lumaNW + lumaSW) - (lumaNE + lumaSE));
    
    float dirReduce = max((lumaNW + lumaNE + lumaSW + lumaSE) * 0.25 * 0.25, 1.0/128.0);
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

// ============================================================================
// VIGNETTE
// ============================================================================

float vignette(vec2 uv, float intensity) {
    vec2 center = uv - 0.5;
    float dist = length(center);
    return 1.0 - smoothstep(0.3, 0.8, dist) * intensity;
}

// ============================================================================
// MAIN
// ============================================================================

void main() {
    vec3 hdrColor;
    
    // Sample with optional FXAA
    if (uEnableFXAA) {
        hdrColor = FXAA(uHDRBuffer, vTexCoord, uResolution);
    } else {
        hdrColor = texture(uHDRBuffer, vTexCoord).rgb;
    }
    
    // Add bloom if enabled
    if (uEnableBloom) {
        vec3 bloom = texture(uBloomBuffer, vTexCoord).rgb;
        hdrColor += bloom * uBloomIntensity;
    }
    
    // Apply exposure
    vec3 exposedColor = hdrColor * uExposure;
    
    // Tone mapping (ACES Filmic)
    vec3 mapped = ACESFilm(exposedColor);
    
    // Gamma correction
    mapped = pow(mapped, vec3(1.0 / uGamma));
    
    // Apply vignette if enabled
    if (uEnableVignette) {
        mapped *= vignette(vTexCoord, uVignetteIntensity);
    }
    
    FragColor = vec4(mapped, 1.0);
}

#version 410 core
//
// hair.frag - Kajiya-Kay Anisotropic Hair Shader
//
// Based on:
// - Kajiya-Kay model for anisotropic specular highlights
// - Marschner model for dual-specular lobes
// - Strand-space shading for realistic hair appearance
//

// ============================================================================
// INPUTS
// ============================================================================

in vec3 vPosition;
in vec3 vNormal;
in vec2 vTexCoord;
in vec3 vTangent;
in vec3 vBitangent;
in float vDepth;

out vec4 FragColor;

// ============================================================================
// TEXTURES
// ============================================================================

uniform sampler2D uHairTexture;      // Hair color/alpha texture
uniform sampler2D uNoiseTexture;     // Noise for strand variation
uniform sampler2D uShiftTexture;     // Tangent shift texture

// ============================================================================
// UNIFORMS
// ============================================================================

uniform vec3 uCameraPos;
uniform vec3 uHairColor;              // Base hair color
uniform vec3 uSpecularColor1;         // Primary specular (shifted towards tip)
uniform vec3 uSpecularColor2;         // Secondary specular (colored, shifted towards root)

// Kajiya-Kay parameters
uniform float uPrimaryShift;          // Primary highlight shift (default: 0.0)
uniform float uSecondaryShift;        // Secondary highlight shift (default: 0.1)
uniform float uSpecularWidth1;        // Primary specular width (default: 10.0)
uniform float uSpecularWidth2;        // Secondary specular width (default: 20.0)
uniform float uSpecularPower1;        // Primary specular power (default: 80.0)
uniform float uSpecularPower2;        // Secondary specular power (default: 20.0)

// Transparency
uniform float uAlphaCutoff;           // Alpha test threshold (default: 0.5)
uniform float uOpacity;               // Overall opacity (default: 1.0)

// ============================================================================
// LIGHTS
// ============================================================================

struct Light {
    vec3 position;
    vec3 color;
    float intensity;
    vec3 direction;
    int type;             // 0=point, 1=directional
};

#define MAX_LIGHTS 4
uniform Light uLights[MAX_LIGHTS];
uniform int uLightCount;
uniform vec3 uAmbientColor;

// ============================================================================
// KAJIYA-KAY FUNCTIONS
// ============================================================================

// Shift tangent along normal based on texture
vec3 shiftTangent(vec3 T, vec3 N, float shift) {
    return normalize(T + shift * N);
}

// Kajiya-Kay specular term
float kajiyaKaySpecular(vec3 T, vec3 H, float exponent) {
    float TdotH = dot(T, H);
    float sinTH = sqrt(max(0.0, 1.0 - TdotH * TdotH));
    return pow(sinTH, exponent);
}

// Marschner-style dual lobe specular
vec3 hairSpecular(vec3 T, vec3 V, vec3 L, vec3 N, float shiftTex) {
    vec3 H = normalize(V + L);
    
    // Shift tangents for primary and secondary highlights
    vec3 T1 = shiftTangent(T, N, uPrimaryShift + shiftTex);
    vec3 T2 = shiftTangent(T, N, uSecondaryShift + shiftTex);
    
    // Primary specular (sharp, white/hair-colored)
    float spec1 = kajiyaKaySpecular(T1, H, uSpecularPower1);
    spec1 = smoothstep(0.0, 1.0, spec1);
    
    // Secondary specular (broader, colored)
    float spec2 = kajiyaKaySpecular(T2, H, uSpecularPower2);
    spec2 = smoothstep(0.0, 1.0, spec2);
    
    return uSpecularColor1 * spec1 + uSpecularColor2 * spec2;
}

// Strand diffuse (wrapped for softness)
float strandDiffuse(vec3 N, vec3 L) {
    float NdotL = dot(N, L);
    // Wrap lighting for softer falloff on hair strands
    return max(0.0, (NdotL + 0.5) / 1.5);
}

// ============================================================================
// MAIN
// ============================================================================

void main() {
    // Sample textures
    vec4 hairTex = texture(uHairTexture, vTexCoord);
    float shiftTex = texture(uShiftTexture, vTexCoord).r * 2.0 - 1.0;
    
    // Alpha test for hair strand transparency
    float alpha = hairTex.a * uOpacity;
    if (alpha < uAlphaCutoff) {
        discard;
    }
    
    // Hair base color
    vec3 hairAlbedo = hairTex.rgb * uHairColor;
    
    // Normalize vectors
    vec3 N = normalize(vNormal);
    vec3 T = normalize(vTangent);
    vec3 V = normalize(uCameraPos - vPosition);
    
    // Accumulate lighting
    vec3 diffuse = vec3(0.0);
    vec3 specular = vec3(0.0);
    
    for (int i = 0; i < uLightCount && i < MAX_LIGHTS; i++) {
        vec3 L;
        float attenuation;
        
        if (uLights[i].type == 1) {
            // Directional light
            L = normalize(-uLights[i].direction);
            attenuation = 1.0;
        } else {
            // Point light
            vec3 toLight = uLights[i].position - vPosition;
            float distance = length(toLight);
            L = toLight / distance;
            attenuation = 1.0 / (1.0 + distance * distance * 0.1);
        }
        
        vec3 lightColor = uLights[i].color * uLights[i].intensity * attenuation;
        
        // Diffuse
        float diff = strandDiffuse(N, L);
        diffuse += hairAlbedo * diff * lightColor;
        
        // Specular
        vec3 spec = hairSpecular(T, V, L, N, shiftTex);
        specular += spec * lightColor;
    }
    
    // Ambient
    vec3 ambient = uAmbientColor * hairAlbedo * 0.3;
    
    // Rim lighting for strand definition
    float rim = pow(1.0 - max(dot(N, V), 0.0), 4.0);
    vec3 rimColor = vec3(0.2) * rim;
    
    // Final color
    vec3 color = ambient + diffuse + specular * 0.8 + rimColor;
    
    // Depth-based fade for hair tips (optional, for softening)
    float depthFade = smoothstep(0.99, 0.95, vDepth);
    alpha *= depthFade;
    
    FragColor = vec4(color, alpha);
}

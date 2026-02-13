#version 410 core
//
// skin.frag - PBR Skin Shader with Subsurface Scattering
//
// Based on:
// - Cook-Torrance BRDF for specular
// - Pre-integrated SSS (Penner & Borshukov, GPU Pro 2)
// - Physically-based skin F0 values
//

// ============================================================================
// INPUTS
// ============================================================================

in vec3 vPosition;
in vec3 vNormal;
in vec2 vTexCoord;
in vec3 vTangent;
in vec3 vBitangent;

out vec4 FragColor;

// ============================================================================
// TEXTURES
// ============================================================================

uniform sampler2D uAlbedo;           // Base color (sRGB)
uniform sampler2D uNormal;           // Normal map (tangent space)
uniform sampler2D uRoughness;        // Roughness map
uniform sampler2D uSSS;              // Subsurface scattering thickness
uniform sampler2D uAO;               // Ambient occlusion
uniform sampler2D uSkinLUT;          // Pre-integrated skin LUT

// ============================================================================
// UNIFORMS
// ============================================================================

uniform vec3 uCameraPos;

// SSS parameters
uniform float uSSSWidth;             // Scattering width (default: 0.5)
uniform vec3 uSSSColor;              // Scattering color (default: 1.0, 0.4, 0.3)
uniform float uSSSIntensity;         // SSS strength (default: 0.5)

// Skin-specific
uniform float uSkinSmoothness;       // Additional smoothness control
uniform vec3 uSkinTint;              // Color adjustment

// ============================================================================
// LIGHTS
// ============================================================================

struct Light {
    vec3 position;
    vec3 color;
    float intensity;
    vec3 direction;       // For directional lights
    int type;             // 0=point, 1=directional, 2=area
    vec2 areaSize;        // For area lights
};

#define MAX_LIGHTS 4
uniform Light uLights[MAX_LIGHTS];
uniform int uLightCount;

// ============================================================================
// CONSTANTS
// ============================================================================

const float PI = 3.14159265359;

// Skin reflectance (slightly tinted for realism)
const vec3 SKIN_F0 = vec3(0.028, 0.026, 0.024);

// ============================================================================
// NORMAL MAPPING
// ============================================================================

vec3 getNormalFromMap() {
    vec3 tangentNormal = texture(uNormal, vTexCoord).xyz * 2.0 - 1.0;
    
    // Reconstruct TBN matrix
    vec3 T = normalize(vTangent);
    vec3 B = normalize(vBitangent);
    vec3 N = normalize(vNormal);
    mat3 TBN = mat3(T, B, N);
    
    return normalize(TBN * tangentNormal);
}

// ============================================================================
// PBR FUNCTIONS
// ============================================================================

// GGX/Trowbridge-Reitz Normal Distribution Function
float DistributionGGX(vec3 N, vec3 H, float roughness) {
    float a = roughness * roughness;
    float a2 = a * a;
    float NdotH = max(dot(N, H), 0.0);
    float NdotH2 = NdotH * NdotH;
    
    float num = a2;
    float denom = (NdotH2 * (a2 - 1.0) + 1.0);
    denom = PI * denom * denom;
    
    return num / max(denom, 0.0001);
}

// Schlick-GGX Geometry Function
float GeometrySchlickGGX(float NdotV, float roughness) {
    float r = (roughness + 1.0);
    float k = (r * r) / 8.0;
    return NdotV / (NdotV * (1.0 - k) + k);
}

// Smith's Geometry Function
float GeometrySmith(vec3 N, vec3 V, vec3 L, float roughness) {
    float NdotV = max(dot(N, V), 0.0);
    float NdotL = max(dot(N, L), 0.0);
    float ggx2 = GeometrySchlickGGX(NdotV, roughness);
    float ggx1 = GeometrySchlickGGX(NdotL, roughness);
    return ggx1 * ggx2;
}

// Schlick Fresnel Approximation
vec3 fresnelSchlick(float cosTheta, vec3 F0) {
    return F0 + (1.0 - F0) * pow(clamp(1.0 - cosTheta, 0.0, 1.0), 5.0);
}

// ============================================================================
// SUBSURFACE SCATTERING
// ============================================================================

// Pre-integrated skin scattering lookup
vec3 skinScatteringLUT(float NdotL, float curvature) {
    // If LUT texture is available, use it
    // Otherwise, use analytical approximation
    vec2 lutCoord = vec2(NdotL * 0.5 + 0.5, curvature);
    
    // Fallback approximation (Jimenez et al.)
    float scatter = exp(-curvature * curvature * 10.0) * max(0.0, NdotL + 0.5);
    vec3 sssColor = uSSSColor * scatter;
    
    return sssColor;
}

// Simple SSS approximation for thin regions
vec3 subsurfaceScattering(vec3 N, vec3 L, vec3 V, vec3 lightColor, float thickness) {
    // Transmittance through thin surfaces
    vec3 scatterDir = normalize(L + N * uSSSWidth);
    float VdotScatter = pow(clamp(dot(V, -scatterDir), 0.0, 1.0), 2.0);
    
    // Thickness-based attenuation
    float transmittance = exp(-thickness * 3.0);
    
    return uSSSColor * lightColor * VdotScatter * transmittance * uSSSIntensity;
}

// Curvature estimation from screen-space derivatives
float estimateCurvature(vec3 N) {
    // Approximate curvature from normal derivatives
    vec3 dNdx = dFdx(N);
    vec3 dNdy = dFdy(N);
    float curvature = length(dNdx) + length(dNdy);
    return clamp(curvature * 2.0, 0.0, 1.0);
}

// ============================================================================
// MAIN
// ============================================================================

void main() {
    // Sample textures
    vec3 albedo = pow(texture(uAlbedo, vTexCoord).rgb, vec3(2.2)); // sRGB to linear
    float roughness = texture(uRoughness, vTexCoord).r;
    float sssThickness = texture(uSSS, vTexCoord).r;
    float ao = texture(uAO, vTexCoord).r;
    
    // Apply skin-specific adjustments
    roughness = mix(roughness, roughness * 0.8, uSkinSmoothness);
    albedo *= uSkinTint;
    
    // Get normal from normal map
    vec3 N = getNormalFromMap();
    vec3 V = normalize(uCameraPos - vPosition);
    
    // Estimate curvature for SSS
    float curvature = estimateCurvature(N);
    
    // Accumulate lighting
    vec3 Lo = vec3(0.0);
    vec3 sss = vec3(0.0);
    
    for (int i = 0; i < uLightCount; i++) {
        // Light direction and attenuation
        vec3 L;
        float attenuation;
        
        if (uLights[i].type == 1) {
            // Directional light
            L = normalize(-uLights[i].direction);
            attenuation = 1.0;
        } else {
            // Point/Area light
            vec3 toLight = uLights[i].position - vPosition;
            float distance = length(toLight);
            L = toLight / distance;
            attenuation = 1.0 / (distance * distance);
        }
        
        vec3 H = normalize(V + L);
        vec3 radiance = uLights[i].color * uLights[i].intensity * attenuation;
        
        // Cook-Torrance BRDF
        float NDF = DistributionGGX(N, H, roughness);
        float G = GeometrySmith(N, V, L, roughness);
        vec3 F = fresnelSchlick(max(dot(H, V), 0.0), SKIN_F0);
        
        vec3 numerator = NDF * G * F;
        float denominator = 4.0 * max(dot(N, V), 0.0) * max(dot(N, L), 0.0) + 0.0001;
        vec3 specular = numerator / denominator;
        
        // Energy conservation
        vec3 kS = F;
        vec3 kD = vec3(1.0) - kS;
        
        // Diffuse with wrapped NdotL for softer falloff
        float NdotL = max(dot(N, L), 0.0);
        float wrappedNdotL = (NdotL + 0.5) / 1.5; // Wrap lighting for skin
        
        // Standard diffuse + specular
        Lo += (kD * albedo / PI + specular) * radiance * NdotL;
        
        // Add SSS contribution
        vec3 sssContrib = skinScatteringLUT(NdotL, curvature) * radiance;
        sssContrib += subsurfaceScattering(N, L, V, uLights[i].color, sssThickness);
        sss += sssContrib * uLights[i].intensity * attenuation;
    }
    
    // Ambient term
    vec3 ambient = vec3(0.03) * albedo * ao;
    
    // Combine direct lighting and SSS
    vec3 color = ambient + Lo + sss * albedo * 0.5;
    
    // Simple rim lighting for edge definition
    float rim = 1.0 - max(dot(N, V), 0.0);
    rim = pow(rim, 3.0) * 0.15;
    color += vec3(1.0, 0.95, 0.9) * rim;
    
    // Output (will be tone-mapped in post-process)
    FragColor = vec4(color, 1.0);
}

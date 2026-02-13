#version 410 core

in vec3 vPosition;
in vec3 vNormal;
in vec2 vTexCoord;
in vec3 vTangent;
in vec3 vViewDir;

out vec4 FragColor;

uniform sampler2D uIrisTexture;
uniform sampler2D uScleraTexture;

uniform vec3 uCameraPos;
uniform vec2 uIrisOffset;
uniform float uIOR;
uniform float uPupilSize;
uniform vec3 uIrisColor;
uniform float uIrisScale;

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
const float EYE_IOR = 1.376;

vec3 refractRay(vec3 I, vec3 N, float eta) {
    float cosi = clamp(dot(-I, N), -1.0, 1.0);
    float etai = 1.0, etat = eta;
    vec3 n = N;
    
    if (cosi < 0.0) {
        cosi = -cosi;
        n = -N;
        float temp = etai;
        etai = etat;
        etat = temp;
    }
    
    float etaRatio = etai / etat;
    float k = 1.0 - etaRatio * etaRatio * (1.0 - cosi * cosi);
    
    if (k < 0.0) {
        return reflect(I, n);
    }
    
    return etaRatio * I + (etaRatio * cosi - sqrt(k)) * n;
}

float fresnelSchlick(float cosTheta, float F0) {
    return F0 + (1.0 - F0) * pow(clamp(1.0 - cosTheta, 0.0, 1.0), 5.0);
}

void main() {
    vec3 N = normalize(vNormal);
    vec3 V = normalize(vViewDir);
    
    float NdotV = max(dot(N, V), 0.0);
    
    vec3 refracted = refractRay(-V, N, uIOR);
    
    vec2 uvOffset = refracted.xy * 0.1;
    vec2 irisUV = vTexCoord + uIrisOffset * 0.15 + uvOffset;
    
    irisUV = (irisUV - 0.5) * uIrisScale + 0.5;
    
    float distFromCenter = length(irisUV - 0.5) * 2.0;
    
    vec3 irisColor = texture(uIrisTexture, irisUV).rgb * uIrisColor;
    vec3 scleraColor = texture(uScleraTexture, vTexCoord).rgb;
    
    float pupilMask = smoothstep(uPupilSize, uPupilSize + 0.05, distFromCenter);
    irisColor = mix(vec3(0.02), irisColor, pupilMask);
    
    float irisMask = smoothstep(0.45, 0.5, distFromCenter);
    vec3 eyeColor = mix(irisColor, scleraColor, irisMask);
    
    float F0 = 0.04;
    float fresnel = fresnelSchlick(NdotV, F0);
    
    vec3 specular = vec3(0.0);
    for (int i = 0; i < uLightCount && i < MAX_LIGHTS; i++) {
        vec3 L = normalize(uLights[i].position - vPosition);
        vec3 H = normalize(V + L);
        float NdotH = max(dot(N, H), 0.0);
        
        float spec = pow(NdotH, 128.0);
        float dist = length(uLights[i].position - vPosition);
        float attenuation = 1.0 / (dist * dist);
        
        specular += uLights[i].color * spec * uLights[i].intensity * attenuation * 0.5;
    }
    
    vec3 ambient = vec3(0.15);
    
    vec3 corneaReflection = vec3(fresnel * 0.3);
    
    vec3 finalColor = eyeColor * (ambient + 0.5) + specular + corneaReflection;
    
    FragColor = vec4(finalColor, 1.0);
}

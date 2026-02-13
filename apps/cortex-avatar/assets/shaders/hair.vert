#version 410 core
//
// hair.vert - Anisotropic Hair Vertex Shader
//
// Based on Kajiya-Kay model for realistic hair rendering
//

layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aTexCoord;
layout(location = 3) in vec3 aTangent;

out vec3 vPosition;
out vec3 vNormal;
out vec2 vTexCoord;
out vec3 vTangent;
out vec3 vBitangent;
out float vDepth;

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
    
    vec4 clipPos = uProjection * uView * worldPos;
    gl_Position = clipPos;
    vDepth = clipPos.z / clipPos.w;
}

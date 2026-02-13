#version 410 core

layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aTexCoord;
layout(location = 3) in vec3 aTangent;

out vec3 vPosition;
out vec3 vNormal;
out vec2 vTexCoord;
out vec3 vTangent;
out vec3 vViewDir;

uniform mat4 uModel;
uniform mat4 uView;
uniform mat4 uProjection;
uniform vec3 uCameraPos;

void main() {
    vec4 worldPos = uModel * vec4(aPosition, 1.0);
    vPosition = worldPos.xyz;
    
    mat3 normalMatrix = transpose(inverse(mat3(uModel)));
    vNormal = normalize(normalMatrix * aNormal);
    vTangent = normalize(normalMatrix * aTangent);
    
    vTexCoord = aTexCoord;
    vViewDir = normalize(uCameraPos - worldPos.xyz);
    
    gl_Position = uProjection * uView * worldPos;
}

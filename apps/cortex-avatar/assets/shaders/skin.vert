#version 410 core
//
// skin.vert - Standard Vertex Shader with Blendshape Support
//
// Handles:
// - Model/View/Projection transforms
// - Normal matrix for correct lighting
// - Blendshape vertex displacement
// - Tangent space for normal mapping
//

// ============================================================================
// VERTEX ATTRIBUTES
// ============================================================================

layout(location = 0) in vec3 aPosition;
layout(location = 1) in vec3 aNormal;
layout(location = 2) in vec2 aTexCoord;
layout(location = 3) in vec3 aTangent;
layout(location = 4) in vec3 aBitangent;

// Blendshape deltas (passed as additional attributes or from SSBO)
layout(location = 5) in vec3 aBlendPosition;  // Blended position from CPU
layout(location = 6) in vec3 aBlendNormal;    // Blended normal from CPU

// ============================================================================
// OUTPUTS
// ============================================================================

out vec3 vPosition;      // World-space position
out vec3 vNormal;        // World-space normal
out vec2 vTexCoord;      // Texture coordinates
out vec3 vTangent;       // World-space tangent
out vec3 vBitangent;     // World-space bitangent

// ============================================================================
// UNIFORMS
// ============================================================================

uniform mat4 uModel;
uniform mat4 uView;
uniform mat4 uProjection;

// Blendshape mode
// 0 = use aPosition/aNormal directly
// 1 = use aBlendPosition/aBlendNormal (pre-computed on CPU)
// 2 = compute blendshapes in shader (GPU blendshapes)
uniform int uBlendshapeMode;

// GPU blendshape uniforms (when uBlendshapeMode == 2)
#define MAX_BLENDSHAPES 52
uniform float uBlendWeights[MAX_BLENDSHAPES];
uniform int uActiveBlendshapeCount;

// ============================================================================
// BLENDSHAPE SSBO (for GPU blendshapes)
// ============================================================================

// Uncomment for GPU blendshape support (requires OpenGL 4.3+)
// layout(std430, binding = 0) buffer BlendshapeDeltas {
//     vec4 deltas[];  // [vertex_id * MAX_BLENDSHAPES + blendshape_id]
// };

// ============================================================================
// MAIN
// ============================================================================

void main() {
    // Determine vertex position and normal based on blendshape mode
    vec3 position;
    vec3 normal;
    
    if (uBlendshapeMode == 1) {
        // Pre-computed blendshapes (CPU)
        position = aBlendPosition;
        normal = aBlendNormal;
    } else if (uBlendshapeMode == 2) {
        // GPU blendshapes (compute in shader)
        // This requires SSBO support (OpenGL 4.3+)
        position = aPosition;
        normal = aNormal;
        
        // Accumulate blendshape deltas
        // for (int i = 0; i < uActiveBlendshapeCount; i++) {
        //     if (uBlendWeights[i] > 0.001) {
        //         int index = gl_VertexID * MAX_BLENDSHAPES + i;
        //         vec4 delta = deltas[index];
        //         position += delta.xyz * uBlendWeights[i];
        //     }
        // }
    } else {
        // No blendshapes
        position = aPosition;
        normal = aNormal;
    }
    
    // Transform to world space
    vec4 worldPos = uModel * vec4(position, 1.0);
    vPosition = worldPos.xyz;
    
    // Normal matrix (inverse transpose of model matrix)
    mat3 normalMatrix = transpose(inverse(mat3(uModel)));
    vNormal = normalize(normalMatrix * normal);
    vTangent = normalize(normalMatrix * aTangent);
    
    // Re-orthogonalize tangent (Gram-Schmidt)
    vTangent = normalize(vTangent - dot(vTangent, vNormal) * vNormal);
    
    // Calculate bitangent
    vBitangent = cross(vNormal, vTangent);
    
    // Pass through texture coordinates
    vTexCoord = aTexCoord;
    
    // Final clip-space position
    gl_Position = uProjection * uView * worldPos;
}

#version 410 core
//
// postprocess.vert - Fullscreen quad vertex shader
//

out vec2 vTexCoord;

void main() {
    // Generate fullscreen triangle (more efficient than quad)
    vec2 positions[3] = vec2[](
        vec2(-1.0, -1.0),
        vec2(3.0, -1.0),
        vec2(-1.0, 3.0)
    );
    
    gl_Position = vec4(positions[gl_VertexID], 0.0, 1.0);
    vTexCoord = (positions[gl_VertexID] + 1.0) * 0.5;
}

// internal/renderer/shader.go
//
// Shader compilation, management, and hot-reload support
package renderer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fsnotify/fsnotify"
	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/mathgl/mgl32"
)

// Shader represents a compiled OpenGL shader program
type Shader struct {
	ID uint32

	// Source paths for hot-reload
	vertPath string
	fragPath string

	// Uniform location cache
	uniformCache map[string]int32
	mu           sync.RWMutex
}

// NewShaderFromFiles loads and compiles shaders from files
func NewShaderFromFiles(vertPath, fragPath string) (*Shader, error) {
	vertSrc, err := os.ReadFile(vertPath)
	if err != nil {
		return nil, fmt.Errorf("read vertex shader %s: %w", vertPath, err)
	}

	fragSrc, err := os.ReadFile(fragPath)
	if err != nil {
		return nil, fmt.Errorf("read fragment shader %s: %w", fragPath, err)
	}

	// Ensure null termination
	vertStr := string(vertSrc)
	if !strings.HasSuffix(vertStr, "\x00") {
		vertStr += "\x00"
	}

	fragStr := string(fragSrc)
	if !strings.HasSuffix(fragStr, "\x00") {
		fragStr += "\x00"
	}

	shader, err := NewShaderFromSource(vertStr, fragStr)
	if err != nil {
		return nil, err
	}

	shader.vertPath = vertPath
	shader.fragPath = fragPath

	return shader, nil
}

// NewShaderFromSource compiles shaders from source strings
func NewShaderFromSource(vertSrc, fragSrc string) (*Shader, error) {
	// Compile vertex shader
	vertShader, err := compileShader(vertSrc, gl.VERTEX_SHADER)
	if err != nil {
		return nil, fmt.Errorf("vertex shader: %w", err)
	}
	defer gl.DeleteShader(vertShader)

	// Compile fragment shader
	fragShader, err := compileShader(fragSrc, gl.FRAGMENT_SHADER)
	if err != nil {
		return nil, fmt.Errorf("fragment shader: %w", err)
	}
	defer gl.DeleteShader(fragShader)

	// Link program
	program := gl.CreateProgram()
	gl.AttachShader(program, vertShader)
	gl.AttachShader(program, fragShader)
	gl.LinkProgram(program)

	// Check link status
	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return nil, fmt.Errorf("link failed: %s", log)
	}

	return &Shader{
		ID:           program,
		uniformCache: make(map[string]int32),
	}, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csource, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csource, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		typeName := "vertex"
		if shaderType == gl.FRAGMENT_SHADER {
			typeName = "fragment"
		}

		return 0, fmt.Errorf("%s compile error: %s", typeName, log)
	}

	return shader, nil
}

// Use activates this shader program
func (s *Shader) Use() {
	gl.UseProgram(s.ID)
}

// Delete releases shader resources
func (s *Shader) Delete() {
	gl.DeleteProgram(s.ID)
}

// Reload recompiles the shader from its source files
func (s *Shader) Reload() error {
	if s.vertPath == "" || s.fragPath == "" {
		return fmt.Errorf("shader was not loaded from files")
	}

	newShader, err := NewShaderFromFiles(s.vertPath, s.fragPath)
	if err != nil {
		return err
	}

	// Swap program ID
	oldID := s.ID
	s.ID = newShader.ID

	// Clear uniform cache
	s.mu.Lock()
	s.uniformCache = make(map[string]int32)
	s.mu.Unlock()

	// Delete old program
	gl.DeleteProgram(oldID)

	return nil
}

// getUniformLocation returns cached uniform location
func (s *Shader) getUniformLocation(name string) int32 {
	s.mu.RLock()
	if loc, ok := s.uniformCache[name]; ok {
		s.mu.RUnlock()
		return loc
	}
	s.mu.RUnlock()

	s.mu.Lock()
	defer s.mu.Unlock()

	loc := gl.GetUniformLocation(s.ID, gl.Str(name+"\x00"))
	s.uniformCache[name] = loc
	return loc
}

// SetBool sets a boolean uniform
func (s *Shader) SetBool(name string, value bool) {
	var v int32
	if value {
		v = 1
	}
	gl.Uniform1i(s.getUniformLocation(name), v)
}

// SetInt sets an integer uniform
func (s *Shader) SetInt(name string, value int32) {
	gl.Uniform1i(s.getUniformLocation(name), value)
}

// SetFloat sets a float uniform
func (s *Shader) SetFloat(name string, value float32) {
	gl.Uniform1f(s.getUniformLocation(name), value)
}

// SetVec2 sets a vec2 uniform
func (s *Shader) SetVec2(name string, v mgl32.Vec2) {
	gl.Uniform2fv(s.getUniformLocation(name), 1, &v[0])
}

// SetVec3 sets a vec3 uniform
func (s *Shader) SetVec3(name string, v mgl32.Vec3) {
	gl.Uniform3fv(s.getUniformLocation(name), 1, &v[0])
}

// SetVec4 sets a vec4 uniform
func (s *Shader) SetVec4(name string, v mgl32.Vec4) {
	gl.Uniform4fv(s.getUniformLocation(name), 1, &v[0])
}

// SetMat3 sets a mat3 uniform
func (s *Shader) SetMat3(name string, m mgl32.Mat3) {
	gl.UniformMatrix3fv(s.getUniformLocation(name), 1, false, &m[0])
}

// SetMat4 sets a mat4 uniform
func (s *Shader) SetMat4(name string, m mgl32.Mat4) {
	gl.UniformMatrix4fv(s.getUniformLocation(name), 1, false, &m[0])
}

// SetFloatArray sets a float array uniform
func (s *Shader) SetFloatArray(name string, values []float32) {
	if len(values) > 0 {
		gl.Uniform1fv(s.getUniformLocation(name), int32(len(values)), &values[0])
	}
}

// SetVec3Array sets a vec3 array uniform
func (s *Shader) SetVec3Array(name string, values []mgl32.Vec3) {
	if len(values) > 0 {
		// Flatten to float32 slice
		flat := make([]float32, len(values)*3)
		for i, v := range values {
			flat[i*3] = v[0]
			flat[i*3+1] = v[1]
			flat[i*3+2] = v[2]
		}
		gl.Uniform3fv(s.getUniformLocation(name), int32(len(values)), &flat[0])
	}
}

// =============================================================================
// SHADER HOT-RELOAD WATCHER
// =============================================================================

// ShaderWatcher watches shader files for changes and reloads them
type ShaderWatcher struct {
	watcher *fsnotify.Watcher
	shaders map[string]*Shader // path -> shader
	mu      sync.RWMutex
	done    chan struct{}
}

// NewShaderWatcher creates a new shader watcher
func NewShaderWatcher() (*ShaderWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	sw := &ShaderWatcher{
		watcher: watcher,
		shaders: make(map[string]*Shader),
		done:    make(chan struct{}),
	}

	go sw.watchLoop()

	return sw, nil
}

// Watch adds a shader to be watched for changes
func (sw *ShaderWatcher) Watch(shader *Shader) error {
	if shader.vertPath == "" || shader.fragPath == "" {
		return fmt.Errorf("shader was not loaded from files")
	}

	sw.mu.Lock()
	defer sw.mu.Unlock()

	// Watch vertex shader directory
	vertDir := filepath.Dir(shader.vertPath)
	if err := sw.watcher.Add(vertDir); err != nil {
		return err
	}

	// Watch fragment shader directory (if different)
	fragDir := filepath.Dir(shader.fragPath)
	if fragDir != vertDir {
		if err := sw.watcher.Add(fragDir); err != nil {
			return err
		}
	}

	sw.shaders[shader.vertPath] = shader
	sw.shaders[shader.fragPath] = shader

	return nil
}

func (sw *ShaderWatcher) watchLoop() {
	for {
		select {
		case <-sw.done:
			return
		case event, ok := <-sw.watcher.Events:
			if !ok {
				return
			}
			if event.Has(fsnotify.Write) {
				sw.mu.RLock()
				if shader, ok := sw.shaders[event.Name]; ok {
					// Reload on next frame (defer to main thread)
					go func(s *Shader) {
						if err := s.Reload(); err != nil {
							fmt.Printf("Shader reload failed: %v\n", err)
						} else {
							fmt.Printf("Shader reloaded: %s\n", event.Name)
						}
					}(shader)
				}
				sw.mu.RUnlock()
			}
		case err, ok := <-sw.watcher.Errors:
			if !ok {
				return
			}
			fmt.Printf("Shader watcher error: %v\n", err)
		}
	}
}

// Close stops the shader watcher
func (sw *ShaderWatcher) Close() error {
	close(sw.done)
	return sw.watcher.Close()
}

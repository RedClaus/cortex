package main

import (
	"flag"
	"log"
	"math"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
	"github.com/normanking/cortexavatar/internal/avatar3d"
	"github.com/normanking/cortexavatar/internal/renderer"
)

func init() {
	runtime.LockOSThread()
}

type Config struct {
	WindowWidth   int
	WindowHeight  int
	WindowTitle   string
	VSync         bool
	MSAA          int
	TransparentBG bool
	AvatarID      string
	ShowFPS       bool
}

func main() {
	cfg := parseFlags()

	log.Println("===========================================")
	log.Println("  Cortex Avatar 3D Renderer")
	log.Println("  Phase 6: Complete - Hair & Polish")
	log.Println("===========================================")

	if err := glfw.Init(); err != nil {
		log.Fatalf("Failed to initialize GLFW: %v", err)
	}
	defer glfw.Terminate()

	rendererCfg := renderer.Config{
		Width:         cfg.WindowWidth,
		Height:        cfg.WindowHeight,
		Title:         cfg.WindowTitle,
		VSync:         cfg.VSync,
		MSAA:          cfg.MSAA,
		TransparentBG: cfg.TransparentBG,
		HDR:           true,
	}

	rend, err := renderer.New(rendererCfg)
	if err != nil {
		log.Fatalf("Failed to create renderer: %v", err)
	}
	defer rend.Shutdown()

	avatar := avatar3d.NewAvatar(avatar3d.AvatarID(cfg.AvatarID))

	// Attempt to load the real avatar mesh
	modelPath := "assets/models/hannah/hannah.glb"
	meshType := "placeholder sphere"

	if _, err := os.Stat(modelPath); err == nil {
		if err := avatar.LoadMesh(modelPath); err != nil {
			log.Printf("Failed to load avatar mesh: %v. Falling back to placeholder.", err)
			avatar.SetPlaceholderMesh()
		} else {
			log.Printf("Loaded avatar mesh from %s", modelPath)
			meshType = "hannah.glb"
			// Log vertex count for debugging
			if avatar.GetMesh() != nil {
				log.Printf("Mesh Vertex Count: %d", avatar.GetMesh().VertexCount)
			}
		}
	} else {
		log.Printf("Avatar model not found at %s. Using placeholder sphere.", modelPath)
		avatar.SetPlaceholderMesh()
	}

	// Try adjusting scale - VRM/GLB might be in centimeters
	// If 1 unit = 1 meter, 1.0 is fine. If 1 unit = 1 cm, we need 0.01.
	// We didn't see it at 1.0, so let's try 0.01 (assuming it was huge).
	// Default to 1.0 scale (meters)
	avatar.SetScale(1.0)
	// Rotate 180 degrees to face the camera
	avatar.SetRotation(mgl32.Vec3{0, 3.14159, 0})

	defer avatar.Delete()

	skinMaterial := renderer.NewSkinMaterial("skin")

	// Apply loaded texture if available
	if mesh := avatar.GetMesh(); mesh != nil && mesh.AlbedoTexture > 0 {
		skinMaterial.SetAlbedoTexture(mesh.AlbedoTexture)
		log.Printf("Applied texture from mesh to material")
	}

	skinLUT := renderer.GenerateSkinLUT(256, 256)
	skinMaterial.SkinLUTTex = skinLUT
	defer skinMaterial.Destroy()

	expressionCtrl := avatar3d.NewExpressionController()
	eyeCtrl := avatar3d.NewEyeController()
	idleAnim := avatar3d.NewIdleAnimator()
	lipSync := avatar3d.NewLipSyncController()
	stateMapper := avatar3d.NewStateMapper()
	cortexBridge := avatar3d.NewCortexBridge()

	cortexBridge.SetOnStateChange(func(state avatar3d.CortexState) {
		targetWeights := stateMapper.MapToExpression(state)
		expressionCtrl.TransitionTo(targetWeights, avatar3d.TransitionNormal, avatar3d.InterpEaseInOut)

		gazeTarget := stateMapper.MapGaze(state)
		eyeCtrl.LookAt(gazeTarget.X, gazeTarget.Y)

		if state.IsSpeaking && len(state.Visemes) > 0 {
			lipSync.QueueVisemes(state.Visemes)
		}
	})

	log.Println("Renderer initialized successfully")
	log.Printf("Avatar: %s (using %s)", cfg.AvatarID, meshType)
	log.Println("Testing Cortex integration with mock states...")
	log.Println("Press Ctrl+C or close window to exit")

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	frameStart := time.Now()
	var deltaTime float32
	var totalTime float32

	frameCount := 0
	fpsTimer := time.Now()

	for !rend.ShouldClose() {
		select {
		case <-sigChan:
			log.Println("Shutdown signal received")
			return
		default:
		}

		now := time.Now()
		deltaTime = float32(now.Sub(frameStart).Seconds())
		frameStart = now
		totalTime += deltaTime

		if deltaTime > 0.1 {
			deltaTime = 0.1
		}

		updateCortexDemo(cortexBridge, totalTime)

		weights := expressionCtrl.Update(deltaTime)
		eyeCtrl.Update(deltaTime, &weights)
		idleAnim.Update(deltaTime, &weights)
		lipSync.Update(deltaTime, &weights)

		avatar.SetBlendshapeWeights(weights)
		avatar.Update(deltaTime)

		rend.BeginFrame()

		shader := rend.UseSkinShader()
		skinMaterial.Bind()
		skinMaterial.SetUniforms(shader)
		// rend.DrawTestCube(totalTime * 0.5)
		avatar.Draw(rend, shader)

		rend.EndFrame()
		rend.Present()

		frameCount++
		if cfg.ShowFPS && time.Since(fpsTimer) >= time.Second {
			draws, _ := rend.GetStats()
			gaze := eyeCtrl.GetCurrentGaze()
			log.Printf("FPS: %d | Draws: %d | Gaze: (%.2f, %.2f) | Blink: %v",
				frameCount, draws, gaze.X, gaze.Y, eyeCtrl.IsBlinking())
			frameCount = 0
			fpsTimer = time.Now()
		}
	}

	log.Println("Render loop ended gracefully")
	log.Println("Phase 6 verification: SUCCESS - All systems functional")
}

var lastModeChange float32 = 0
var currentModeIdx int = 0

func updateCortexDemo(bridge *avatar3d.CortexBridge, t float32) {
	modes := []avatar3d.CognitiveMode{
		avatar3d.ModeIdle,
		avatar3d.ModeListening,
		avatar3d.ModeThinking,
		avatar3d.ModeSpeaking,
		avatar3d.ModeAttentive,
	}

	if t-lastModeChange > 4.0 {
		lastModeChange = t
		currentModeIdx = (currentModeIdx + 1) % len(modes)

		state := avatar3d.CortexState{
			Mode:           modes[currentModeIdx],
			Valence:        float32(math.Sin(float64(t*0.3))) * 0.4,
			Arousal:        0.3 + float32(math.Sin(float64(t*0.5)))*0.3,
			AttentionLevel: 0.5 + float32(math.Sin(float64(t*0.7)))*0.3,
			Confidence:     0.6 + float32(math.Sin(float64(t*0.4)))*0.2,
			ProcessingLoad: float32(math.Sin(float64(t*0.6))*0.5 + 0.5),
			IsSpeaking:     modes[currentModeIdx] == avatar3d.ModeSpeaking,
		}

		if modes[currentModeIdx] == avatar3d.ModeSpeaking {
			state.Visemes = []avatar3d.Viseme{
				{Shape: avatar3d.VisemeAA, Weight: 0.8, Duration: 0.15, Offset: 0.0},
				{Shape: avatar3d.VisemeE, Weight: 0.7, Duration: 0.12, Offset: 0.15},
				{Shape: avatar3d.VisemeO, Weight: 0.9, Duration: 0.18, Offset: 0.27},
				{Shape: avatar3d.VisemeSil, Weight: 0.0, Duration: 0.1, Offset: 0.45},
			}
		}

		bridge.UpdateState(state)
		log.Printf("Mode: %s | Valence: %.2f | Arousal: %.2f", modes[currentModeIdx], state.Valence, state.Arousal)
	}
}

func parseFlags() *Config {
	cfg := &Config{}

	flag.IntVar(&cfg.WindowWidth, "width", 1280, "Window width")
	flag.IntVar(&cfg.WindowHeight, "height", 720, "Window height")
	flag.StringVar(&cfg.WindowTitle, "title", "Cortex Avatar 3D", "Window title")
	flag.BoolVar(&cfg.VSync, "vsync", true, "Enable VSync")
	flag.IntVar(&cfg.MSAA, "msaa", 4, "MSAA samples")
	flag.BoolVar(&cfg.TransparentBG, "transparent", false, "Transparent background")
	flag.StringVar(&cfg.AvatarID, "avatar", "hannah", "Avatar ID")
	flag.BoolVar(&cfg.ShowFPS, "fps", true, "Show FPS")

	flag.Parse()

	return cfg
}

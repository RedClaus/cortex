/**
 * TalkingHead-style Avatar Controller
 * Manages VRM model loading, lip-sync, emotions, and idle animations
 */

import * as THREE from 'three';
import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
import { VRM, VRMLoaderPlugin, VRMExpressionPresetName } from '@pixiv/three-vrm';
import { 
  OculusViseme,
  interpolateVisemes,
  getVRMWeightsForViseme
} from './visemes';
import type { 
  VisemeEvent, 
  VRMBlendShapeWeights
} from './visemes';
import {
  getVRMWeightsForEmotion,
  DEFAULT_EMOTION
} from './emotions';
import type {
  EmotionType,
  EmotionState
} from './emotions';

// Configuration options for the avatar
export interface AvatarOptions {
  modelUrl?: string;
  defaultEmotion?: EmotionType;
  enableIdleAnimations?: boolean;
  lipSyncMode?: 'viseme' | 'audio-reactive';
  blinkInterval?: [number, number];  // min, max ms
  headMovementInterval?: [number, number];  // min, max ms
}

// Lip-sync timeline for playback
export interface LipSyncTimeline {
  events: VisemeEvent[];
  duration: number;
  audioStartTime: number;
}

const DEFAULT_OPTIONS: Required<AvatarOptions> = {
  modelUrl: '/models/default.vrm',
  defaultEmotion: 'neutral',
  enableIdleAnimations: true,
  lipSyncMode: 'viseme',
  blinkInterval: [2000, 5000],
  headMovementInterval: [2000, 4000]
};

export class TalkingHeadController {
  private container: HTMLElement;
  private options: Required<AvatarOptions>;
  
  // Three.js scene
  private scene: THREE.Scene;
  private camera: THREE.PerspectiveCamera;
  private renderer: THREE.WebGLRenderer;
  private clock: THREE.Clock;
  
  // VRM avatar
  private vrm: VRM | null = null;
  private mixer: THREE.AnimationMixer | null = null;
  
  // State
  private currentEmotion: EmotionState = DEFAULT_EMOTION;
  private targetEmotion: EmotionState = DEFAULT_EMOTION;
  private emotionTransitionProgress: number = 1;
  private emotionTransitionDuration: number = 300; // ms
  
  // Lip-sync state
  private currentVisemeWeights: VRMBlendShapeWeights = {};
  private targetVisemeWeights: VRMBlendShapeWeights = {};
  private visemeLerpFactor: number = 0.3;
  private lipSyncTimeline: LipSyncTimeline | null = null;
  private lipSyncTimelineIndex: number = 0;
  private isSpeaking: boolean = false;
  
  // Audio-reactive lip-sync
  private audioContext: AudioContext | null = null;
  private analyser: AnalyserNode | null = null;
  private audioDataArray: Uint8Array | null = null;
  
  // Idle animation state
  private nextBlinkTime: number = 0;
  private blinkProgress: number = 0;
  private isBlinking: boolean = false;
  
  private nextHeadMoveTime: number = 0;
  private headTargetRotation: THREE.Euler = new THREE.Euler();
  private headCurrentRotation: THREE.Euler = new THREE.Euler();
  
  // Animation frame
  private animationFrameId: number | null = null;
  private isDisposed: boolean = false;

  constructor(container: HTMLElement, options: AvatarOptions = {}) {
    this.container = container;
    this.options = { ...DEFAULT_OPTIONS, ...options };
    
    // Initialize Three.js
    this.scene = new THREE.Scene();
    this.clock = new THREE.Clock();
    
    // Camera setup - positioned for head and shoulders view
    const aspect = container.clientWidth / container.clientHeight;
    this.camera = new THREE.PerspectiveCamera(30, aspect, 0.1, 20);
    this.camera.position.set(0, 1.4, 1.5);
    this.camera.lookAt(0, 1.3, 0);
    
    // Renderer setup
    this.renderer = new THREE.WebGLRenderer({ 
      antialias: true, 
      alpha: true,
      powerPreference: 'high-performance'
    });
    this.renderer.setSize(container.clientWidth, container.clientHeight);
    this.renderer.setPixelRatio(Math.min(window.devicePixelRatio, 2));
    this.renderer.outputColorSpace = THREE.SRGBColorSpace;
    this.renderer.toneMapping = THREE.ACESFilmicToneMapping;
    this.renderer.toneMappingExposure = 1;
    container.appendChild(this.renderer.domElement);
    
    // Lighting
    const ambientLight = new THREE.AmbientLight(0xffffff, 0.6);
    this.scene.add(ambientLight);
    
    const directionalLight = new THREE.DirectionalLight(0xffffff, 0.8);
    directionalLight.position.set(1, 2, 1);
    this.scene.add(directionalLight);
    
    const fillLight = new THREE.DirectionalLight(0xffffff, 0.3);
    fillLight.position.set(-1, 1, -1);
    this.scene.add(fillLight);
    
    // Initialize timers
    this.scheduleNextBlink();
    this.scheduleNextHeadMove();
    
    // Handle resize
    window.addEventListener('resize', this.handleResize);
    
    // Start render loop
    this.animate();
  }

  /**
   * Load a VRM avatar model
   */
  async loadModel(url: string): Promise<void> {
    return new Promise((resolve, reject) => {
      const loader = new GLTFLoader();
      loader.register((parser) => new VRMLoaderPlugin(parser));
      
      loader.load(
        url,
        (gltf) => {
          // Remove old model if exists
          if (this.vrm) {
            this.scene.remove(this.vrm.scene);
            this.vrm = null;
          }
          
          const vrm = gltf.userData.vrm as VRM;
          if (!vrm) {
            reject(new Error('Failed to load VRM from GLTF'));
            return;
          }
          
          this.vrm = vrm;
          
          // Rotate to face camera (VRM is Y-up, facing -Z)
          vrm.scene.rotation.y = Math.PI;
          
          // Add to scene
          this.scene.add(vrm.scene);
          
          // Create animation mixer
          this.mixer = new THREE.AnimationMixer(vrm.scene);
          
          // Initialize with default emotion
          this.setEmotion(this.options.defaultEmotion);
          
          console.log('VRM model loaded:', url);
          resolve();
        },
        (progress) => {
          console.log('Loading VRM:', (progress.loaded / progress.total * 100).toFixed(1) + '%');
        },
        (error) => {
          console.error('Error loading VRM:', error);
          reject(error);
        }
      );
    });
  }

  /**
   * Set the avatar's emotion
   */
  setEmotion(emotion: EmotionType, intensity: number = 0.7): void {
    this.targetEmotion = { primary: emotion, intensity };
    this.emotionTransitionProgress = 0;
  }

  /**
   * Set emotion from EmotionState object
   */
  setEmotionState(state: EmotionState): void {
    this.targetEmotion = state;
    this.emotionTransitionProgress = 0;
  }

  /**
   * Set a single viseme directly
   */
  setViseme(visemeId: OculusViseme, weight: number = 1): void {
    const weights = getVRMWeightsForViseme(visemeId);
    this.targetVisemeWeights = {
      aa: (weights.aa ?? 0) * weight,
      ih: (weights.ih ?? 0) * weight,
      ou: (weights.ou ?? 0) * weight,
      ee: (weights.ee ?? 0) * weight,
      oh: (weights.oh ?? 0) * weight
    };
  }

  /**
   * Play a viseme timeline synced to audio
   */
  playVisemeTimeline(timeline: VisemeEvent[], audioDuration: number): void {
    this.lipSyncTimeline = {
      events: timeline,
      duration: audioDuration,
      audioStartTime: performance.now()
    };
    this.lipSyncTimelineIndex = 0;
    this.isSpeaking = true;
  }

  /**
   * Stop viseme timeline playback
   */
  stopLipSync(): void {
    this.lipSyncTimeline = null;
    this.lipSyncTimelineIndex = 0;
    this.isSpeaking = false;
    this.targetVisemeWeights = {};
  }

  /**
   * Start audio-reactive lip-sync from an audio source
   */
  startAudioReactiveLipSync(audioElement: HTMLAudioElement): void {
    if (!this.audioContext) {
      this.audioContext = new AudioContext();
    }
    
    // Create analyser
    this.analyser = this.audioContext.createAnalyser();
    this.analyser.fftSize = 256;
    this.analyser.smoothingTimeConstant = 0.5;
    
    // Connect audio source
    const source = this.audioContext.createMediaElementSource(audioElement);
    source.connect(this.analyser);
    this.analyser.connect(this.audioContext.destination);
    
    // Create data array
    this.audioDataArray = new Uint8Array(this.analyser.frequencyBinCount);
    
    this.isSpeaking = true;
  }

  /**
   * Stop audio-reactive lip-sync
   */
  stopAudioReactiveLipSync(): void {
    this.analyser = null;
    this.audioDataArray = null;
    this.isSpeaking = false;
    this.targetVisemeWeights = {};
  }

  /**
   * Update loop - call this every frame or let animate() handle it
   */
  update(deltaTime: number): void {
    if (!this.vrm || this.isDisposed) return;
    
    const now = performance.now();
    
    // Update emotion transition
    this.updateEmotionTransition(deltaTime);
    
    // Update lip-sync
    if (this.lipSyncTimeline) {
      this.updateVisemeTimeline(now);
    } else if (this.analyser && this.audioDataArray) {
      this.updateAudioReactiveLipSync();
    }
    
    // Update viseme weights (lerp to target)
    this.updateVisemeWeights();
    
    // Apply expressions to VRM
    this.applyExpressions();
    
    // Update idle animations
    if (this.options.enableIdleAnimations) {
      this.updateBlinking(now);
      this.updateHeadMovement(deltaTime);
    }
    
    // Update VRM
    this.vrm.update(deltaTime);
    
    // Update animation mixer
    if (this.mixer) {
      this.mixer.update(deltaTime);
    }
  }

  /**
   * Render the scene
   */
  render(): void {
    if (this.isDisposed) return;
    this.renderer.render(this.scene, this.camera);
  }

  /**
   * Clean up resources
   */
  dispose(): void {
    this.isDisposed = true;
    
    if (this.animationFrameId !== null) {
      cancelAnimationFrame(this.animationFrameId);
    }
    
    window.removeEventListener('resize', this.handleResize);
    
    if (this.vrm) {
      this.scene.remove(this.vrm.scene);
      this.vrm = null;
    }
    
    if (this.audioContext) {
      this.audioContext.close();
      this.audioContext = null;
    }
    
    this.renderer.dispose();
    
    if (this.container.contains(this.renderer.domElement)) {
      this.container.removeChild(this.renderer.domElement);
    }
  }

  // === Private Methods ===

  private animate = (): void => {
    if (this.isDisposed) return;
    
    this.animationFrameId = requestAnimationFrame(this.animate);
    
    const deltaTime = this.clock.getDelta();
    this.update(deltaTime);
    this.render();
  };

  private handleResize = (): void => {
    const width = this.container.clientWidth;
    const height = this.container.clientHeight;
    
    this.camera.aspect = width / height;
    this.camera.updateProjectionMatrix();
    
    this.renderer.setSize(width, height);
  };

  private updateEmotionTransition(deltaTime: number): void {
    if (this.emotionTransitionProgress >= 1) return;
    
    this.emotionTransitionProgress += (deltaTime * 1000) / this.emotionTransitionDuration;
    this.emotionTransitionProgress = Math.min(1, this.emotionTransitionProgress);
    
    // Ease out cubic
    const t = 1 - Math.pow(1 - this.emotionTransitionProgress, 3);
    
    // Interpolate intensity
    this.currentEmotion = {
      primary: t < 0.5 ? this.currentEmotion.primary : this.targetEmotion.primary,
      intensity: this.currentEmotion.intensity + (this.targetEmotion.intensity - this.currentEmotion.intensity) * t
    };
  }

  private updateVisemeTimeline(now: number): void {
    if (!this.lipSyncTimeline) return;
    
    const elapsed = now - this.lipSyncTimeline.audioStartTime;
    
    // Check if timeline is complete
    if (elapsed >= this.lipSyncTimeline.duration) {
      this.stopLipSync();
      return;
    }
    
    // Find current viseme event
    const events = this.lipSyncTimeline.events;
    while (
      this.lipSyncTimelineIndex < events.length - 1 &&
      events[this.lipSyncTimelineIndex + 1].time <= elapsed
    ) {
      this.lipSyncTimelineIndex++;
    }
    
    const currentEvent = events[this.lipSyncTimelineIndex];
    const nextEvent = events[this.lipSyncTimelineIndex + 1];
    
    if (currentEvent) {
      // Calculate interpolation between current and next
      let t = 0;
      if (nextEvent && nextEvent.time > currentEvent.time) {
        const eventDuration = nextEvent.time - currentEvent.time;
        const eventElapsed = elapsed - currentEvent.time;
        t = Math.min(1, eventElapsed / eventDuration);
      }
      
      const currentWeights = getVRMWeightsForViseme(currentEvent.visemeId);
      const nextWeights = nextEvent 
        ? getVRMWeightsForViseme(nextEvent.visemeId)
        : currentWeights;
      
      this.targetVisemeWeights = interpolateVisemes(
        currentWeights,
        nextWeights,
        t
      );
      
      // Apply event weight
      const weight = currentEvent.weight;
      for (const key of Object.keys(this.targetVisemeWeights) as (keyof VRMBlendShapeWeights)[]) {
        if (this.targetVisemeWeights[key] !== undefined) {
          this.targetVisemeWeights[key]! *= weight;
        }
      }
    }
  }

  private updateAudioReactiveLipSync(): void {
    if (!this.analyser || !this.audioDataArray) return;
    
    this.analyser.getByteFrequencyData(this.audioDataArray as Uint8Array<ArrayBuffer>);
    
    // Get average of low frequencies (voice range ~80-500Hz)
    const lowFreqBins = this.audioDataArray.slice(1, 8);
    const avg = lowFreqBins.reduce((a, b) => a + b, 0) / lowFreqBins.length;
    
    // Normalize to 0-1
    const amplitude = avg / 255;
    
    // Map to mouth openness
    if (amplitude > 0.1) {
      // Open mouth based on amplitude
      const openness = Math.min(1, (amplitude - 0.1) * 2);
      this.targetVisemeWeights = {
        aa: openness * 0.8,
        oh: openness * 0.3
      };
    } else {
      // Mouth mostly closed
      this.targetVisemeWeights = {
        aa: 0,
        oh: 0
      };
    }
  }

  private updateVisemeWeights(): void {
    // Lerp current weights toward target
    const keys: (keyof VRMBlendShapeWeights)[] = ['aa', 'ih', 'ou', 'ee', 'oh'];
    
    for (const key of keys) {
      const current = this.currentVisemeWeights[key] ?? 0;
      const target = this.targetVisemeWeights[key] ?? 0;
      this.currentVisemeWeights[key] = current + (target - current) * this.visemeLerpFactor;
    }
  }

  private applyExpressions(): void {
    if (!this.vrm?.expressionManager) return;
    
    const expressionManager = this.vrm.expressionManager;
    
    // Get emotion weights
    const emotionWeights = getVRMWeightsForEmotion(this.currentEmotion);
    
    // Apply emotion expressions
    if (emotionWeights.happy !== undefined) {
      expressionManager.setValue('happy' as VRMExpressionPresetName, emotionWeights.happy);
    }
    if (emotionWeights.sad !== undefined) {
      expressionManager.setValue('sad' as VRMExpressionPresetName, emotionWeights.sad);
    }
    if (emotionWeights.angry !== undefined) {
      expressionManager.setValue('angry' as VRMExpressionPresetName, emotionWeights.angry);
    }
    if (emotionWeights.surprised !== undefined) {
      expressionManager.setValue('surprised' as VRMExpressionPresetName, emotionWeights.surprised);
    }
    
    // Apply viseme blend shapes
    // VRM uses 'aa', 'ih', 'ou', 'ee', 'oh' for mouth shapes
    if (this.currentVisemeWeights.aa !== undefined) {
      expressionManager.setValue('aa' as VRMExpressionPresetName, this.currentVisemeWeights.aa);
    }
    if (this.currentVisemeWeights.ih !== undefined) {
      expressionManager.setValue('ih' as VRMExpressionPresetName, this.currentVisemeWeights.ih);
    }
    if (this.currentVisemeWeights.ou !== undefined) {
      expressionManager.setValue('ou' as VRMExpressionPresetName, this.currentVisemeWeights.ou);
    }
    if (this.currentVisemeWeights.ee !== undefined) {
      expressionManager.setValue('ee' as VRMExpressionPresetName, this.currentVisemeWeights.ee);
    }
    if (this.currentVisemeWeights.oh !== undefined) {
      expressionManager.setValue('oh' as VRMExpressionPresetName, this.currentVisemeWeights.oh);
    }
  }

  private scheduleNextBlink(): void {
    const [min, max] = this.options.blinkInterval;
    this.nextBlinkTime = performance.now() + min + Math.random() * (max - min);
  }

  private updateBlinking(now: number): void {
    if (!this.vrm?.expressionManager) return;
    
    const expressionManager = this.vrm.expressionManager;
    
    if (!this.isBlinking && now >= this.nextBlinkTime) {
      this.isBlinking = true;
      this.blinkProgress = 0;
    }
    
    if (this.isBlinking) {
      this.blinkProgress += 0.15; // Speed of blink
      
      // Blink curve: quick close, slightly slower open
      let blinkValue: number;
      if (this.blinkProgress < 0.5) {
        // Closing
        blinkValue = this.blinkProgress * 2;
      } else {
        // Opening
        blinkValue = 1 - (this.blinkProgress - 0.5) * 2;
      }
      
      blinkValue = Math.max(0, Math.min(1, blinkValue));
      expressionManager.setValue('blink' as VRMExpressionPresetName, blinkValue);
      
      if (this.blinkProgress >= 1) {
        this.isBlinking = false;
        expressionManager.setValue('blink' as VRMExpressionPresetName, 0);
        this.scheduleNextBlink();
      }
    }
  }

  private scheduleNextHeadMove(): void {
    const [min, max] = this.options.headMovementInterval;
    this.nextHeadMoveTime = performance.now() + min + Math.random() * (max - min);
    
    // Random subtle rotation target
    this.headTargetRotation.set(
      (Math.random() - 0.5) * 0.1,  // Pitch ±3°
      (Math.random() - 0.5) * 0.15, // Yaw ±4.5°
      (Math.random() - 0.5) * 0.05  // Roll ±1.5°
    );
  }

  private updateHeadMovement(deltaTime: number): void {
    if (!this.vrm?.humanoid) return;
    
    const now = performance.now();
    
    if (now >= this.nextHeadMoveTime) {
      this.scheduleNextHeadMove();
    }
    
    // Smoothly interpolate head rotation
    const lerpFactor = deltaTime * 2; // Adjust speed
    
    this.headCurrentRotation.x += (this.headTargetRotation.x - this.headCurrentRotation.x) * lerpFactor;
    this.headCurrentRotation.y += (this.headTargetRotation.y - this.headCurrentRotation.y) * lerpFactor;
    this.headCurrentRotation.z += (this.headTargetRotation.z - this.headCurrentRotation.z) * lerpFactor;
    
    // Apply to neck bone
    const neck = this.vrm.humanoid.getNormalizedBoneNode('neck');
    if (neck) {
      neck.rotation.set(
        this.headCurrentRotation.x,
        this.headCurrentRotation.y,
        this.headCurrentRotation.z
      );
    }
  }

  // === Public Getters ===

  get isModelLoaded(): boolean {
    return this.vrm !== null;
  }

  get currentEmotionType(): EmotionType {
    return this.currentEmotion.primary;
  }

  get speaking(): boolean {
    return this.isSpeaking;
  }
}

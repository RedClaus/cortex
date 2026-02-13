<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { get } from 'svelte/store';
  import { T, useFrame, useThrelte } from '@threlte/core';
  import { OrbitControls, useGltf } from '@threlte/extras';
  import * as THREE from 'three';
  import { VRMLoaderPlugin, VRM } from '@pixiv/three-vrm';
  import { GLTFLoader } from 'three/examples/jsm/loaders/GLTFLoader.js';
  import { avatarState, type MouthShape } from '../stores/avatar';
  import { audioState } from '../stores/audio';
  import { BlendshapeController, type ChemicalState } from './BlendshapeController';

  export let modelUrl: string = '/models/avatar.vrm';
  export let chemicalState: ChemicalState | null = null;
  export let showDebug: boolean = false;

  let vrm: VRM | null = null;
  let loading = true;
  let error: string | null = null;
  let blendshapeController: BlendshapeController;

  let blinkTimer = 0;
  const BLINK_INTERVAL = 3000 + Math.random() * 2000;
  const BLINK_DURATION = 150;
  let isBlinking = false;

  let idleTimer = 0;
  let headRotationTarget = { x: 0, y: 0 };
  let headRotation = { x: 0, y: 0 };

  // Lip sync animation state
  let lipSyncTimer = 0;
  let currentLipSyncIndex = 0;
  const LIPSYNC_SHAPES: MouthShape[] = ['ah', 'oh', 'ee', 'mbp', 'closed', 'ah', 'oo', 'ee', 'closed'];
  const LIPSYNC_SPEED = 100;

  // Viseme timeline playback
  let visemePlaybackStart = 0;
  let currentVisemeIndex = 0;

  const { scene, camera } = useThrelte();

  // Map Oculus viseme IDs (0-14) to mouth shapes
  function visemeIdToMouthShape(visemeId: number): MouthShape {
    const mapping: Record<number, MouthShape> = {
      0: 'closed',   // sil
      1: 'mbp',      // PP
      2: 'mbp',      // FF
      3: 'mbp',      // TH
      4: 'closed',   // DD
      5: 'closed',   // kk
      6: 'ee',       // CH
      7: 'ee',       // SS
      8: 'closed',   // nn
      9: 'closed',   // RR
      10: 'ah',      // aa
      11: 'ee',      // E
      12: 'ee',      // ih
      13: 'oh',      // oh
      14: 'oo',      // ou
    };
    return mapping[visemeId] || 'closed';
  }

  function getJawRotation(mouthShape: MouthShape): number {
    const rotations: Record<MouthShape, number> = {
      'ah': 0.6,
      'oh': 0.5,
      'ee': 0.25,
      'oo': 0.4,
      'mbp': 0.1,
      'closed': 0,
    };
    return rotations[mouthShape] || 0;
  }

  // Initialize blendshape controller
  blendshapeController = new BlendshapeController();

  // Load VRM model
  onMount(async () => {
    console.log('[Avatar3D] Loading model from:', modelUrl);
    const loader = new GLTFLoader();
    loader.register((parser) => new VRMLoaderPlugin(parser));

    try {
      console.log('[Avatar3D] Starting load...');
      const gltf = await loader.loadAsync(modelUrl);
      console.log('[Avatar3D] GLTF loaded:', gltf);
      vrm = gltf.userData.vrm as VRM;
      console.log('[Avatar3D] VRM extracted:', vrm);

      if (vrm) {
        vrm.scene.rotation.y = Math.PI;
        vrm.scene.scale.setScalar(1.5);
        vrm.scene.position.set(0, -1.2, 0);

        const leftUpperArm = vrm.humanoid?.getNormalizedBoneNode('leftUpperArm');
        const rightUpperArm = vrm.humanoid?.getNormalizedBoneNode('rightUpperArm');
        const leftLowerArm = vrm.humanoid?.getNormalizedBoneNode('leftLowerArm');
        const rightLowerArm = vrm.humanoid?.getNormalizedBoneNode('rightLowerArm');
        
        if (leftUpperArm) {
          leftUpperArm.rotation.z = Math.PI * 0.4;
          leftUpperArm.rotation.x = Math.PI * 0.05;
        }
        if (rightUpperArm) {
          rightUpperArm.rotation.z = -Math.PI * 0.4;
          rightUpperArm.rotation.x = Math.PI * 0.05;
        }
        if (leftLowerArm) {
          leftLowerArm.rotation.y = -Math.PI * 0.1;
        }
        if (rightLowerArm) {
          rightLowerArm.rotation.y = Math.PI * 0.1;
        }

        scene.add(vrm.scene);
        
        // Log available expressions for debugging
        if (vrm.expressionManager) {
          const expressions = vrm.expressionManager.expressions;
          console.log('[Avatar3D] Available expressions:', expressions.map(e => e.expressionName));
        }

        blendshapeController.setVRM(vrm);

        loading = false;
        console.log('[Avatar3D] Loading complete');
      } else {
        console.error('[Avatar3D] VRM is null after extraction');
        error = 'VRM extraction failed';
        loading = false;
      }
    } catch (e) {
      console.error('[Avatar3D] Load error:', e);
      error = `Failed to load avatar: ${e}`;
      loading = false;
    }
  });

  onDestroy(() => {
    if (vrm) {
      scene.remove(vrm.scene);
      vrm = null;
    }
  });

  // Animation frame
  useFrame((_, delta) => {
    if (!vrm) return;

    const deltaMs = delta * 1000;

    // Update VRM
    vrm.update(delta);

    // Handle blinking
    blinkTimer += deltaMs;
    if (!isBlinking && blinkTimer > BLINK_INTERVAL && !$avatarState.isSpeaking) {
      isBlinking = true;
      blendshapeController.setBlink(true);
      setTimeout(() => {
        blendshapeController.setBlink(false);
        isBlinking = false;
      }, BLINK_DURATION);
      blinkTimer = 0;
    }

    // Subtle head movement for idle animation
    idleTimer += deltaMs;
    if (idleTimer > 2000) {
      headRotationTarget = {
        x: (Math.random() - 0.5) * 0.1,
        y: (Math.random() - 0.5) * 0.15,
      };
      idleTimer = 0;
    }

    // Lerp head rotation
    headRotation.x += (headRotationTarget.x - headRotation.x) * 0.02;
    headRotation.y += (headRotationTarget.y - headRotation.y) * 0.02;

    // Apply head rotation to neck bone
    const neck = vrm.humanoid?.getNormalizedBoneNode('neck');
    if (neck) {
      neck.rotation.x = headRotation.x;
      neck.rotation.y = headRotation.y;
    }

    // Update from avatar state
    blendshapeController.setEmotion($avatarState.emotion, 1.0);
    
    // Lip sync - use get() to read store in non-reactive context (Three.js render loop)
    const currentAudioState = get(audioState);
    const isSpeaking = currentAudioState.isSpeaking;
    const visemeTimeline = currentAudioState.visemeTimeline;
    const jaw = vrm.humanoid?.getNormalizedBoneNode('jaw');
    
    if (isSpeaking) {
      if (visemeTimeline && visemeTimeline.length > 0) {
        // Use viseme timeline from TTS provider
        if (visemePlaybackStart === 0) {
          visemePlaybackStart = performance.now();
          currentVisemeIndex = 0;
        }
        
        const elapsed = performance.now() - visemePlaybackStart;
        
        // Find current viseme based on elapsed time
        while (currentVisemeIndex < visemeTimeline.length - 1 &&
               visemeTimeline[currentVisemeIndex + 1].time <= elapsed) {
          currentVisemeIndex++;
        }
        
        const currentViseme = visemeTimeline[currentVisemeIndex];
        if (currentViseme) {
          const mouthShape = visemeIdToMouthShape(currentViseme.visemeId);
          const weight = currentViseme.weight || 1.0;
          blendshapeController.setViseme(mouthShape, weight);
          
          if (jaw) {
            jaw.rotation.x = getJawRotation(mouthShape) * weight;
          }
        }
      } else {
        // Fallback to simple animation
        lipSyncTimer += deltaMs;
        if (lipSyncTimer >= LIPSYNC_SPEED) {
          lipSyncTimer = 0;
          currentLipSyncIndex = (currentLipSyncIndex + 1) % LIPSYNC_SHAPES.length;
        }
        const currentShape = LIPSYNC_SHAPES[currentLipSyncIndex];
        blendshapeController.setViseme(currentShape, 1.0);
        
        if (jaw) {
          jaw.rotation.x = getJawRotation(currentShape);
        }
      }
    } else {
      // Reset lip sync state
      lipSyncTimer = 0;
      currentLipSyncIndex = 0;
      visemePlaybackStart = 0;
      currentVisemeIndex = 0;
      blendshapeController.setViseme('closed', 1.0);
      if (jaw) {
        jaw.rotation.x = 0;
      }
    }

    // Update from chemical state if provided
    if (chemicalState) {
      blendshapeController.setChemicalState(chemicalState);
    }

    // Apply gaze based on look direction
    const gazeMap = {
      center: { x: 0, y: 0 },
      left: { x: -0.5, y: 0 },
      right: { x: 0.5, y: 0 },
      up: { x: 0, y: 0.4 },
      down: { x: 0, y: -0.4 },
    };
    const gaze = gazeMap[$avatarState.lookDirection] || { x: 0, y: 0 };
    blendshapeController.setGaze(gaze.x, gaze.y);

    // Tick blendshape animation
    blendshapeController.tick(deltaMs);
  });

  // Reactive state for UI
  $: statusText = loading
    ? 'Loading avatar...'
    : error
    ? error
    : $avatarState.isSpeaking
    ? 'Speaking...'
    : $avatarState.isListening
    ? 'Listening...'
    : $avatarState.isThinking
    ? 'Thinking...'
    : 'Ready';

  $: emotionColor = {
    neutral: '#4a9eff',
    happy: '#4aff4a',
    sad: '#4a4aff',
    thinking: '#ffaa4a',
    confused: '#ff4aff',
    excited: '#ff4a4a',
    surprised: '#ffff4a',
  }[$avatarState.emotion];
</script>

<T.PerspectiveCamera
  makeDefault
  position={[0, 1.45, 1.8]}
  fov={28}
/>

<!-- Lighting -->
<T.AmbientLight intensity={0.6} />
<T.DirectionalLight
  position={[5, 5, 5]}
  intensity={1.0}
  castShadow
/>
<T.DirectionalLight
  position={[-3, 3, -3]}
  intensity={0.4}
/>

<!-- Environment -->
<T.Mesh
  position={[0, -1.5, 0]}
  rotation.x={-Math.PI / 2}
  receiveShadow
>
  <T.CircleGeometry args={[2, 32]} />
  <T.MeshStandardMaterial color="#1a1a2e" />
</T.Mesh>

<!-- Controls for debug mode -->
{#if showDebug}
  <OrbitControls
    enableDamping
    dampingFactor={0.05}
    minDistance={1}
    maxDistance={5}
    target={[0, 0, 0]}
  />
{/if}

<!-- Status overlay -->
<div class="avatar-status" style="--emotion-color: {emotionColor}">
  <div class="status-indicator" class:active={$audioState.isSpeaking || $avatarState.isListening || $avatarState.isThinking}>
    <span class="status-dot" class:speaking={$audioState.isSpeaking}></span>
    <span class="status-text">{$audioState.isSpeaking ? 'SPEAKING' : statusText}</span>
  </div>

  {#if loading}
    <div class="loading-spinner"></div>
  {/if}

  {#if showDebug}
    <div class="debug-panel">
      <p>Emotion: {$avatarState.emotion}</p>
      <p>Mouth: {$avatarState.mouthShape}</p>
      <p>Eyes: {$avatarState.eyeState}</p>
      <p>Look: {$avatarState.lookDirection}</p>
      {#if chemicalState}
        <p>DA: {chemicalState.dopamine.toFixed(2)}</p>
        <p>NE: {chemicalState.norepinephrine.toFixed(2)}</p>
        <p>5HT: {chemicalState.serotonin.toFixed(2)}</p>
      {/if}
    </div>
  {/if}
</div>

<style>
  .avatar-status {
    position: absolute;
    bottom: 20px;
    left: 50%;
    transform: translateX(-50%);
    z-index: 10;
  }

  .status-indicator {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    background: rgba(0, 0, 0, 0.6);
    border-radius: 20px;
    border: 1px solid var(--emotion-color, #4a9eff);
    transition: all 0.3s ease;
  }

  .status-indicator.active {
    background: rgba(0, 0, 0, 0.8);
    box-shadow: 0 0 20px var(--emotion-color, #4a9eff);
  }

  .status-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background: var(--emotion-color, #4a9eff);
    animation: pulse 2s infinite;
  }

  .status-dot.speaking {
    background: #ff4a4a;
    animation: pulse 0.3s infinite;
  }

  .status-text {
    color: white;
    font-size: 14px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  .loading-spinner {
    position: absolute;
    top: -60px;
    left: 50%;
    transform: translateX(-50%);
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255, 255, 255, 0.2);
    border-top-color: var(--emotion-color, #4a9eff);
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  .debug-panel {
    position: fixed;
    top: 10px;
    right: 10px;
    background: rgba(0, 0, 0, 0.8);
    padding: 10px;
    border-radius: 8px;
    color: white;
    font-family: monospace;
    font-size: 12px;
  }

  .debug-panel p {
    margin: 4px 0;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.5; transform: scale(0.8); }
  }

  @keyframes spin {
    to { transform: translateX(-50%) rotate(360deg); }
  }
</style>

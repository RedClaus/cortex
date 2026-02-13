/**
 * BlendshapeController.ts
 *
 * ARKit 52 Blendshape controller for VRM avatars.
 * Maps Cortex emotion/viseme state to ARKit-compatible blendshapes.
 */

import type { VRM, VRMExpressionManager } from '@pixiv/three-vrm';
import type { EmotionState, MouthShape } from '../stores/avatar';

// ARKit 52 Blendshape names (standard set)
export const ARKIT_BLENDSHAPES = [
  // Eyes
  'eyeBlinkLeft', 'eyeBlinkRight',
  'eyeLookDownLeft', 'eyeLookDownRight',
  'eyeLookInLeft', 'eyeLookInRight',
  'eyeLookOutLeft', 'eyeLookOutRight',
  'eyeLookUpLeft', 'eyeLookUpRight',
  'eyeSquintLeft', 'eyeSquintRight',
  'eyeWideLeft', 'eyeWideRight',

  // Jaw
  'jawForward', 'jawLeft', 'jawRight', 'jawOpen',

  // Mouth
  'mouthClose',
  'mouthFunnel', 'mouthPucker',
  'mouthLeft', 'mouthRight',
  'mouthSmileLeft', 'mouthSmileRight',
  'mouthFrownLeft', 'mouthFrownRight',
  'mouthDimpleLeft', 'mouthDimpleRight',
  'mouthStretchLeft', 'mouthStretchRight',
  'mouthRollLower', 'mouthRollUpper',
  'mouthShrugLower', 'mouthShrugUpper',
  'mouthPressLeft', 'mouthPressRight',
  'mouthLowerDownLeft', 'mouthLowerDownRight',
  'mouthUpperUpLeft', 'mouthUpperUpRight',

  // Brows
  'browDownLeft', 'browDownRight',
  'browInnerUp',
  'browOuterUpLeft', 'browOuterUpRight',

  // Cheek
  'cheekPuff',
  'cheekSquintLeft', 'cheekSquintRight',

  // Nose
  'noseSneerLeft', 'noseSneerRight',

  // Tongue
  'tongueOut',
] as const;

export type ARKitBlendshape = typeof ARKIT_BLENDSHAPES[number];

export interface BlendshapeWeights {
  [key: string]: number;
}

// Emotion to blendshape presets
const EMOTION_PRESETS: Record<EmotionState, BlendshapeWeights> = {
  neutral: {},
  happy: {
    mouthSmileLeft: 0.7,
    mouthSmileRight: 0.7,
    cheekSquintLeft: 0.4,
    cheekSquintRight: 0.4,
    eyeSquintLeft: 0.2,
    eyeSquintRight: 0.2,
  },
  sad: {
    mouthFrownLeft: 0.6,
    mouthFrownRight: 0.6,
    browInnerUp: 0.5,
    browDownLeft: 0.3,
    browDownRight: 0.3,
    eyeLookDownLeft: 0.3,
    eyeLookDownRight: 0.3,
  },
  thinking: {
    browDownLeft: 0.4,
    browDownRight: 0.2,
    eyeLookUpLeft: 0.3,
    eyeLookUpRight: 0.3,
    mouthPucker: 0.2,
  },
  confused: {
    browInnerUp: 0.5,
    browDownLeft: 0.3,
    eyeSquintLeft: 0.2,
    mouthFrownLeft: 0.2,
    mouthFrownRight: 0.2,
  },
  excited: {
    mouthSmileLeft: 0.9,
    mouthSmileRight: 0.9,
    eyeWideLeft: 0.5,
    eyeWideRight: 0.5,
    browOuterUpLeft: 0.4,
    browOuterUpRight: 0.4,
    cheekSquintLeft: 0.3,
    cheekSquintRight: 0.3,
  },
  surprised: {
    eyeWideLeft: 0.7,
    eyeWideRight: 0.7,
    browOuterUpLeft: 0.6,
    browOuterUpRight: 0.6,
    browInnerUp: 0.5,
    jawOpen: 0.4,
    mouthFunnel: 0.3,
  },
};

// Viseme (mouth shape) to blendshape mapping - MAX values for testing
const VISEME_PRESETS: Record<MouthShape, BlendshapeWeights> = {
  closed: {
    mouthClose: 1.0,
  },
  ah: {
    jawOpen: 1.0,
    mouthFunnel: 0.8,
    mouthLowerDownLeft: 1.0,
    mouthLowerDownRight: 1.0,
  },
  oh: {
    jawOpen: 0.9,
    mouthFunnel: 1.0,
    mouthPucker: 1.0,
  },
  ee: {
    jawOpen: 0.3,
    mouthSmileLeft: 1.0,
    mouthSmileRight: 1.0,
    mouthStretchLeft: 1.0,
    mouthStretchRight: 1.0,
  },
  fv: {
    mouthUpperUpLeft: 1.0,
    mouthUpperUpRight: 1.0,
    mouthLowerDownLeft: 0.8,
    mouthLowerDownRight: 0.8,
  },
  th: {
    jawOpen: 0.6,
    tongueOut: 1.0,
    mouthLowerDownLeft: 0.8,
    mouthLowerDownRight: 0.8,
  },
  mbp: {
    mouthClose: 1.0,
    mouthPressLeft: 1.0,
    mouthPressRight: 1.0,
  },
  lnt: {
    jawOpen: 0.7,
    mouthUpperUpLeft: 0.8,
    mouthUpperUpRight: 0.8,
  },
  wq: {
    mouthPucker: 1.0,
    mouthFunnel: 1.0,
  },
  oo: {
    jawOpen: 0.6,
    mouthPucker: 1.0,
    mouthFunnel: 1.0,
  },
  ih: {
    jawOpen: 0.5,
    mouthSmileLeft: 0.7,
    mouthSmileRight: 0.7,
    mouthStretchLeft: 0.6,
    mouthStretchRight: 0.6,
  },
  er: {
    jawOpen: 0.7,
    mouthPucker: 0.8,
    mouthFunnel: 0.7,
    mouthRollLower: 0.6,
  },
  ch: {
    jawOpen: 0.6,
    mouthPucker: 1.0,
    mouthFunnel: 0.9,
    mouthShrugUpper: 0.7,
  },
  ng: {
    jawOpen: 0.5,
    mouthClose: 0.7,
    mouthShrugLower: 0.6,
  },
  k: {
    jawOpen: 0.8,
    mouthFunnel: 0.5,
    mouthShrugLower: 0.5,
  },
};

// Limbic chemical state to expression mapping
export interface ChemicalState {
  dopamine: number;      // 0-1: reward/motivation
  norepinephrine: number; // 0-1: urgency/attention
  serotonin: number;     // 0-1: control/safety
}

export class BlendshapeController {
  private currentWeights: BlendshapeWeights = {};
  private targetWeights: BlendshapeWeights = {};
  private blendSpeed: number = 0.15; // Lerp speed per frame
  private vrm: VRM | null = null;

  constructor(vrm?: VRM) {
    if (vrm) {
      this.vrm = vrm;
    }
  }

  setVRM(vrm: VRM) {
    this.vrm = vrm;
  }

  setBlendSpeed(speed: number) {
    this.blendSpeed = Math.max(0.01, Math.min(1, speed));
  }

  /**
   * Update target weights from emotion state
   */
  setEmotion(emotion: EmotionState, intensity: number = 1.0): void {
    const preset = EMOTION_PRESETS[emotion] || {};
    this.targetWeights = { ...this.targetWeights };

    // Apply emotion preset with intensity
    for (const [key, value] of Object.entries(preset)) {
      this.targetWeights[key] = value * intensity;
    }
  }

  /**
   * Update target weights from viseme (mouth shape)
   */
  setViseme(viseme: MouthShape, intensity: number = 1.0): void {
    const preset = VISEME_PRESETS[viseme] || {};

    // Clear previous mouth-related weights
    const mouthKeys = Object.keys(VISEME_PRESETS).flatMap(v =>
      Object.keys(VISEME_PRESETS[v as MouthShape])
    );
    for (const key of mouthKeys) {
      if (this.targetWeights[key] !== undefined) {
        this.targetWeights[key] = 0;
      }
    }

    // Apply new viseme
    for (const [key, value] of Object.entries(preset)) {
      this.targetWeights[key] = value * intensity;
    }

    // Also set VRM standard viseme expressions directly
    const vrmVisemeMap: Record<MouthShape, Record<string, number>> = {
      'closed': { 'aa': 0, 'ih': 0, 'ou': 0, 'ee': 0, 'oh': 0 },
      'ah': { 'aa': 1.0 },
      'oh': { 'oh': 1.0 },
      'ee': { 'ee': 1.0, 'ih': 0.5 },
      'oo': { 'ou': 1.0 },
      'ih': { 'ih': 0.9 },
      'mbp': { 'aa': 0, 'oh': 0, 'ou': 0.3 },
      'fv': { 'ih': 0.5 },
      'th': { 'aa': 0.5 },
      'lnt': { 'ih': 0.6, 'aa': 0.4 },
      'wq': { 'ou': 0.9 },
      'er': { 'oh': 0.6, 'aa': 0.4 },
      'ch': { 'ee': 0.5, 'ih': 0.5 },
      'ng': { 'ih': 0.4 },
      'k': { 'aa': 0.6 },
    };
    
    const vrmPreset = vrmVisemeMap[viseme];
    if (vrmPreset) {
      for (const [key, value] of Object.entries(vrmPreset)) {
        this.targetWeights[key] = value * intensity;
      }
    }
  }

  /**
   * Update from limbic chemical state
   */
  setChemicalState(chemicals: ChemicalState): void {
    const { dopamine, norepinephrine, serotonin } = chemicals;

    // High dopamine = happy/engaged
    if (dopamine > 0.6) {
      const intensity = (dopamine - 0.6) * 2.5;
      this.blendPreset(EMOTION_PRESETS.happy, intensity * 0.5);
    }

    // High norepinephrine = alert/focused
    if (norepinephrine > 0.6) {
      const intensity = (norepinephrine - 0.6) * 2.5;
      this.targetWeights.eyeWideLeft = (this.targetWeights.eyeWideLeft || 0) + intensity * 0.3;
      this.targetWeights.eyeWideRight = (this.targetWeights.eyeWideRight || 0) + intensity * 0.3;
      this.targetWeights.browDownLeft = (this.targetWeights.browDownLeft || 0) + intensity * 0.2;
      this.targetWeights.browDownRight = (this.targetWeights.browDownRight || 0) + intensity * 0.2;
    }

    // Low serotonin + high norepinephrine = anxious
    if (serotonin < 0.4 && norepinephrine > 0.5) {
      const intensity = (0.4 - serotonin) * 2.5;
      this.targetWeights.browInnerUp = (this.targetWeights.browInnerUp || 0) + intensity * 0.4;
      this.targetWeights.mouthFrownLeft = (this.targetWeights.mouthFrownLeft || 0) + intensity * 0.2;
      this.targetWeights.mouthFrownRight = (this.targetWeights.mouthFrownRight || 0) + intensity * 0.2;
    }

    // High serotonin = calm/cautious
    if (serotonin > 0.7) {
      const intensity = (serotonin - 0.7) * 3.33;
      this.targetWeights.mouthPucker = (this.targetWeights.mouthPucker || 0) + intensity * 0.2;
      this.targetWeights.eyeSquintLeft = (this.targetWeights.eyeSquintLeft || 0) + intensity * 0.15;
      this.targetWeights.eyeSquintRight = (this.targetWeights.eyeSquintRight || 0) + intensity * 0.15;
    }
  }

  /**
   * Set blink state
   */
  setBlink(closed: boolean): void {
    this.targetWeights.eyeBlinkLeft = closed ? 1.0 : 0;
    this.targetWeights.eyeBlinkRight = closed ? 1.0 : 0;
  }

  /**
   * Set gaze direction
   */
  setGaze(x: number, y: number): void {
    // x: -1 (left) to 1 (right)
    // y: -1 (down) to 1 (up)

    // Reset gaze weights
    this.targetWeights.eyeLookInLeft = 0;
    this.targetWeights.eyeLookOutLeft = 0;
    this.targetWeights.eyeLookInRight = 0;
    this.targetWeights.eyeLookOutRight = 0;
    this.targetWeights.eyeLookUpLeft = 0;
    this.targetWeights.eyeLookUpRight = 0;
    this.targetWeights.eyeLookDownLeft = 0;
    this.targetWeights.eyeLookDownRight = 0;

    // Horizontal gaze
    if (x < 0) {
      // Looking left
      this.targetWeights.eyeLookOutLeft = Math.abs(x);
      this.targetWeights.eyeLookInRight = Math.abs(x);
    } else if (x > 0) {
      // Looking right
      this.targetWeights.eyeLookInLeft = x;
      this.targetWeights.eyeLookOutRight = x;
    }

    // Vertical gaze
    if (y > 0) {
      // Looking up
      this.targetWeights.eyeLookUpLeft = y;
      this.targetWeights.eyeLookUpRight = y;
    } else if (y < 0) {
      // Looking down
      this.targetWeights.eyeLookDownLeft = Math.abs(y);
      this.targetWeights.eyeLookDownRight = Math.abs(y);
    }
  }

  /**
   * Tick animation - lerp toward target weights
   */
  tick(deltaTime: number = 16): BlendshapeWeights {
    const alpha = Math.min(1, this.blendSpeed * (deltaTime / 16));

    // Collect all keys
    const allKeys = new Set([
      ...Object.keys(this.currentWeights),
      ...Object.keys(this.targetWeights),
    ]);

    for (const key of allKeys) {
      const current = this.currentWeights[key] || 0;
      const target = this.targetWeights[key] || 0;

      // Lerp toward target
      const newValue = current + (target - current) * alpha;

      // Clean up near-zero values
      if (Math.abs(newValue) < 0.001) {
        delete this.currentWeights[key];
      } else {
        this.currentWeights[key] = Math.max(0, Math.min(1, newValue));
      }
    }

    // Apply to VRM if available
    if (this.vrm?.expressionManager) {
      this.applyToVRM(this.vrm.expressionManager);
    }

    return { ...this.currentWeights };
  }

  /**
   * Apply current weights to VRM expression manager
   */
  private applyToVRM(manager: VRMExpressionManager): void {
    // Map ARKit names to VRM expression names
    const arkitToVrm: Record<string, string> = {
      'jawOpen': 'aa',
      'mouthFunnel': 'oh', 
      'mouthPucker': 'ou',
      'mouthSmileLeft': 'happy',
      'mouthSmileRight': 'happy',
      'mouthFrownLeft': 'sad',
      'mouthFrownRight': 'sad',
      'mouthClose': 'neutral',
      'eyeBlinkLeft': 'blinkLeft',
      'eyeBlinkRight': 'blinkRight',
    };

    for (const [key, value] of Object.entries(this.currentWeights)) {
      try {
        // Try VRM mapped name first
        const vrmName = arkitToVrm[key];
        if (vrmName) {
          manager.setValue(vrmName, value);
        }
        // Also try the original key
        manager.setValue(key, value);
      } catch {
        // Blendshape might not exist on this model
      }
    }
  }

  /**
   * Blend in a preset with intensity
   */
  private blendPreset(preset: BlendshapeWeights, intensity: number): void {
    for (const [key, value] of Object.entries(preset)) {
      this.targetWeights[key] = Math.min(1, (this.targetWeights[key] || 0) + value * intensity);
    }
  }

  /**
   * Reset all weights to zero
   */
  reset(): void {
    this.targetWeights = {};
    this.currentWeights = {};
  }

  /**
   * Get current blendshape weights
   */
  getWeights(): BlendshapeWeights {
    return { ...this.currentWeights };
  }
}

export default BlendshapeController;

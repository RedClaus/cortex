/**
 * LipSync.ts
 *
 * Viseme smoothing and interpolation for natural lip-sync animation.
 * Handles timing, transitions, and coarticulation effects.
 */

import type { MouthShape } from '../stores/avatar';

export interface TimedViseme {
  shape: MouthShape;
  startMs: number;
  endMs: number;
  intensity?: number;
}

export interface LipSyncConfig {
  // Transition timing
  transitionMs: number;      // Default transition time between visemes
  holdMinMs: number;         // Minimum hold time for a viseme

  // Coarticulation
  anticipationMs: number;    // Start transitioning before viseme starts
  overshootFactor: number;   // How much to overshoot target (0-1)

  // Smoothing
  smoothingFactor: number;   // Lerp factor per frame (0-1)
  easeType: 'linear' | 'easeIn' | 'easeOut' | 'easeInOut';
}

const DEFAULT_CONFIG: LipSyncConfig = {
  transitionMs: 50,
  holdMinMs: 30,
  anticipationMs: 20,
  overshootFactor: 0.1,
  smoothingFactor: 0.3,
  easeType: 'easeInOut',
};

/**
 * Easing functions for smooth transitions
 */
const EASE_FUNCTIONS = {
  linear: (t: number) => t,
  easeIn: (t: number) => t * t,
  easeOut: (t: number) => t * (2 - t),
  easeInOut: (t: number) => t < 0.5 ? 2 * t * t : -1 + (4 - 2 * t) * t,
};

/**
 * LipSyncController handles smooth viseme transitions
 */
export class LipSyncController {
  private config: LipSyncConfig;
  private currentViseme: MouthShape = 'closed';
  private targetViseme: MouthShape = 'closed';
  private transitionProgress: number = 1.0;
  private transitionDuration: number = 0;

  private queue: TimedViseme[] = [];
  private queueStartTime: number = 0;
  private isPlaying: boolean = false;

  private onVisemeChange?: (viseme: MouthShape, intensity: number) => void;

  constructor(config: Partial<LipSyncConfig> = {}) {
    this.config = { ...DEFAULT_CONFIG, ...config };
  }

  /**
   * Set callback for viseme changes
   */
  setOnVisemeChange(callback: (viseme: MouthShape, intensity: number) => void): void {
    this.onVisemeChange = callback;
  }

  /**
   * Queue a sequence of visemes for playback
   */
  queueVisemes(visemes: TimedViseme[]): void {
    // Sort by start time
    this.queue = visemes.sort((a, b) => a.startMs - b.startMs);
    this.queueStartTime = performance.now();
    this.isPlaying = true;
  }

  /**
   * Clear the viseme queue
   */
  clearQueue(): void {
    this.queue = [];
    this.isPlaying = false;
    this.transitionTo('closed');
  }

  /**
   * Immediately transition to a viseme
   */
  transitionTo(viseme: MouthShape, durationMs?: number): void {
    if (viseme === this.targetViseme) return;

    this.currentViseme = this.targetViseme;
    this.targetViseme = viseme;
    this.transitionProgress = 0;
    this.transitionDuration = durationMs ?? this.config.transitionMs;
  }

  /**
   * Get the current viseme with interpolation
   */
  getCurrentViseme(): { viseme: MouthShape; intensity: number } {
    const ease = EASE_FUNCTIONS[this.config.easeType];
    const easedProgress = ease(this.transitionProgress);

    // If fully transitioned, return target
    if (this.transitionProgress >= 1.0) {
      return { viseme: this.targetViseme, intensity: 1.0 };
    }

    // During transition, return target with partial intensity
    // The BlendshapeController will handle the actual blending
    return {
      viseme: this.targetViseme,
      intensity: easedProgress,
    };
  }

  /**
   * Update the lip sync state (call every frame)
   */
  tick(deltaMs: number): { viseme: MouthShape; intensity: number } {
    // Update transition progress
    if (this.transitionProgress < 1.0 && this.transitionDuration > 0) {
      this.transitionProgress = Math.min(
        1.0,
        this.transitionProgress + (deltaMs / this.transitionDuration)
      );
    }

    // Process queue
    if (this.isPlaying && this.queue.length > 0) {
      const elapsed = performance.now() - this.queueStartTime;

      // Find current viseme in queue (with anticipation)
      const currentIndex = this.queue.findIndex(v =>
        v.startMs - this.config.anticipationMs <= elapsed &&
        v.endMs > elapsed
      );

      if (currentIndex >= 0) {
        const current = this.queue[currentIndex];
        if (current.shape !== this.targetViseme) {
          this.transitionTo(current.shape);
        }
      } else if (elapsed > this.queue[this.queue.length - 1].endMs) {
        // Queue finished
        this.isPlaying = false;
        this.transitionTo('closed');
      }
    }

    const result = this.getCurrentViseme();

    // Notify listener
    if (this.onVisemeChange) {
      this.onVisemeChange(result.viseme, result.intensity);
    }

    return result;
  }

  /**
   * Check if currently playing a viseme sequence
   */
  get playing(): boolean {
    return this.isPlaying;
  }

  /**
   * Get remaining time in queue
   */
  get remainingMs(): number {
    if (!this.isPlaying || this.queue.length === 0) return 0;
    const elapsed = performance.now() - this.queueStartTime;
    const lastEnd = this.queue[this.queue.length - 1].endMs;
    return Math.max(0, lastEnd - elapsed);
  }
}

/**
 * Coarticulation rules for natural speech
 * Adjusts viseme intensity based on surrounding phonemes
 */
export function applyCoarticulation(
  visemes: TimedViseme[],
  config: Partial<LipSyncConfig> = {}
): TimedViseme[] {
  const cfg = { ...DEFAULT_CONFIG, ...config };
  const result: TimedViseme[] = [];

  for (let i = 0; i < visemes.length; i++) {
    const current = { ...visemes[i] };
    const prev = i > 0 ? visemes[i - 1] : null;
    const next = i < visemes.length - 1 ? visemes[i + 1] : null;

    // Calculate duration
    const duration = current.endMs - current.startMs;

    // Short visemes get reduced intensity
    if (duration < cfg.holdMinMs * 2) {
      current.intensity = (current.intensity ?? 1.0) * 0.7;
    }

    // Bilabials (mbp) before vowels need full closure
    if (current.shape === 'mbp' && next) {
      const isNextVowel = ['ah', 'oh', 'ee', 'oo', 'ih', 'er'].includes(next.shape);
      if (isNextVowel) {
        current.intensity = 1.0;
      }
    }

    // Reduce intensity for repeated similar shapes
    if (prev && prev.shape === current.shape) {
      current.intensity = (current.intensity ?? 1.0) * 0.8;
    }

    result.push(current);
  }

  return result;
}

/**
 * Generate viseme timeline from phoneme data
 */
export function phonemesToVisemes(
  phonemes: Array<{ symbol: string; startMs: number; endMs: number }>,
  phonemeToViseme: Record<string, MouthShape>
): TimedViseme[] {
  return phonemes.map(p => ({
    shape: phonemeToViseme[p.symbol] || 'closed',
    startMs: p.startMs,
    endMs: p.endMs,
    intensity: 1.0,
  }));
}

export default LipSyncController;

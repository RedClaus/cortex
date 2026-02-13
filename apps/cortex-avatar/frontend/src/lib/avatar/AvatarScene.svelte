<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';
  import { TalkingHeadController } from './TalkingHeadController';
  import type { EmotionType } from './emotions';
  import type { VisemeEvent } from './visemes';
  import { detectEmotionFromText } from './emotions';
  import { textToApproximateVisemes } from './visemes';

  // Props
  export let modelUrl: string = '/models/default.vrm';
  export let emotion: EmotionType = 'neutral';
  export let emotionIntensity: number = 0.7;
  export let isSpeaking: boolean = false;
  export let visemeData: VisemeEvent[] | null = null;
  export let audioElement: HTMLAudioElement | null = null;
  export let responseText: string = '';
  export let autoDetectEmotion: boolean = true;
  export let enableIdleAnimations: boolean = true;

  // Internal state
  let container: HTMLElement;
  let controller: TalkingHeadController | null = null;
  let isLoading: boolean = true;
  let loadError: string | null = null;
  let currentModelUrl: string = '';

  const dispatch = createEventDispatcher<{
    ready: void;
    error: Error;
    modelLoaded: string;
  }>();

  // Initialize controller on mount
  onMount(() => {
    if (!container) return;

    controller = new TalkingHeadController(container, {
      modelUrl,
      defaultEmotion: emotion,
      enableIdleAnimations,
      lipSyncMode: visemeData ? 'viseme' : 'audio-reactive'
    });

    loadModel(modelUrl);
  });

  // Cleanup on destroy
  onDestroy(() => {
    if (controller) {
      controller.dispose();
      controller = null;
    }
  });

  // Load model function
  async function loadModel(url: string) {
    if (!controller || url === currentModelUrl) return;

    isLoading = true;
    loadError = null;

    try {
      await controller.loadModel(url);
      currentModelUrl = url;
      isLoading = false;
      dispatch('modelLoaded', url);
      dispatch('ready');
    } catch (err) {
      loadError = err instanceof Error ? err.message : 'Failed to load model';
      isLoading = false;
      dispatch('error', err instanceof Error ? err : new Error(loadError));
    }
  }

  // React to modelUrl changes
  $: if (controller && modelUrl !== currentModelUrl) {
    loadModel(modelUrl);
  }

  // React to emotion changes
  $: if (controller) {
    // If auto-detect is enabled and we have response text, detect emotion
    if (autoDetectEmotion && responseText) {
      const detected = detectEmotionFromText(responseText);
      controller.setEmotionState(detected);
    } else {
      controller.setEmotion(emotion, emotionIntensity);
    }
  }

  // React to speaking state changes
  $: if (controller) {
    if (isSpeaking) {
      if (visemeData && visemeData.length > 0 && audioElement) {
        // Use viseme timeline
        const duration = audioElement.duration * 1000 || estimateSpeechDuration(responseText);
        controller.playVisemeTimeline(visemeData, duration);
      } else if (audioElement) {
        // Use audio-reactive lip sync
        controller.startAudioReactiveLipSync(audioElement);
      } else if (responseText) {
        // Fallback: generate approximate visemes from text
        const duration = estimateSpeechDuration(responseText);
        const approximateVisemes = textToApproximateVisemes(responseText, duration);
        controller.playVisemeTimeline(approximateVisemes, duration);
      }
    } else {
      controller.stopLipSync();
    }
  }

  // Estimate speech duration from text (rough approximation)
  function estimateSpeechDuration(text: string): number {
    const wordsPerMinute = 150;
    const words = text.split(/\s+/).length;
    return (words / wordsPerMinute) * 60 * 1000;
  }
</script>

<div class="avatar-scene" bind:this={container}>
  {#if isLoading}
    <div class="loading-overlay">
      <div class="loading-spinner"></div>
      <span>Loading avatar...</span>
    </div>
  {/if}
  
  {#if loadError}
    <div class="error-overlay">
      <span class="error-icon">!</span>
      <span class="error-text">{loadError}</span>
      <button on:click={() => loadModel(modelUrl)}>Retry</button>
    </div>
  {/if}
</div>

<style>
  .avatar-scene {
    width: 100%;
    height: 100%;
    position: relative;
    background: transparent;
    border-radius: 8px;
    overflow: hidden;
  }

  .avatar-scene :global(canvas) {
    width: 100% !important;
    height: 100% !important;
    display: block;
  }

  .loading-overlay,
  .error-overlay {
    position: absolute;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background: rgba(0, 0, 0, 0.7);
    color: white;
    gap: 12px;
    z-index: 10;
  }

  .loading-spinner {
    width: 40px;
    height: 40px;
    border: 3px solid rgba(255, 255, 255, 0.3);
    border-top-color: white;
    border-radius: 50%;
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    to {
      transform: rotate(360deg);
    }
  }

  .error-overlay {
    background: rgba(200, 50, 50, 0.9);
  }

  .error-icon {
    width: 40px;
    height: 40px;
    border: 2px solid white;
    border-radius: 50%;
    display: flex;
    align-items: center;
    justify-content: center;
    font-weight: bold;
    font-size: 24px;
  }

  .error-text {
    max-width: 80%;
    text-align: center;
  }

  .error-overlay button {
    padding: 8px 16px;
    background: white;
    color: #c83232;
    border: none;
    border-radius: 4px;
    cursor: pointer;
    font-weight: bold;
  }

  .error-overlay button:hover {
    background: #f0f0f0;
  }
</style>

<script lang="ts">
  import { audioState } from '../stores/audio';
  import { onMount, onDestroy } from 'svelte';

  // Props
  export let mode: 'push-to-talk' | 'toggle' = 'push-to-talk';
  export let size: 'small' | 'medium' | 'large' = 'large';
  export let disabled = false;

  // State
  let isPressed = false;
  let isActive = false;
  let pulseAnimation = false;

  // Wails runtime bindings
  declare global {
    interface Window {
      runtime: {
        EventsEmit: (event: string, ...args: any[]) => void;
        EventsOn: (event: string, callback: (...args: any[]) => void) => void;
        EventsOff: (event: string) => void;
      };
      go: {
        bridge: {
          AudioManager: {
            StartListening: () => Promise<void>;
            StopListening: () => Promise<void>;
            GetState: () => Promise<string>;
          };
        };
      };
    }
  }

  // Handle button press (push-to-talk)
  function handleMouseDown() {
    if (disabled || mode !== 'push-to-talk') return;
    isPressed = true;
    startListening();
  }

  function handleMouseUp() {
    if (disabled || mode !== 'push-to-talk') return;
    isPressed = false;
    stopListening();
  }

  // Handle button click (toggle mode)
  function handleClick() {
    if (disabled || mode !== 'toggle') return;
    isActive = !isActive;

    if (isActive) {
      startListening();
    } else {
      stopListening();
    }
  }

  // Start voice capture
  async function startListening() {
    try {
      // Call Go backend to start audio capture
      if (window.go?.bridge?.AudioManager) {
        await window.go.bridge.AudioManager.StartListening();
      }

      // Update local state
      audioState.update(s => ({ ...s, isListening: true }));

      // Emit event for other components
      window.runtime?.EventsEmit('audio:startListening');

      console.log('[VoiceButton] Started listening');
    } catch (error) {
      console.error('[VoiceButton] Failed to start listening:', error);
    }
  }

  // Stop voice capture
  async function stopListening() {
    try {
      // Call Go backend to stop audio capture
      if (window.go?.bridge?.AudioManager) {
        await window.go.bridge.AudioManager.StopListening();
      }

      // Update local state
      audioState.update(s => ({ ...s, isListening: false }));

      // Emit event for other components
      window.runtime?.EventsEmit('audio:stopListening');

      console.log('[VoiceButton] Stopped listening');
    } catch (error) {
      console.error('[VoiceButton] Failed to stop listening:', error);
    }
  }

  // Listen for audio state changes from Go backend
  onMount(() => {
    if (window.runtime) {
      window.runtime.EventsOn('audio:stateChanged', (state: any) => {
        isActive = state.isListening;
        pulseAnimation = state.isListening || state.isSpeaking;
      });

      window.runtime.EventsOn('audio:vadActive', () => {
        pulseAnimation = true;
      });

      window.runtime.EventsOn('audio:vadInactive', () => {
        pulseAnimation = false;
      });
    }
  });

  onDestroy(() => {
    if (window.runtime) {
      window.runtime.EventsOff('audio:stateChanged');
      window.runtime.EventsOff('audio:vadActive');
      window.runtime.EventsOff('audio:vadInactive');
    }
  });

  // Size mappings
  $: sizeClass = {
    small: 'w-12 h-12',
    medium: 'w-16 h-16',
    large: 'w-20 h-20',
  }[size];

  // Color based on state
  $: buttonColor = isPressed || isActive
    ? 'bg-red-500 hover:bg-red-600'
    : 'bg-blue-500 hover:bg-blue-600';
</script>

<button
  class="voice-button {sizeClass} {buttonColor} rounded-full flex items-center justify-center transition-all duration-200 relative shadow-lg"
  class:pulse={pulseAnimation}
  class:pressed={isPressed || isActive}
  class:disabled={disabled}
  on:mousedown={handleMouseDown}
  on:mouseup={handleMouseUp}
  on:mouseleave={handleMouseUp}
  on:click={handleClick}
  {disabled}
  aria-label={mode === 'push-to-talk' ? 'Hold to talk' : 'Toggle voice input'}
>
  <!-- Microphone icon -->
  <svg
    class="w-1/2 h-1/2 text-white"
    fill="none"
    stroke="currentColor"
    viewBox="0 0 24 24"
  >
    {#if isPressed || isActive}
      <!-- Active microphone -->
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
      />
    {:else}
      <!-- Inactive microphone -->
      <path
        stroke-linecap="round"
        stroke-linejoin="round"
        stroke-width="2"
        d="M19 11a7 7 0 01-7 7m0 0a7 7 0 01-7-7m7 7v4m0 0H8m4 0h4m-4-8a3 3 0 01-3-3V5a3 3 0 116 0v6a3 3 0 01-3 3z"
      />
    {/if}
  </svg>

  <!-- Pulse ring animation -->
  {#if pulseAnimation}
    <div class="pulse-ring absolute inset-0 rounded-full"></div>
  {/if}
</button>

<style>
  .voice-button {
    user-select: none;
    -webkit-tap-highlight-color: transparent;
    cursor: pointer;
  }

  .voice-button.disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .voice-button.pressed {
    transform: scale(0.95);
  }

  .voice-button:active:not(.disabled) {
    transform: scale(0.95);
  }

  .pulse {
    animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
  }

  .pulse-ring {
    border: 2px solid currentColor;
    opacity: 0;
    animation: pulse-ring 1.5s cubic-bezier(0, 0, 0.2, 1) infinite;
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.7;
    }
  }

  @keyframes pulse-ring {
    0% {
      transform: scale(1);
      opacity: 0.6;
    }
    100% {
      transform: scale(1.5);
      opacity: 0;
    }
  }
</style>

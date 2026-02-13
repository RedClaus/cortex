<script lang="ts">
  import { onMount } from 'svelte';
  import Avatar from './lib/Avatar.svelte';
  import AvatarCanvas from './lib/AvatarCanvas.svelte';
  import Controls from './lib/Controls.svelte';
  import StatusBar from './lib/StatusBar.svelte';
  import CameraPreview from './lib/CameraPreview.svelte';
  import Notifications from './lib/Notifications.svelte';
  import { avatarState } from './stores/avatar';
  import { connectionState } from './stores/connection';
  import { audioState } from './stores/audio';
  import type { ChemicalState } from './lib/BlendshapeController';

  // Avatar mode: '2d' or '3d'
  let avatarMode: '2d' | '3d' = '3d';
  let showDebug = false;

  // Chemical state from CortexBrain (limbic system)
  let chemicalState: ChemicalState | null = null;

  // VRM model URL - based on persona
  let currentPersona = 'hannah';
  $: modelUrl = `/models/${currentPersona}.vrm`;

  // Wails runtime will be injected
  declare global {
    interface Window {
      runtime: {
        EventsOn: (event: string, callback: (...args: unknown[]) => void) => void;
        EventsOff: (event: string) => void;
      };
    }
  }

  // Listen for events from Go backend
  $: if (typeof window !== 'undefined' && window.runtime) {
    window.runtime.EventsOn('avatar:stateChanged', (state: unknown) => {
      avatarState.set(state as typeof $avatarState);
    });

    window.runtime.EventsOn('connection:status', (status: unknown) => {
      connectionState.update(s => ({ ...s, ...(status as object) }));
    });

    window.runtime.EventsOn('limbic:chemicalState', (state: unknown) => {
      chemicalState = state as ChemicalState;
    });

    window.runtime.EventsOn('avatar:setMode', (mode: unknown) => {
      if (mode === '2d' || mode === '3d') {
        avatarMode = mode;
      }
    });

    window.runtime.EventsOn('persona:changed', (persona: unknown) => {
      if (persona && typeof persona === 'object' && 'id' in persona) {
        currentPersona = (persona as { id: string }).id;
      }
    });

    window.runtime.EventsOn('audio:speaking', (speaking: unknown) => {
      console.log('[App] audio:speaking event received:', speaking);
      audioState.update(s => ({ ...s, isSpeaking: speaking as boolean }));
    });

    window.runtime.EventsOn('cortex:response', (text: unknown) => {
      console.log('[App] cortex:response event received:', (text as string)?.substring(0, 50));
      audioState.update(s => ({ ...s, lastResponse: text as string }));
    });
  }

  // Keyboard shortcuts
  onMount(() => {
    const handleKeydown = (e: KeyboardEvent) => {
      // Ctrl/Cmd + Shift + A: Toggle avatar mode
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'A') {
        avatarMode = avatarMode === '2d' ? '3d' : '2d';
        e.preventDefault();
      }
      // Ctrl/Cmd + Shift + D: Toggle debug mode
      if ((e.ctrlKey || e.metaKey) && e.shiftKey && e.key === 'D') {
        showDebug = !showDebug;
        e.preventDefault();
      }
    };

    window.addEventListener('keydown', handleKeydown);
    return () => window.removeEventListener('keydown', handleKeydown);
  });
</script>

<main class="container">
  <Notifications />
  <StatusBar />

  <!-- Avatar mode toggle -->
  <button
    class="mode-toggle"
    on:click={() => avatarMode = avatarMode === '2d' ? '3d' : '2d'}
    title="Toggle 2D/3D avatar (Ctrl+Shift+A)"
  >
    {avatarMode.toUpperCase()}
  </button>

  <div class="avatar-container">
    {#if avatarMode === '3d'}
      {#key modelUrl}
        <AvatarCanvas {modelUrl} {chemicalState} {showDebug} />
      {/key}
    {:else}
      <Avatar />
    {/if}
    <CameraPreview />
  </div>
  <Controls />
</main>

<style>
  .container {
    display: flex;
    flex-direction: column;
    height: 100vh;
    width: 100vw;
    background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
    color: white;
    position: relative;
  }

  .mode-toggle {
    position: absolute;
    top: 50px;
    right: 10px;
    z-index: 100;
    padding: 6px 12px;
    background: rgba(255, 255, 255, 0.1);
    border: 1px solid rgba(255, 255, 255, 0.3);
    border-radius: 8px;
    color: white;
    font-size: 12px;
    font-weight: bold;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .mode-toggle:hover {
    background: rgba(255, 255, 255, 0.2);
    border-color: rgba(255, 255, 255, 0.5);
  }

  .avatar-container {
    position: relative;
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 10px;
    min-height: 0;
    overflow: hidden;
  }
</style>

<script lang="ts">
  import { onMount } from 'svelte';
  import { Canvas } from '@threlte/core';
  import Avatar3D from './Avatar3D.svelte';
  import type { ChemicalState } from './BlendshapeController';

  export let modelUrl: string = '/models/avatar.vrm';
  export let chemicalState: ChemicalState | null = null;
  export let showDebug: boolean = false;
  export let backgroundUrl: string = '/backgrounds/office.jpeg';

  // Track container dimensions for resize handling
  let containerEl: HTMLDivElement;
  let canvasKey = 0;

  onMount(() => {
    // Use ResizeObserver to handle container size changes
    const resizeObserver = new ResizeObserver(() => {
      // Force canvas re-render on resize by updating key
      canvasKey += 1;
    });

    if (containerEl) {
      resizeObserver.observe(containerEl);
    }

    return () => {
      resizeObserver.disconnect();
    };
  });
</script>

<div 
  class="avatar-canvas-wrapper" 
  bind:this={containerEl}
  style="background-image: url({backgroundUrl}); background-size: cover; background-position: center;"
>
  {#key canvasKey}
    <Canvas autoRender={true}>
      <Avatar3D {modelUrl} {chemicalState} {showDebug} />
    </Canvas>
  {/key}
</div>

<style>
  .avatar-canvas-wrapper {
    width: 100%;
    height: 100%;
    min-height: 200px;
    max-height: calc(100vh - 200px);
    position: relative;
    background-color: #1a1a2e;
    border-radius: 12px;
    overflow: hidden;
  }

  .avatar-canvas-wrapper :global(canvas) {
    width: 100% !important;
    height: 100% !important;
    display: block;
  }
</style>

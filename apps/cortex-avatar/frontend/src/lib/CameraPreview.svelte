<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { audioState } from '../stores/audio';

  let videoElement: HTMLVideoElement;
  let stream: MediaStream | null = null;

  // Watch camera state
  $: if ($audioState.cameraEnabled) {
    startPreview();
  } else {
    stopPreview();
  }

  async function startPreview() {
    if (stream || !videoElement) return;

    try {
      stream = await navigator.mediaDevices.getUserMedia({
        video: { width: 160, height: 120, facingMode: 'user' }
      });
      if (videoElement) {
        videoElement.srcObject = stream;
        await videoElement.play();
      }
    } catch (err) {
      console.error('[CameraPreview] Failed to start:', err);
    }
  }

  function stopPreview() {
    if (stream) {
      stream.getTracks().forEach(track => track.stop());
      stream = null;
    }
    if (videoElement) {
      videoElement.srcObject = null;
    }
  }

  onMount(() => {
    if ($audioState.cameraEnabled) {
      startPreview();
    }
  });

  onDestroy(() => {
    stopPreview();
  });
</script>

{#if $audioState.cameraEnabled}
  <div class="camera-preview">
    <video bind:this={videoElement} autoplay muted playsinline></video>
    <div class="camera-label">Camera Preview</div>
  </div>
{/if}

<style>
  .camera-preview {
    position: absolute;
    top: 50px;
    right: 10px;
    width: 120px;
    height: 90px;
    border-radius: 8px;
    overflow: hidden;
    box-shadow: 0 4px 15px rgba(0, 0, 0, 0.5);
    z-index: 50;
    background: #000;
  }

  video {
    width: 100%;
    height: 100%;
    object-fit: cover;
    transform: scaleX(-1); /* Mirror for selfie view */
  }

  .camera-label {
    position: absolute;
    bottom: 0;
    left: 0;
    right: 0;
    padding: 4px;
    background: rgba(0, 0, 0, 0.6);
    color: rgba(255, 255, 255, 0.8);
    font-size: 9px;
    text-align: center;
  }
</style>

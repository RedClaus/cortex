<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { audioState } from '../stores/audio';

  // Props
  export let autoPlay = true;
  export let visualize = true;
  export let volume = 0.8;

  // State
  let audioQueue: ArrayBuffer[] = [];
  let currentAudio: HTMLAudioElement | null = null;
  let currentSource: MediaElementAudioSourceNode | null = null;
  let isPlaying = false;
  let audioContext: AudioContext | null = null;
  let analyser: AnalyserNode | null = null;
  let canvas: HTMLCanvasElement;
  let canvasCtx: CanvasRenderingContext2D | null = null;
  let animationId: number | null = null;
  let dataArray: Uint8Array;
  let bufferLength: number;

  // Wails runtime bindings
  declare global {
    interface Window {
      runtime: {
        EventsOn: (event: string, callback: (...args: any[]) => void) => void;
        EventsOff: (event: string) => void;
      };
    }
  }

  // Handle incoming audio chunks from Go backend
  function handleAudioChunk(audioData: string) {
    // Convert base64 to ArrayBuffer
    const binaryString = atob(audioData);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }

    // Add to queue
    audioQueue.push(bytes.buffer);

    // Start playback if auto-play enabled
    if (autoPlay && !isPlaying) {
      playNextChunk();
    }
  }

  // Handle complete audio response from Go backend (WAV format)
  function handleAudioResponse(audioBase64: string) {
    // Convert base64 to blob
    const binaryString = atob(audioBase64);
    const bytes = new Uint8Array(binaryString.length);
    for (let i = 0; i < binaryString.length; i++) {
      bytes[i] = binaryString.charCodeAt(i);
    }

    const blob = new Blob([bytes], { type: 'audio/wav' });
    const url = URL.createObjectURL(blob);

    // Play audio
    playAudioUrl(url);
  }

  // Play audio from URL
  async function playAudioUrl(url: string) {
    try {
      // Clear queue (new audio supersedes queued chunks)
      audioQueue = [];

      // Disconnect and clean up previous audio source
      if (currentSource) {
        try {
          currentSource.disconnect();
        } catch (e) {
          console.warn('[AudioPlayer] Failed to disconnect previous source:', e);
        }
        currentSource = null;
      }

      // Stop current audio
      if (currentAudio) {
        currentAudio.pause();
        currentAudio = null;
      }

      // Create new audio element
      currentAudio = new Audio(url);
      currentAudio.volume = volume;

      // Set up audio context for visualization (only once)
      if (visualize && !audioContext) {
        audioContext = new AudioContext();
        analyser = audioContext.createAnalyser();
        analyser.fftSize = 2048;
        bufferLength = analyser.frequencyBinCount;
        dataArray = new Uint8Array(bufferLength);
      }

      // Connect new audio element to analyser
      if (visualize && audioContext && analyser) {
        currentSource = audioContext.createMediaElementSource(currentAudio);
        currentSource.connect(analyser);
        analyser.connect(audioContext.destination);
      }

      // Event handlers
      currentAudio.onplay = () => {
        isPlaying = true;
        audioState.update(s => ({ ...s, isSpeaking: true, currentAudioElement: currentAudio }));

        if (visualize) {
          drawVisualization();
        }
      };

      currentAudio.onended = () => {
        isPlaying = false;
        audioState.update(s => ({ ...s, isSpeaking: false, currentAudioElement: null }));

        if (animationId) {
          cancelAnimationFrame(animationId);
          animationId = null;
        }

        // Clean up object URL
        URL.revokeObjectURL(url);

        // Play next chunk if available
        if (audioQueue.length > 0) {
          playNextChunk();
        }
      };

      currentAudio.onerror = (e) => {
        console.error('[AudioPlayer] Playback error:', e);
        isPlaying = false;
        audioState.update(s => ({ ...s, isSpeaking: false }));
      };

      // Start playback
      await currentAudio.play();
    } catch (error) {
      console.error('[AudioPlayer] Failed to play audio:', error);
      isPlaying = false;
    }
  }

  // Play next audio chunk from queue
  function playNextChunk() {
    if (audioQueue.length === 0) {
      isPlaying = false;
      return;
    }

    const chunk = audioQueue.shift();
    if (!chunk) return;

    // Convert chunk to WAV blob
    const blob = new Blob([chunk], { type: 'audio/wav' });
    const url = URL.createObjectURL(blob);

    playAudioUrl(url);
  }

  // Stop playback
  function stopPlayback() {
    // Disconnect audio source
    if (currentSource) {
      try {
        currentSource.disconnect();
      } catch (e) {
        console.warn('[AudioPlayer] Failed to disconnect source:', e);
      }
      currentSource = null;
    }

    if (currentAudio) {
      currentAudio.pause();
      currentAudio = null;
    }

    audioQueue = [];
    isPlaying = false;

    if (animationId) {
      cancelAnimationFrame(animationId);
      animationId = null;
    }

    audioState.update(s => ({ ...s, isSpeaking: false }));
  }

  // Draw audio visualization
  function drawVisualization() {
    if (!analyser || !canvasCtx || !canvas) return;

    animationId = requestAnimationFrame(drawVisualization);

    analyser.getByteFrequencyData(dataArray);

    canvasCtx.fillStyle = 'rgb(20, 20, 30)';
    canvasCtx.fillRect(0, 0, canvas.width, canvas.height);

    const barWidth = (canvas.width / bufferLength) * 2.5;
    let barHeight;
    let x = 0;

    for (let i = 0; i < bufferLength; i++) {
      barHeight = (dataArray[i] / 255) * canvas.height;

      // Gradient from blue to purple
      const gradient = canvasCtx.createLinearGradient(0, canvas.height - barHeight, 0, canvas.height);
      gradient.addColorStop(0, 'rgb(147, 51, 234)'); // purple-600
      gradient.addColorStop(1, 'rgb(59, 130, 246)'); // blue-500

      canvasCtx.fillStyle = gradient;
      canvasCtx.fillRect(x, canvas.height - barHeight, barWidth, barHeight);

      x += barWidth + 1;
    }
  }

  // Listen for events from Go backend
  onMount(() => {
    if (canvas && visualize) {
      canvasCtx = canvas.getContext('2d');
    }

    if (window.runtime) {
      // Listen for streaming audio chunks
      window.runtime.EventsOn('audio:chunk', handleAudioChunk);

      // Listen for complete audio response
      window.runtime.EventsOn('audio:response', handleAudioResponse);

      // Listen for playback control
      window.runtime.EventsOn('audio:stop', stopPlayback);
    }
  });

  onDestroy(() => {
    stopPlayback();

    // Clean up audio context
    if (analyser) {
      analyser.disconnect();
      analyser = null;
    }

    if (audioContext) {
      audioContext.close();
      audioContext = null;
    }

    // Clean up event listeners
    if (window.runtime) {
      window.runtime.EventsOff('audio:chunk');
      window.runtime.EventsOff('audio:response');
      window.runtime.EventsOff('audio:stop');
    }
  });

  // Update volume when prop changes
  $: if (currentAudio) {
    currentAudio.volume = volume;
  }
</script>

<div class="audio-player">
  {#if visualize}
    <canvas
      bind:this={canvas}
      width="400"
      height="100"
      class="visualization-canvas rounded-lg shadow-md"
    />
  {/if}

  <div class="player-controls">
    <div class="flex items-center justify-between">
      <div class="flex items-center gap-2">
        <div
          class="status-indicator"
          class:playing={isPlaying}
        ></div>
        <span class="text-sm text-gray-300">
          {isPlaying ? 'Speaking...' : 'Silent'}
        </span>
      </div>

      <div class="flex items-center gap-2">
        <!-- Volume control -->
        <label class="flex items-center gap-2 text-sm text-gray-400">
          <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path
              stroke-linecap="round"
              stroke-linejoin="round"
              stroke-width="2"
              d="M15.536 8.464a5 5 0 010 7.072m2.828-9.9a9 9 0 010 12.728M5.586 15H4a1 1 0 01-1-1v-4a1 1 0 011-1h1.586l4.707-4.707C10.923 3.663 12 4.109 12 5v14c0 .891-1.077 1.337-1.707.707L5.586 15z"
            />
          </svg>
          <input
            type="range"
            min="0"
            max="1"
            step="0.1"
            bind:value={volume}
            class="volume-slider"
          />
        </label>

        <!-- Stop button (if playing) -->
        {#if isPlaying}
          <button
            on:click={stopPlayback}
            class="stop-button px-2 py-1 bg-red-500 hover:bg-red-600 rounded text-white text-xs"
            aria-label="Stop playback"
          >
            Stop
          </button>
        {/if}
      </div>
    </div>
  </div>

  {#if $audioState.lastResponse}
    <div class="response-text mt-2 p-2 bg-gray-800 rounded text-sm text-gray-200">
      <strong>Response:</strong> {$audioState.lastResponse}
    </div>
  {/if}
</div>

<style>
  .audio-player {
    display: flex;
    flex-direction: column;
    gap: 0.75rem;
  }

  .visualization-canvas {
    width: 100%;
    background: rgb(20, 20, 30);
  }

  .player-controls {
    padding: 0.5rem;
  }

  .status-indicator {
    width: 0.75rem;
    height: 0.75rem;
    border-radius: 50%;
    background-color: rgb(107, 114, 128); /* gray-500 */
    transition: background-color 0.3s;
  }

  .status-indicator.playing {
    background-color: rgb(147, 51, 234); /* purple-600 */
    animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
  }

  .volume-slider {
    width: 80px;
    accent-color: rgb(147, 51, 234); /* purple-600 */
  }

  .stop-button {
    transition: background-color 0.2s;
  }

  .response-text {
    animation: slideIn 0.3s ease-out;
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.6;
    }
  }

  @keyframes slideIn {
    from {
      opacity: 0;
      transform: translateY(-10px);
    }
    to {
      opacity: 1;
      transform: translateY(0);
    }
  }
</style>

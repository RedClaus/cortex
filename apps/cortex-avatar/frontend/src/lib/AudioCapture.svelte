<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { audioState } from '../stores/audio';

  // Props
  export let vadEnabled = true;
  export let sampleRate = 16000;
  export let chunkSize = 4096;
  export let visualize = true;

  // State
  let audioContext: AudioContext | null = null;
  let mediaStream: MediaStream | null = null;
  let analyser: AnalyserNode | null = null;
  let processor: ScriptProcessorNode | null = null;
  let canvas: HTMLCanvasElement;
  let canvasCtx: CanvasRenderingContext2D | null = null;
  let animationId: number | null = null;

  // Audio visualization data
  let dataArray: Uint8Array;
  let bufferLength: number;

  // Wails runtime bindings
  declare global {
    interface Window {
      runtime: {
        EventsEmit: (event: string, ...args: any[]) => void;
        EventsOn: (event: string, callback: (...args: any[]) => void) => void;
      };
      go: {
        bridge: {
          AudioManager: {
            ProcessAudioChunk: (audioBase64: string, isSpeech: boolean, rms: float) => Promise<void>;
          };
        };
      };
    }
  }

  // Initialize audio capture
  async function initAudioCapture() {
    try {
      // Request microphone access
      mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          sampleRate,
          channelCount: 1,
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        },
      });

      // Create audio context
      audioContext = new AudioContext({ sampleRate });
      const source = audioContext.createMediaStreamSource(mediaStream);

      // Create analyser for visualization
      if (visualize) {
        analyser = audioContext.createAnalyser();
        analyser.fftSize = 2048;
        bufferLength = analyser.frequencyBinCount;
        dataArray = new Uint8Array(bufferLength);
        source.connect(analyser);
      }

      // Create processor for audio chunks
      processor = audioContext.createScriptProcessor(chunkSize, 1, 1);

      processor.onaudioprocess = (event) => {
        const inputData = event.inputBuffer.getChannelData(0);

        // Calculate RMS (volume level)
        const rms = calculateRMS(inputData);

        // Convert Float32Array to Int16Array (PCM 16-bit)
        const pcm16 = convertToPCM16(inputData);

        // Convert to base64 for Go backend
        const audioBase64 = arrayBufferToBase64(pcm16.buffer);

        // Simple VAD: check if RMS exceeds threshold
        const isSpeech = vadEnabled ? rms > 0.01 : true;

        // Send to Go backend for processing
        if (window.go?.bridge?.AudioManager) {
          window.go.bridge.AudioManager.ProcessAudioChunk(audioBase64, isSpeech, rms);
        }
      };

      source.connect(processor);
      processor.connect(audioContext.destination);

      // Update state
      audioState.update(s => ({ ...s, micEnabled: true }));

      // Start visualization
      if (visualize) {
        drawWaveform();
      }

      console.log('[AudioCapture] Audio capture initialized');
    } catch (error) {
      console.error('[AudioCapture] Failed to initialize:', error);
      audioState.update(s => ({ ...s, micEnabled: false }));
    }
  }

  // Stop audio capture
  function stopAudioCapture() {
    if (processor) {
      processor.disconnect();
      processor = null;
    }

    if (analyser) {
      analyser.disconnect();
      analyser = null;
    }

    if (mediaStream) {
      mediaStream.getTracks().forEach(track => track.stop());
      mediaStream = null;
    }

    if (audioContext) {
      audioContext.close();
      audioContext = null;
    }

    if (animationId) {
      cancelAnimationFrame(animationId);
      animationId = null;
    }

    audioState.update(s => ({ ...s, micEnabled: false }));

    console.log('[AudioCapture] Audio capture stopped');
  }

  // Calculate RMS (Root Mean Square) for volume level
  function calculateRMS(buffer: Float32Array): number {
    let sum = 0;
    for (let i = 0; i < buffer.length; i++) {
      sum += buffer[i] * buffer[i];
    }
    return Math.sqrt(sum / buffer.length);
  }

  // Convert Float32Array to PCM 16-bit Int16Array
  function convertToPCM16(float32Array: Float32Array): Int16Array {
    const pcm16 = new Int16Array(float32Array.length);
    for (let i = 0; i < float32Array.length; i++) {
      const s = Math.max(-1, Math.min(1, float32Array[i]));
      pcm16[i] = s < 0 ? s * 0x8000 : s * 0x7fff;
    }
    return pcm16;
  }

  // Convert ArrayBuffer to Base64
  function arrayBufferToBase64(buffer: ArrayBuffer): string {
    const bytes = new Uint8Array(buffer);
    let binary = '';
    for (let i = 0; i < bytes.byteLength; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    return btoa(binary);
  }

  // Draw waveform visualization
  function drawWaveform() {
    if (!analyser || !canvasCtx || !canvas) return;

    animationId = requestAnimationFrame(drawWaveform);

    analyser.getByteTimeDomainData(dataArray);

    canvasCtx.fillStyle = 'rgb(20, 20, 30)';
    canvasCtx.fillRect(0, 0, canvas.width, canvas.height);

    canvasCtx.lineWidth = 2;
    canvasCtx.strokeStyle = $audioState.vadActive ? 'rgb(34, 197, 94)' : 'rgb(59, 130, 246)';

    canvasCtx.beginPath();

    const sliceWidth = (canvas.width * 1.0) / bufferLength;
    let x = 0;

    for (let i = 0; i < bufferLength; i++) {
      const v = dataArray[i] / 128.0;
      const y = (v * canvas.height) / 2;

      if (i === 0) {
        canvasCtx.moveTo(x, y);
      } else {
        canvasCtx.lineTo(x, y);
      }

      x += sliceWidth;
    }

    canvasCtx.lineTo(canvas.width, canvas.height / 2);
    canvasCtx.stroke();
  }

  // Listen for events from Go backend
  onMount(() => {
    if (canvas && visualize) {
      canvasCtx = canvas.getContext('2d');
    }

    if (window.runtime) {
      window.runtime.EventsOn('audio:startListening', initAudioCapture);
      window.runtime.EventsOn('audio:stopListening', stopAudioCapture);
    }
  });

  onDestroy(() => {
    stopAudioCapture();

    if (window.runtime) {
      window.runtime.EventsOff('audio:startListening');
      window.runtime.EventsOff('audio:stopListening');
    }
  });
</script>

<div class="audio-capture">
  {#if visualize}
    <canvas
      bind:this={canvas}
      width="400"
      height="100"
      class="waveform-canvas rounded-lg shadow-md"
    />
  {/if}

  <div class="audio-status">
    <div class="flex items-center gap-2 text-sm">
      <div
        class="status-dot"
        class:active={$audioState.micEnabled}
        class:speech={$audioState.vadActive}
      ></div>
      <span class="text-gray-300">
        {$audioState.micEnabled ? ($audioState.vadActive ? 'Speech detected' : 'Listening...') : 'Microphone off'}
      </span>
    </div>

    {#if $audioState.transcript}
      <div class="transcript mt-2 p-2 bg-gray-800 rounded text-sm text-gray-200">
        <strong>Transcript:</strong> {$audioState.transcript}
      </div>
    {/if}
  </div>
</div>

<style>
  .audio-capture {
    display: flex;
    flex-direction: column;
    gap: 1rem;
  }

  .waveform-canvas {
    width: 100%;
    background: rgb(20, 20, 30);
  }

  .audio-status {
    padding: 0.5rem;
  }

  .status-dot {
    width: 0.75rem;
    height: 0.75rem;
    border-radius: 50%;
    background-color: rgb(107, 114, 128); /* gray-500 */
    transition: background-color 0.3s;
  }

  .status-dot.active {
    background-color: rgb(59, 130, 246); /* blue-500 */
    animation: pulse 2s cubic-bezier(0.4, 0, 0.6, 1) infinite;
  }

  .status-dot.speech {
    background-color: rgb(34, 197, 94); /* green-500 */
  }

  .transcript {
    animation: slideIn 0.3s ease-out;
  }

  @keyframes pulse {
    0%,
    100% {
      opacity: 1;
    }
    50% {
      opacity: 0.5;
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

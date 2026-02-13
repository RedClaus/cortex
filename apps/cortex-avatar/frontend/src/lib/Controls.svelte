<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { audioState, audioCaptureManager, speechRecognitionManager, initAudioEvents, audioPlaybackManager, sendTextMessage } from '../stores/audio';
  import Settings from './Settings.svelte';

  let textInput = '';
  let isSending = false;

  async function handleSendMessage() {
    if (!textInput.trim() || isSending) return;

    const message = textInput.trim();
    textInput = '';
    isSending = true;

    try {
      await sendTextMessage(message);
    } catch (e) {
      console.error('[Controls] Failed to send message:', e);
    } finally {
      isSending = false;
    }
  }

  function handleKeyPress(event: KeyboardEvent) {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      handleSendMessage();
    }
  }

  interface Persona {
    id: string;
    name: string;
    gender: string;
    voice_id: string;
  }

  let personas: Persona[] = [];
  let currentPersona: Persona | null = null;
  let showPersonaMenu = false;
  let showSettings = false;
  let showVolumeSlider = false;

  // Camera capture
  let cameraStream: MediaStream | null = null;
  let cameraInterval: number | null = null;

  interface SettingsData {
    microphoneId: string;
    speakerId: string;
    vadThreshold: number;
    outputVolume: number;
    cameraId: string;
    maxFps: number;
    jpegQuality: number;
    serverUrl: string;
    persona: string;
  }

  // Wails bindings (will be generated)
  declare const go: {
    bridge: {
      AudioBridge: {
        StartListening: () => Promise<void>;
        StopListening: () => Promise<void>;
        ToggleMic: () => Promise<boolean>;
        ToggleSpeaker: () => Promise<boolean>;
        ToggleCamera: () => Promise<boolean>;
        ToggleScreenShare: () => Promise<boolean>;
        GetPersonas: () => Promise<Persona[]>;
        GetCurrentPersona: () => Promise<Persona | null>;
        SetPersona: (id: string) => Promise<void>;
      };
      SettingsBridge: {
        GetSettings: () => Promise<SettingsData>;
        SetVolume: (volume: number) => Promise<void>;
      };
    };
  };

  onMount(async () => {
    // Initialize audio events
    initAudioEvents();

    // Load saved settings (including volume)
    if (typeof go !== 'undefined' && go.bridge?.SettingsBridge?.GetSettings) {
      try {
        const settings = await go.bridge.SettingsBridge.GetSettings();
        console.log('[Controls] Loaded saved settings:', settings);
        // Apply saved volume
        if (settings.outputVolume !== undefined) {
          audioState.update(s => ({ ...s, outputVolume: settings.outputVolume }));
        }
      } catch (e) {
        console.error('[Controls] Failed to load settings:', e);
      }
    }

    // Load personas
    if (typeof go !== 'undefined') {
      try {
        personas = await go.bridge.AudioBridge.GetPersonas();
        currentPersona = await go.bridge.AudioBridge.GetCurrentPersona();
      } catch (e) {
        console.error('Failed to load personas:', e);
        // Default personas
        personas = [
          { id: 'henry', name: 'Henry', gender: 'male', voice_id: 'am_adam' },
          { id: 'hannah', name: 'Hannah', gender: 'female', voice_id: 'af_bella' },
        ];
        currentPersona = personas[1]; // Default to Hannah
      }
    } else {
      // Dev mode defaults
      personas = [
        { id: 'henry', name: 'Henry', gender: 'male', voice_id: 'am_adam' },
        { id: 'hannah', name: 'Hannah', gender: 'female', voice_id: 'af_bella' },
      ];
      currentPersona = personas[1];
    }
  });

  onDestroy(() => {
    stopCameraCapture();
    speechRecognitionManager.stop();
    audioCaptureManager.stopCapture();
  });

  // Camera capture functions
  async function startCameraCapture(): Promise<boolean> {
    try {
      cameraStream = await navigator.mediaDevices.getUserMedia({
        video: { width: 640, height: 480, facingMode: 'user' }
      });

      // Create hidden video element to capture frames
      const video = document.createElement('video');
      video.srcObject = cameraStream;
      video.autoplay = true;
      video.muted = true;
      await video.play();

      // Create canvas for frame capture
      const canvas = document.createElement('canvas');
      canvas.width = 640;
      canvas.height = 480;
      const ctx = canvas.getContext('2d');

      // Capture frames every 500ms (2 FPS for vision analysis)
      cameraInterval = window.setInterval(() => {
        if (ctx && video.readyState >= video.HAVE_CURRENT_DATA) {
          ctx.drawImage(video, 0, 0, 640, 480);
          const dataUrl = canvas.toDataURL('image/jpeg', 0.7);
          const base64 = dataUrl.split(',')[1];

          // Send to Go backend
          if (typeof go !== 'undefined' && go.bridge?.AudioBridge?.ProcessCameraFrame) {
            go.bridge.AudioBridge.ProcessCameraFrame(base64, 640, 480);
          }
        }
      }, 500);

      console.log('[Camera] Capture started');
      return true;
    } catch (err) {
      console.error('[Camera] Failed to start capture:', err);
      return false;
    }
  }

  function stopCameraCapture(): void {
    if (cameraInterval) {
      clearInterval(cameraInterval);
      cameraInterval = null;
    }
    if (cameraStream) {
      cameraStream.getTracks().forEach(track => track.stop());
      cameraStream = null;
    }
    console.log('[Camera] Capture stopped');
  }

  async function toggleMic() {
    // Visual feedback immediately
    audioState.update(s => ({ ...s, transcript: 'Mic button clicked...' }));

    try {
      const newState = !$audioState.micEnabled;
      console.log('[Controls] toggleMic called, newState:', newState);

      if (newState) {
        console.log('[Controls] Enabling microphone...');

        // Update state first so UI shows enabled
        audioState.update(s => ({ ...s, micEnabled: true }));

        // Start speech recognition (handles STT directly)
        try {
          console.log('[Controls] Starting speech recognition...');
          audioState.update(s => ({ ...s, transcript: 'Starting speech recognition...' }));
          await speechRecognitionManager.start();
          console.log('[Controls] Speech recognition started');
          audioState.update(s => ({ ...s, transcript: 'Listening... Speak now!' }));
        } catch (sttErr) {
          console.error('[Controls] Speech recognition failed:', sttErr);
          audioState.update(s => ({ ...s, transcript: 'STT Error: ' + String(sttErr) }));
          // Continue anyway - STT might not be available
        }

        // Also start audio capture for volume visualization
        try {
          console.log('[Controls] Starting audio capture...');
          await audioCaptureManager.startCapture();
          console.log('[Controls] Audio capture started');
        } catch (audioErr) {
          console.error('[Controls] Audio capture failed:', audioErr);
        }

        // Notify Go backend
        if (typeof go !== 'undefined') {
          await go.bridge.AudioBridge.StartListening();
        }
        console.log('[Controls] Microphone enabled successfully');
      } else {
        console.log('[Controls] Disabling microphone...');
        audioState.update(s => ({ ...s, micEnabled: false }));
        // Stop speech recognition
        speechRecognitionManager.stop();
        // Stop audio capture
        audioCaptureManager.stopCapture();
        if (typeof go !== 'undefined') {
          await go.bridge.AudioBridge.StopListening();
        }
        console.log('[Controls] Microphone disabled');
      }
    } catch (e) {
      console.error('[Controls] Failed to toggle mic:', e);
      audioState.update(s => ({ ...s, micEnabled: false, transcript: 'Mic error: ' + String(e) }));
    }
  }

  async function toggleSpeaker() {
    try {
      if (typeof go !== 'undefined') {
        const enabled = await go.bridge.AudioBridge.ToggleSpeaker();
        audioState.update(s => ({ ...s, speakerEnabled: enabled }));
      } else {
        audioState.update(s => ({ ...s, speakerEnabled: !s.speakerEnabled }));
      }
    } catch (e) {
      console.error('Failed to toggle speaker:', e);
    }
  }

  async function toggleCamera() {
    try {
      const newState = !$audioState.cameraEnabled;
      audioState.update(s => ({ ...s, cameraEnabled: newState }));

      if (newState) {
        // Start camera capture
        const success = await startCameraCapture();
        if (!success) {
          audioState.update(s => ({ ...s, cameraEnabled: false }));
          return;
        }
      } else {
        // Stop camera capture
        stopCameraCapture();
      }

      // Notify Go backend
      if (typeof go !== 'undefined') {
        await go.bridge.AudioBridge.ToggleCamera();
      }
    } catch (e) {
      console.error('Failed to toggle camera:', e);
      audioState.update(s => ({ ...s, cameraEnabled: false }));
    }
  }

  function handleVolumeChange(event: Event) {
    const target = event.target as HTMLInputElement;
    const newVolume = parseInt(target.value, 10);
    audioState.update(s => ({ ...s, outputVolume: newVolume }));
    console.log('[Audio] Volume set to:', newVolume);
    // Save to backend
    if (typeof go !== 'undefined' && go.bridge?.SettingsBridge?.SetVolume) {
      go.bridge.SettingsBridge.SetVolume(newVolume).catch(e => {
        console.error('[Audio] Failed to save volume:', e);
      });
    }
  }

  function toggleVolumeSlider() {
    showVolumeSlider = !showVolumeSlider;
  }

  async function toggleScreenShare() {
    try {
      if (typeof go !== 'undefined') {
        const enabled = await go.bridge.AudioBridge.ToggleScreenShare();
        audioState.update(s => ({ ...s, screenShareEnabled: enabled }));
      } else {
        audioState.update(s => ({ ...s, screenShareEnabled: !s.screenShareEnabled }));
      }
    } catch (e) {
      console.error('Failed to toggle screen share:', e);
    }
  }

  async function selectPersona(persona: Persona) {
    currentPersona = persona;
    showPersonaMenu = false;
    if (typeof go !== 'undefined') {
      try {
        await go.bridge.AudioBridge.SetPersona(persona.id);
      } catch (e) {
        console.error('Failed to set persona:', e);
      }
    }
  }

  function togglePersonaMenu() {
    showPersonaMenu = !showPersonaMenu;
  }

  function openSettings() {
    showSettings = true;
  }

  function closeSettings() {
    showSettings = false;
  }

  function handleSettingsSaved() {
    // Reload personas after settings are saved
    loadPersonas();
    showSettings = false;
  }

  async function loadPersonas() {
    if (typeof go !== 'undefined') {
      try {
        personas = await go.bridge.AudioBridge.GetPersonas();
        currentPersona = await go.bridge.AudioBridge.GetCurrentPersona();
      } catch (e) {
        console.error('Failed to load personas:', e);
      }
    }
  }
</script>

<div class="controls-wrapper">
  <!-- Text input for typing messages -->
  <div class="text-input-container">
    <input
      type="text"
      class="text-input"
      placeholder="Type a message..."
      bind:value={textInput}
      on:keypress={handleKeyPress}
      disabled={isSending}
    />
    <button
      class="send-btn"
      on:click={handleSendMessage}
      disabled={!textInput.trim() || isSending}
      title="Send message"
    >
      {#if isSending}
        <svg class="spinner" viewBox="0 0 24 24" width="20" height="20">
          <circle cx="12" cy="12" r="10" stroke="currentColor" stroke-width="2" fill="none" stroke-dasharray="60" stroke-linecap="round"/>
        </svg>
      {:else}
        <svg viewBox="0 0 24 24" width="20" height="20">
          <path fill="currentColor" d="M2,21L23,12L2,3V10L17,12L2,14V21Z"/>
        </svg>
      {/if}
    </button>
  </div>

  <div class="controls">
    <!-- Persona selector -->
    <div class="persona-selector">
    <button
      class="control-btn persona-btn"
      on:click={togglePersonaMenu}
      title="Select persona"
    >
      <span class="persona-icon">{currentPersona?.gender === 'male' ? '👨' : '👩'}</span>
      <span class="persona-name">{currentPersona?.name || 'Select'}</span>
    </button>
    {#if showPersonaMenu}
      <div class="persona-menu">
        {#each personas as persona}
          <button
            class="persona-option"
            class:active={currentPersona?.id === persona.id}
            on:click={() => selectPersona(persona)}
          >
            <span>{persona.gender === 'male' ? '👨' : '👩'}</span>
            <span>{persona.name}</span>
          </button>
        {/each}
      </div>
    {/if}
  </div>

  <div class="control-buttons">
    <button
      class="control-btn"
      class:active={$audioState.micEnabled}
      class:listening={$audioState.isListening}
      class:vad-active={$audioState.vadActive}
      on:click={toggleMic}
      title={$audioState.micEnabled ? 'Mute microphone' : 'Unmute microphone'}
    >
      <svg viewBox="0 0 24 24" width="24" height="24">
        {#if $audioState.micEnabled}
          <path fill="currentColor" d="M12,2A3,3 0 0,1 15,5V11A3,3 0 0,1 12,14A3,3 0 0,1 9,11V5A3,3 0 0,1 12,2M19,11C19,14.53 16.39,17.44 13,17.93V21H11V17.93C7.61,17.44 5,14.53 5,11H7A5,5 0 0,0 12,16A5,5 0 0,0 17,11H19Z"/>
        {:else}
          <path fill="currentColor" d="M19,11C19,12.19 18.66,13.3 18.1,14.28L16.87,13.05C17.14,12.43 17.3,11.74 17.3,11H19M15,11.16L9,5.18V5A3,3 0 0,1 12,2A3,3 0 0,1 15,5V11L15,11.16M4.27,3L21,19.73L19.73,21L15.54,16.81C14.77,17.27 13.91,17.58 13,17.72V21H11V17.72C7.72,17.23 5,14.41 5,11H6.7C6.7,14 9.24,16.1 12,16.1C12.81,16.1 13.6,15.91 14.31,15.58L12.65,13.92L12,14A3,3 0 0,1 9,11V10.28L3,4.27L4.27,3Z"/>
        {/if}
      </svg>
      {#if $audioState.vadActive}
        <div class="vad-indicator"></div>
      {/if}
    </button>

    <div class="speaker-control">
      <button
        class="control-btn"
        class:active={$audioState.speakerEnabled}
        class:speaking={$audioState.isSpeaking}
        on:click={toggleSpeaker}
        on:contextmenu|preventDefault={toggleVolumeSlider}
        title={$audioState.speakerEnabled ? 'Mute speaker (right-click for volume)' : 'Unmute speaker'}
      >
        <svg viewBox="0 0 24 24" width="24" height="24">
          {#if $audioState.speakerEnabled}
            <path fill="currentColor" d="M14,3.23V5.29C16.89,6.15 19,8.83 19,12C19,15.17 16.89,17.84 14,18.7V20.77C18,19.86 21,16.28 21,12C21,7.72 18,4.14 14,3.23M16.5,12C16.5,10.23 15.5,8.71 14,7.97V16C15.5,15.29 16.5,13.76 16.5,12M3,9V15H7L12,20V4L7,9H3Z"/>
          {:else}
            <path fill="currentColor" d="M12,4L9.91,6.09L12,8.18M4.27,3L3,4.27L7.73,9H3V15H7L12,20V13.27L16.25,17.53C15.58,18.04 14.83,18.46 14,18.7V20.77C15.38,20.45 16.63,19.82 17.68,18.96L19.73,21L21,19.73L12,10.73M19,12C19,12.94 18.8,13.82 18.46,14.64L19.97,16.15C20.62,14.91 21,13.5 21,12C21,7.72 18,4.14 14,3.23V5.29C16.89,6.15 19,8.83 19,12M16.5,12C16.5,10.23 15.5,8.71 14,7.97V10.18L16.45,12.63C16.5,12.43 16.5,12.21 16.5,12Z"/>
          {/if}
        </svg>
      </button>
      {#if showVolumeSlider}
        <div class="volume-slider-popup">
          <input
            type="range"
            min="0"
            max="100"
            value={$audioState.outputVolume}
            on:input={handleVolumeChange}
            class="volume-slider"
          />
          <span class="volume-label">{$audioState.outputVolume}%</span>
        </div>
      {/if}
    </div>

    <button
      class="control-btn"
      class:active={$audioState.cameraEnabled}
      on:click={toggleCamera}
      title={$audioState.cameraEnabled ? 'Disable camera' : 'Enable camera'}
    >
      <svg viewBox="0 0 24 24" width="24" height="24">
        {#if $audioState.cameraEnabled}
          <path fill="currentColor" d="M17,10.5V7A1,1 0 0,0 16,6H4A1,1 0 0,0 3,7V17A1,1 0 0,0 4,18H16A1,1 0 0,0 17,17V13.5L21,17.5V6.5L17,10.5Z"/>
        {:else}
          <path fill="currentColor" d="M3.27,2L2,3.27L4.73,6H4A1,1 0 0,0 3,7V17A1,1 0 0,0 4,18H16C16.2,18 16.39,17.92 16.54,17.82L19.73,21L21,19.73M21,6.5L17,10.5V7A1,1 0 0,0 16,6H9.82L21,17.18V6.5Z"/>
        {/if}
      </svg>
    </button>

    <button
      class="control-btn"
      class:active={$audioState.screenShareEnabled}
      on:click={toggleScreenShare}
      title={$audioState.screenShareEnabled ? 'Stop screen share' : 'Start screen share'}
    >
      <svg viewBox="0 0 24 24" width="24" height="24">
        {#if $audioState.screenShareEnabled}
          <path fill="currentColor" d="M21,16H3V4H21M21,2H3C1.89,2 1,2.89 1,4V16A2,2 0 0,0 3,18H10V20H8V22H16V20H14V18H21A2,2 0 0,0 23,16V4C23,2.89 22.1,2 21,2Z"/>
        {:else}
          <path fill="currentColor" d="M21,16V4H3V16H21M21,2A2,2 0 0,1 23,4V16A2,2 0 0,1 21,18H14V20H16V22H8V20H10V18H3A2,2 0 0,1 1,16V4A2,2 0 0,1 3,2H21M5,6H14V8H5V6M5,10H11V12H5V10M5,14H14V16H5V14Z"/>
        {/if}
      </svg>
    </button>

    <!-- Settings button -->
    <button
      class="control-btn settings-btn"
      on:click={openSettings}
      title="Settings"
    >
      <svg viewBox="0 0 24 24" width="24" height="24">
        <path fill="currentColor" d="M12,15.5A3.5,3.5 0 0,1 8.5,12A3.5,3.5 0 0,1 12,8.5A3.5,3.5 0 0,1 15.5,12A3.5,3.5 0 0,1 12,15.5M19.43,12.97C19.47,12.65 19.5,12.33 19.5,12C19.5,11.67 19.47,11.34 19.43,11L21.54,9.37C21.73,9.22 21.78,8.95 21.66,8.73L19.66,5.27C19.54,5.05 19.27,4.96 19.05,5.05L16.56,6.05C16.04,5.66 15.5,5.32 14.87,5.07L14.5,2.42C14.46,2.18 14.25,2 14,2H10C9.75,2 9.54,2.18 9.5,2.42L9.13,5.07C8.5,5.32 7.96,5.66 7.44,6.05L4.95,5.05C4.73,4.96 4.46,5.05 4.34,5.27L2.34,8.73C2.21,8.95 2.27,9.22 2.46,9.37L4.57,11C4.53,11.34 4.5,11.67 4.5,12C4.5,12.33 4.53,12.65 4.57,12.97L2.46,14.63C2.27,14.78 2.21,15.05 2.34,15.27L4.34,18.73C4.46,18.95 4.73,19.03 4.95,18.95L7.44,17.94C7.96,18.34 8.5,18.68 9.13,18.93L9.5,21.58C9.54,21.82 9.75,22 10,22H14C14.25,22 14.46,21.82 14.5,21.58L14.87,18.93C15.5,18.67 16.04,18.34 16.56,17.94L19.05,18.95C19.27,19.03 19.54,18.95 19.66,18.73L21.66,15.27C21.78,15.05 21.73,14.78 21.54,14.63L19.43,12.97Z"/>
      </svg>
    </button>
  </div>
</div>
</div>

<!-- Settings modal -->
{#if showSettings}
  <Settings on:close={closeSettings} on:saved={handleSettingsSaved} />
{/if}

<!-- Chat display - shows transcript and response -->
<div class="chat-display">
  {#if $audioState.transcript}
    <div class="chat-bubble user">
      <span class="label">You:</span>
      <span class="text">{$audioState.transcript}</span>
    </div>
  {/if}
  {#if $audioState.lastResponse}
    <div class="chat-bubble assistant">
      <span class="label">Cortex:</span>
      <span class="text">{$audioState.lastResponse}</span>
      {#if $audioState.isSpeaking}
        <span class="speaking-indicator">🔊</span>
      {/if}
    </div>
  {/if}
</div>

<style>
  .controls-wrapper {
    display: flex;
    flex-direction: column;
    background: rgba(0, 0, 0, 0.3);
    border-radius: 12px 12px 0 0;
  }

  .controls {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 16px 20px;
  }

  .persona-selector {
    position: relative;
  }

  .persona-btn {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 16px;
    border-radius: 20px;
    width: auto;
    height: auto;
  }

  .persona-icon {
    font-size: 20px;
  }

  .persona-name {
    font-size: 14px;
    font-weight: 500;
  }

  .persona-menu {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 8px;
    background: rgba(30, 30, 40, 0.95);
    border-radius: 12px;
    padding: 8px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
    display: flex;
    gap: 8px;
  }

  .persona-option {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 4px;
    padding: 12px 16px;
    border: none;
    border-radius: 8px;
    background: transparent;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .persona-option:hover {
    background: rgba(255, 255, 255, 0.1);
    color: white;
  }

  .persona-option.active {
    background: rgba(74, 158, 255, 0.3);
    color: #4a9eff;
  }

  .control-buttons {
    display: flex;
    justify-content: center;
    gap: 12px;
    flex-wrap: wrap;
  }

  .settings-btn {
    background: #ff6b00;
    color: white;
  }

  .settings-btn:hover {
    background: #ff8533;
  }

  .control-btn {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 48px;
    height: 48px;
    border: none;
    border-radius: 50%;
    background: rgba(255, 255, 255, 0.1);
    color: rgba(255, 255, 255, 0.6);
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .control-btn:hover {
    background: rgba(255, 255, 255, 0.2);
    color: white;
  }

  .control-btn.active {
    background: rgba(74, 158, 255, 0.3);
    color: #4a9eff;
  }

  .control-btn.listening {
    animation: pulse-glow 1.5s infinite;
  }

  .control-btn.vad-active {
    background: rgba(76, 217, 100, 0.4);
    color: #4cd964;
  }

  .control-btn.speaking {
    background: rgba(255, 149, 0, 0.4);
    color: #ff9500;
  }

  .vad-indicator {
    position: absolute;
    top: -4px;
    right: -4px;
    width: 12px;
    height: 12px;
    border-radius: 50%;
    background: #4cd964;
    animation: pulse-indicator 0.5s infinite alternate;
  }

  @keyframes pulse-glow {
    0%, 100% {
      box-shadow: 0 0 0 0 rgba(74, 158, 255, 0.4);
    }
    50% {
      box-shadow: 0 0 0 10px rgba(74, 158, 255, 0);
    }
  }

  @keyframes pulse-indicator {
    from {
      transform: scale(1);
      opacity: 1;
    }
    to {
      transform: scale(1.2);
      opacity: 0.8;
    }
  }

  .chat-display {
    position: absolute;
    bottom: 100%;
    left: 0;
    right: 0;
    margin-bottom: 12px;
    padding: 8px 16px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    max-height: 150px;
    overflow-y: auto;
  }

  .chat-bubble {
    padding: 10px 14px;
    border-radius: 12px;
    font-size: 13px;
    max-width: 100%;
    word-wrap: break-word;
    animation: fadeIn 0.3s ease;
  }

  .chat-bubble.user {
    background: rgba(74, 158, 255, 0.2);
    border: 1px solid rgba(74, 158, 255, 0.3);
    align-self: flex-end;
  }

  .chat-bubble.assistant {
    background: rgba(76, 217, 100, 0.2);
    border: 1px solid rgba(76, 217, 100, 0.3);
    align-self: flex-start;
  }

  .chat-bubble .label {
    color: rgba(255, 255, 255, 0.6);
    font-weight: 600;
    margin-right: 6px;
  }

  .chat-bubble .text {
    color: white;
  }

  .speaking-indicator {
    margin-left: 8px;
    animation: pulse-indicator 0.5s infinite alternate;
  }

  @keyframes fadeIn {
    from { opacity: 0; transform: translateY(10px); }
    to { opacity: 1; transform: translateY(0); }
  }

  .speaker-control {
    position: relative;
  }

  .volume-slider-popup {
    position: absolute;
    bottom: 100%;
    left: 50%;
    transform: translateX(-50%);
    margin-bottom: 12px;
    padding: 12px 16px;
    background: rgba(30, 30, 40, 0.95);
    border-radius: 12px;
    box-shadow: 0 4px 20px rgba(0, 0, 0, 0.4);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    z-index: 100;
  }

  .volume-slider {
    width: 120px;
    height: 6px;
    -webkit-appearance: none;
    appearance: none;
    background: rgba(255, 255, 255, 0.2);
    border-radius: 3px;
    outline: none;
  }

  .volume-slider::-webkit-slider-thumb {
    -webkit-appearance: none;
    appearance: none;
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #4a9eff;
    cursor: pointer;
    border: 2px solid white;
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
  }

  .volume-slider::-moz-range-thumb {
    width: 18px;
    height: 18px;
    border-radius: 50%;
    background: #4a9eff;
    cursor: pointer;
    border: 2px solid white;
    box-shadow: 0 2px 6px rgba(0, 0, 0, 0.3);
  }

  .volume-label {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.8);
    font-weight: 500;
  }

  .text-input-container {
    display: flex;
    gap: 8px;
    padding: 12px 16px;
    background: rgba(0, 0, 0, 0.2);
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .text-input {
    flex: 1;
    padding: 10px 14px;
    border: 1px solid rgba(255, 255, 255, 0.2);
    border-radius: 20px;
    background: rgba(255, 255, 255, 0.1);
    color: white;
    font-size: 14px;
    outline: none;
    transition: all 0.2s ease;
  }

  .text-input::placeholder {
    color: rgba(255, 255, 255, 0.4);
  }

  .text-input:focus {
    border-color: rgba(74, 158, 255, 0.5);
    background: rgba(255, 255, 255, 0.15);
  }

  .text-input:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .send-btn {
    display: flex;
    align-items: center;
    justify-content: center;
    width: 40px;
    height: 40px;
    border: none;
    border-radius: 50%;
    background: #4a9eff;
    color: white;
    cursor: pointer;
    transition: all 0.2s ease;
  }

  .send-btn:hover:not(:disabled) {
    background: #3a8eef;
    transform: scale(1.05);
  }

  .send-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .send-btn .spinner {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>

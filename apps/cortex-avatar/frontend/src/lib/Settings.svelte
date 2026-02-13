<script lang="ts">
  import { onMount, onDestroy, createEventDispatcher } from 'svelte';

  const dispatch = createEventDispatcher();

  interface MediaDevice {
    deviceId: string;
    label: string;
    kind: string;
    groupId: string;
  }

  interface Persona {
    id: string;
    name: string;
    gender: string;
    voice_id: string;
  }

  interface Voice {
    id: string;
    name: string;
    provider: string;
    gender: string;
  }

  interface Brain {
    id: string;
    name: string;
    version: string;
    url: string;
    protocol: string;
    description: string;
    provider: string;
    model: string;
    status: string;
    latency: number;
    lastSeen: string;
    requiresAuth: boolean;
    skills: string[];
  }

  // Device lists
  let microphones: MediaDevice[] = [];
  let speakers: MediaDevice[] = [];
  let cameras: MediaDevice[] = [];
  let personas: Persona[] = [];
  let voices: Voice[] = [];
  let brains: Brain[] = [];

  // Selected devices
  let selectedMic = '';
  let selectedSpeaker = '';
  let selectedCamera = '';
  let selectedPersona = '';
  let selectedVoice = 'nova';  // Default to OpenAI Nova for natural voice
  let serverUrl = 'http://localhost:8080';
  let vadThreshold = 0.5;
  let useFrontierModel = false;

  // Brain selection
  let selectedBrainId = '';
  let customBrainUrl = '';
  let scanningBrains = false;
  let brainConnecting = false;

  // Test states
  let testingMic = false;
  let testingSpeaker = false;
  let testingCamera = false;
  let micLevel = 0;
  let cameraPreview: HTMLVideoElement;
  let cameraStream: MediaStream | null = null;

  // Loading states
  let loading = true;
  let saving = false;

  // Event listener cleanup
  let cleanupListeners: (() => void)[] = [];

  // Wails bindings
  declare const runtime: {
    EventsOn: (event: string, callback: (...args: any[]) => void) => () => void;
  };

  declare const go: {
    bridge: {
      SettingsBridge: {
        GetSettings: () => Promise<{
          microphoneId: string;
          speakerId: string;
          cameraId: string;
          persona: string;
          serverUrl: string;
          vadThreshold: number;
          useFrontierModel: boolean;
        }>;
        SaveSettings: (settings: any) => Promise<void>;
        SetUseFrontierModel: (use: boolean) => Promise<void>;
        GetPersonas: () => Promise<Persona[]>;
        GetVoices: () => Promise<Voice[]>;
        GetMicrophones: () => Promise<MediaDevice[]>;
        GetSpeakers: () => Promise<MediaDevice[]>;
        GetCameras: () => Promise<MediaDevice[]>;
        GetAllDevices: () => Promise<MediaDevice[]>;
        SetMicrophone: (id: string) => Promise<void>;
        SetSpeaker: (id: string) => Promise<void>;
        SetCamera: (id: string) => Promise<void>;
        SetPersona: (id: string) => Promise<void>;
        SetVoice: (id: string) => Promise<void>;
        SetServerURL: (url: string) => Promise<void>;
        SetVADThreshold: (threshold: number) => Promise<void>;
        TestVoice: (voiceId: string, text: string) => Promise<void>;
      };
      BrainBridge: {
        GetBrains: () => Promise<Brain[]>;
        ScanBrains: () => Promise<Brain[]>;
        GetSelectedBrain: () => Promise<Brain | null>;
        SelectBrain: (brainId: string) => Promise<void>;
        AddCustomBrainURL: (url: string) => Promise<void>;
        RemoveCustomBrainURL: (url: string) => Promise<void>;
        GetCurrentBrainInfo: () => Promise<{
          serverURL: string;
          connected: boolean;
          name?: string;
          version?: string;
          protocol?: string;
          description?: string;
        }>;
      };
    };
  };

  onMount(async () => {
    await enumerateDevices();
    await loadSettings();
    await loadBrains();
    setupBrainEvents();
    loading = false;
  });

  onDestroy(() => {
    // Clean up event listeners
    cleanupListeners.forEach(cleanup => cleanup());
    cleanupListeners = [];
  });

  function setupBrainEvents() {
    if (typeof runtime !== 'undefined') {
      // Listen for brain list updates
      const unsub1 = runtime.EventsOn('brain:list-updated', (brainList: Brain[]) => {
        brains = brainList || [];
      });
      cleanupListeners.push(unsub1);

      // Listen for brain selection
      const unsub2 = runtime.EventsOn('brain:selected', (brain: Brain) => {
        if (brain) {
          selectedBrainId = brain.id;
        }
      });
      cleanupListeners.push(unsub2);

      // Listen for connection status
      const unsub3 = runtime.EventsOn('brain:connecting', () => {
        brainConnecting = true;
      });
      cleanupListeners.push(unsub3);

      const unsub4 = runtime.EventsOn('brain:connected', () => {
        brainConnecting = false;
      });
      cleanupListeners.push(unsub4);

      const unsub5 = runtime.EventsOn('brain:connection-failed', () => {
        brainConnecting = false;
      });
      cleanupListeners.push(unsub5);
    }
  }

  async function loadBrains() {
    try {
      if (typeof go !== 'undefined') {
        brains = await go.bridge.BrainBridge.GetBrains() || [];
        const selected = await go.bridge.BrainBridge.GetSelectedBrain();
        if (selected) {
          selectedBrainId = selected.id;
        }
      }
    } catch (err) {
      console.error('Failed to load brains:', err);
    }
  }

  async function scanBrains() {
    scanningBrains = true;
    try {
      if (typeof go !== 'undefined') {
        brains = await go.bridge.BrainBridge.ScanBrains() || [];
      }
    } catch (err) {
      console.error('Failed to scan brains:', err);
    }
    scanningBrains = false;
  }

  async function selectBrain(brainId: string) {
    try {
      if (typeof go !== 'undefined') {
        await go.bridge.BrainBridge.SelectBrain(brainId);
        selectedBrainId = brainId;
        // Update serverUrl to match selected brain
        const brain = brains.find(b => b.id === brainId);
        if (brain) {
          serverUrl = brain.url;
        }
      }
    } catch (err) {
      console.error('Failed to select brain:', err);
    }
  }

  async function addCustomBrainUrl() {
    if (!customBrainUrl.trim()) return;
    try {
      if (typeof go !== 'undefined') {
        await go.bridge.BrainBridge.AddCustomBrainURL(customBrainUrl.trim());
        customBrainUrl = '';
        // Refresh the brain list
        await scanBrains();
      }
    } catch (err) {
      console.error('Failed to add custom brain URL:', err);
    }
  }

  async function enumerateDevices() {
    try {
      // Use Go-side native device enumeration
      if (typeof go !== 'undefined') {
        console.log('[Settings] Enumerating devices via Go...');

        // Get devices from Go backend using native macOS APIs
        const mics = await go.bridge.SettingsBridge.GetMicrophones();
        const spkrs = await go.bridge.SettingsBridge.GetSpeakers();
        const cams = await go.bridge.SettingsBridge.GetCameras();

        microphones = mics.map(d => ({
          deviceId: d.deviceId,
          label: d.label,
          kind: d.kind,
          groupId: ''
        }));

        speakers = spkrs.map(d => ({
          deviceId: d.deviceId,
          label: d.label,
          kind: d.kind,
          groupId: ''
        }));

        cameras = cams.map(d => ({
          deviceId: d.deviceId,
          label: d.label,
          kind: d.kind,
          groupId: ''
        }));

        console.log(`[Settings] Found ${microphones.length} mics, ${speakers.length} speakers, ${cameras.length} cameras`);
      } else {
        // Fallback to browser API for dev mode
        console.log('[Settings] Enumerating devices via browser API...');
        try {
          await navigator.mediaDevices.getUserMedia({ audio: true, video: true });
        } catch (e) {
          console.warn('[Settings] Permission denied, trying without permissions');
        }

        const devices = await navigator.mediaDevices.enumerateDevices();

        microphones = devices
          .filter(d => d.kind === 'audioinput')
          .map(d => ({
            deviceId: d.deviceId,
            label: d.label || `Microphone ${d.deviceId.slice(0, 8)}`,
            kind: d.kind,
            groupId: d.groupId
          }));

        speakers = devices
          .filter(d => d.kind === 'audiooutput')
          .map(d => ({
            deviceId: d.deviceId,
            label: d.label || `Speaker ${d.deviceId.slice(0, 8)}`,
            kind: d.kind,
            groupId: d.groupId
          }));

        cameras = devices
          .filter(d => d.kind === 'videoinput')
          .map(d => ({
            deviceId: d.deviceId,
            label: d.label || `Camera ${d.deviceId.slice(0, 8)}`,
            kind: d.kind,
            groupId: d.groupId
          }));

        console.log(`[Settings] Found ${microphones.length} mics, ${speakers.length} speakers, ${cameras.length} cameras`);
      }
    } catch (err) {
      console.error('Failed to enumerate devices:', err);
    }
  }

  async function loadSettings() {
    try {
      if (typeof go !== 'undefined') {
        const settings = await go.bridge.SettingsBridge.GetSettings();
        selectedMic = settings.microphoneId || '';
        selectedSpeaker = settings.speakerId || '';
        selectedCamera = settings.cameraId || '';
        selectedPersona = settings.persona || 'hannah';
        selectedVoice = (settings as any).voiceId || 'nova';
        serverUrl = settings.serverUrl || 'http://localhost:8080';
        vadThreshold = settings.vadThreshold || 0.5;
        useFrontierModel = settings.useFrontierModel || false;

        personas = await go.bridge.SettingsBridge.GetPersonas();
        voices = await go.bridge.SettingsBridge.GetVoices();
      } else {
        // Dev mode defaults
        personas = [
          { id: 'henry', name: 'Henry', gender: 'male', voice_id: 'am_adam' },
          { id: 'hannah', name: 'Hannah', gender: 'female', voice_id: 'af_bella' },
        ];
        voices = [
          { id: 'nova', name: 'Nova (Natural Female)', provider: 'openai', gender: 'female' },
          { id: 'shimmer', name: 'Shimmer (Clear Female)', provider: 'openai', gender: 'female' },
          { id: 'echo', name: 'Echo (Warm Male)', provider: 'openai', gender: 'male' },
          { id: 'onyx', name: 'Onyx (Deep Male)', provider: 'openai', gender: 'male' },
          { id: 'alloy', name: 'Alloy (Neutral)', provider: 'openai', gender: 'neutral' },
          { id: 'fable', name: 'Fable (British)', provider: 'openai', gender: 'neutral' },
        ];
        selectedPersona = 'hannah';
        selectedVoice = 'nova';
      }
    } catch (err) {
      console.error('Failed to load settings:', err);
    }
  }

  async function saveSettings() {
    saving = true;
    try {
      if (typeof go !== 'undefined') {
        await go.bridge.SettingsBridge.SaveSettings({
          microphoneId: selectedMic,
          speakerId: selectedSpeaker,
          cameraId: selectedCamera,
          persona: selectedPersona,
          voiceId: selectedVoice,
          serverUrl: serverUrl,
          vadThreshold: vadThreshold,
          useFrontierModel: useFrontierModel,
        });
      }
      dispatch('saved');
    } catch (err) {
      console.error('Failed to save settings:', err);
    }
    saving = false;
  }

  async function testMicrophone() {
    testingMic = true;
    try {
      const stream = await navigator.mediaDevices.getUserMedia({
        audio: { deviceId: selectedMic ? { exact: selectedMic } : undefined }
      });

      const audioContext = new AudioContext();
      const source = audioContext.createMediaStreamSource(stream);
      const analyser = audioContext.createAnalyser();
      analyser.fftSize = 256;
      source.connect(analyser);

      const dataArray = new Uint8Array(analyser.frequencyBinCount);

      const updateLevel = () => {
        if (!testingMic) {
          stream.getTracks().forEach(t => t.stop());
          audioContext.close();
          return;
        }
        analyser.getByteFrequencyData(dataArray);
        let sum = 0;
        for (let i = 0; i < dataArray.length; i++) {
          sum += dataArray[i];
        }
        micLevel = sum / dataArray.length / 255;
        requestAnimationFrame(updateLevel);
      };
      updateLevel();

      // Stop after 5 seconds
      setTimeout(() => {
        testingMic = false;
        micLevel = 0;
      }, 5000);
    } catch (err) {
      console.error('Microphone test failed:', err);
      testingMic = false;
    }
  }

  async function testSpeaker() {
    testingSpeaker = true;
    try {
      const audioContext = new AudioContext();
      const oscillator = audioContext.createOscillator();
      const gainNode = audioContext.createGain();

      oscillator.connect(gainNode);
      gainNode.connect(audioContext.destination);

      oscillator.frequency.value = 440; // A4 note
      gainNode.gain.value = 0.3;

      oscillator.start();
      setTimeout(() => {
        oscillator.stop();
        testingSpeaker = false;
      }, 500);
    } catch (err) {
      console.error('Speaker test failed:', err);
      testingSpeaker = false;
    }
  }

  async function testVoice() {
    try {
      if (typeof go !== 'undefined') {
        await go.bridge.SettingsBridge.TestVoice(selectedVoice, "Hello, I'm your AI assistant. How can I help you today?");
      } else {
        // Dev mode - use browser TTS
        const utterance = new SpeechSynthesisUtterance("Hello, I'm your AI assistant.");
        speechSynthesis.speak(utterance);
      }
    } catch (err) {
      console.error('Voice test failed:', err);
    }
  }

  async function testCamera() {
    if (testingCamera) {
      // Stop camera
      if (cameraStream) {
        cameraStream.getTracks().forEach(t => t.stop());
        cameraStream = null;
      }
      testingCamera = false;
      return;
    }

    testingCamera = true;
    try {
      cameraStream = await navigator.mediaDevices.getUserMedia({
        video: { deviceId: selectedCamera ? { exact: selectedCamera } : undefined }
      });

      if (cameraPreview) {
        cameraPreview.srcObject = cameraStream;
      }
    } catch (err) {
      console.error('Camera test failed:', err);
      testingCamera = false;
    }
  }

  function close() {
    // Stop any active tests
    testingMic = false;
    testingSpeaker = false;
    if (cameraStream) {
      cameraStream.getTracks().forEach(t => t.stop());
      cameraStream = null;
    }
    testingCamera = false;

    dispatch('close');
  }
</script>

<div class="settings-overlay" on:click|self={close}>
  <div class="settings-panel">
    <div class="settings-header">
      <h2>Settings</h2>
      <button class="close-btn" on:click={close}>
        <svg viewBox="0 0 24 24" width="20" height="20">
          <path fill="currentColor" d="M19,6.41L17.59,5L12,10.59L6.41,5L5,6.41L10.59,12L5,17.59L6.41,19L12,13.41L17.59,19L19,17.59L13.41,12L19,6.41Z"/>
        </svg>
      </button>
    </div>

    {#if loading}
      <div class="loading">Loading settings...</div>
    {:else}
      <div class="settings-content">
        <!-- Audio Section -->
        <section class="settings-section">
          <h3>Audio</h3>

          <div class="setting-row">
            <label for="microphone">Microphone</label>
            <div class="select-with-test">
              <select id="microphone" bind:value={selectedMic}>
                <option value="">Default</option>
                {#each microphones as mic}
                  <option value={mic.deviceId}>{mic.label}</option>
                {/each}
              </select>
              <button
                class="test-btn"
                class:active={testingMic}
                on:click={testMicrophone}
              >
                {testingMic ? 'Testing...' : 'Test'}
              </button>
            </div>
            {#if testingMic}
              <div class="level-meter">
                <div class="level-fill" style="width: {micLevel * 100}%"></div>
              </div>
            {/if}
          </div>

          <div class="setting-row">
            <label for="speaker">Speaker</label>
            <div class="select-with-test">
              <select id="speaker" bind:value={selectedSpeaker}>
                <option value="">Default</option>
                {#each speakers as speaker}
                  <option value={speaker.deviceId}>{speaker.label}</option>
                {/each}
              </select>
              <button
                class="test-btn"
                class:active={testingSpeaker}
                on:click={testSpeaker}
              >
                {testingSpeaker ? 'Playing...' : 'Test'}
              </button>
            </div>
          </div>

          <div class="setting-row">
            <label for="vad">Voice Detection Sensitivity</label>
            <div class="slider-row">
              <input
                type="range"
                id="vad"
                min="0.1"
                max="1.0"
                step="0.1"
                bind:value={vadThreshold}
              />
              <span class="slider-value">{vadThreshold.toFixed(1)}</span>
            </div>
          </div>
        </section>

        <!-- Video Section -->
        <section class="settings-section">
          <h3>Video</h3>

          <div class="setting-row">
            <label for="camera">Camera</label>
            <div class="select-with-test">
              <select id="camera" bind:value={selectedCamera}>
                <option value="">Default</option>
                {#each cameras as cam}
                  <option value={cam.deviceId}>{cam.label}</option>
                {/each}
              </select>
              <button
                class="test-btn"
                class:active={testingCamera}
                on:click={testCamera}
              >
                {testingCamera ? 'Stop' : 'Preview'}
              </button>
            </div>
          </div>

          {#if testingCamera}
            <div class="camera-preview">
              <video bind:this={cameraPreview} autoplay playsinline muted></video>
            </div>
          {/if}
        </section>

        <!-- Avatar Section -->
        <section class="settings-section">
          <h3>Avatar</h3>

          <div class="setting-row">
            <label>Persona</label>
            <div class="persona-grid">
              {#each personas as persona}
                <button
                  class="persona-card"
                  class:selected={selectedPersona === persona.id}
                  on:click={() => selectedPersona = persona.id}
                >
                  <span class="persona-icon">{persona.gender === 'male' ? '👨' : '👩'}</span>
                  <span class="persona-name">{persona.name}</span>
                </button>
              {/each}
            </div>
          </div>
        </section>

        <!-- Voice Section -->
        <section class="settings-section">
          <h3>Voice</h3>
          <p class="setting-hint">Choose a voice for Hannah to speak with. OpenAI voices are natural-sounding.</p>

          <div class="setting-row">
            <label for="voice">TTS Voice</label>
            <div class="select-with-test">
              <select id="voice" bind:value={selectedVoice}>
                {#each voices as voice}
                  <option value={voice.id}>
                    {voice.name} ({voice.provider})
                  </option>
                {/each}
              </select>
              <button
                class="test-btn"
                on:click={testVoice}
              >
                Test
              </button>
            </div>
          </div>
        </section>

        <!-- Brain Selection Section -->
        <section class="settings-section">
          <h3>Brain Selection</h3>
          <p class="setting-hint">Choose which CortexBrain instance to connect to.</p>

          <div class="brain-list">
            {#if brains.length === 0}
              <div class="no-brains">
                <span>No brains discovered</span>
                <button
                  class="scan-btn"
                  on:click={scanBrains}
                  disabled={scanningBrains}
                >
                  {scanningBrains ? 'Scanning...' : 'Scan Now'}
                </button>
              </div>
            {:else}
              {#each brains as brain}
                <button
                  class="brain-card"
                  class:selected={selectedBrainId === brain.id}
                  class:offline={brain.status !== 'online'}
                  class:connecting={brainConnecting && selectedBrainId === brain.id}
                  on:click={() => selectBrain(brain.id)}
                  disabled={brain.status !== 'online' || brainConnecting}
                >
                  <div class="brain-header">
                    <span class="brain-radio" class:checked={selectedBrainId === brain.id}></span>
                    <span class="brain-name">{brain.name || 'Unknown Brain'}</span>
                    {#if brain.requiresAuth}
                      <span class="brain-auth-badge" title="Requires authentication">Auth</span>
                    {/if}
                  </div>
                  <div class="brain-url">{brain.url}</div>
                  <div class="brain-details">
                    <span class="brain-model">{brain.provider}/{brain.model}</span>
                    <span class="brain-latency">{brain.latency}ms</span>
                    <span class="brain-status" class:online={brain.status === 'online'}>
                      {brain.status}
                    </span>
                  </div>
                </button>
              {/each}
            {/if}
          </div>

          <div class="brain-actions">
            <div class="custom-url-input">
              <input
                type="text"
                bind:value={customBrainUrl}
                placeholder="http://192.168.1.x:8080"
                on:keydown={(e) => e.key === 'Enter' && addCustomBrainUrl()}
              />
              <button
                class="add-btn"
                on:click={addCustomBrainUrl}
                disabled={!customBrainUrl.trim()}
              >
                + Add
              </button>
            </div>
            <button
              class="refresh-btn"
              on:click={scanBrains}
              disabled={scanningBrains}
              title="Refresh brain list"
            >
              <svg viewBox="0 0 24 24" width="16" height="16" class:spinning={scanningBrains}>
                <path fill="currentColor" d="M17.65,6.35C16.2,4.9 14.21,4 12,4A8,8 0 0,0 4,12A8,8 0 0,0 12,20C15.73,20 18.84,17.45 19.73,14H17.65C16.83,16.33 14.61,18 12,18A6,6 0 0,1 6,12A6,6 0 0,1 12,6C13.66,6 15.14,6.69 16.22,7.78L13,11H20V4L17.65,6.35Z"/>
              </svg>
            </button>
          </div>
        </section>

        <!-- Connection Section -->
        <section class="settings-section">
          <h3>Connection</h3>

          <div class="setting-row">
            <label for="server">Manual Server URL</label>
            <input
              type="text"
              id="server"
              bind:value={serverUrl}
              placeholder="http://localhost:8080"
            />
            <p class="setting-hint">
              Override auto-discovery with a specific server URL.
            </p>
          </div>

          <div class="setting-row">
            <label class="toggle-label">
              <span>Use Frontier Model (Cloud AI)</span>
              <div class="toggle-switch" class:active={useFrontierModel}>
                <input
                  type="checkbox"
                  bind:checked={useFrontierModel}
                />
                <span class="toggle-slider"></span>
              </div>
            </label>
            <p class="setting-hint">
              Use Claude/GPT-4 instead of local Ollama. Requires API key configured in CortexBrain.
            </p>
          </div>
        </section>
      </div>

      <div class="settings-footer">
        <button class="cancel-btn" on:click={close}>Cancel</button>
        <button class="save-btn" on:click={saveSettings} disabled={saving}>
          {saving ? 'Saving...' : 'Save Settings'}
        </button>
      </div>
    {/if}
  </div>
</div>

<style>
  .settings-overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background: rgba(0, 0, 0, 0.7);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 1000;
  }

  .settings-panel {
    background: #1a1a2e;
    border-radius: 16px;
    width: 90%;
    max-width: 480px;
    max-height: 85vh;
    display: flex;
    flex-direction: column;
    box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
  }

  .settings-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 16px 20px;
    border-bottom: 1px solid rgba(255, 255, 255, 0.1);
  }

  .settings-header h2 {
    margin: 0;
    font-size: 18px;
    font-weight: 600;
    color: white;
  }

  .close-btn {
    background: none;
    border: none;
    color: rgba(255, 255, 255, 0.5);
    cursor: pointer;
    padding: 4px;
    border-radius: 4px;
  }

  .close-btn:hover {
    color: white;
    background: rgba(255, 255, 255, 0.1);
  }

  .loading {
    padding: 40px;
    text-align: center;
    color: rgba(255, 255, 255, 0.5);
  }

  .settings-content {
    flex: 1;
    overflow-y: auto;
    padding: 16px 20px;
  }

  .settings-section {
    margin-bottom: 24px;
  }

  .settings-section h3 {
    margin: 0 0 12px 0;
    font-size: 13px;
    font-weight: 600;
    color: rgba(255, 255, 255, 0.4);
    text-transform: uppercase;
    letter-spacing: 0.5px;
  }

  .setting-row {
    margin-bottom: 16px;
  }

  .setting-row label {
    display: block;
    margin-bottom: 6px;
    font-size: 14px;
    color: rgba(255, 255, 255, 0.8);
  }

  .select-with-test {
    display: flex;
    gap: 8px;
  }

  select, input[type="text"] {
    flex: 1;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 8px;
    padding: 10px 12px;
    color: white;
    font-size: 14px;
    outline: none;
  }

  select:focus, input[type="text"]:focus {
    border-color: #4a9eff;
  }

  .test-btn {
    padding: 10px 16px;
    background: rgba(255, 255, 255, 0.1);
    border: none;
    border-radius: 8px;
    color: white;
    font-size: 13px;
    cursor: pointer;
    transition: background 0.2s;
    white-space: nowrap;
  }

  .test-btn:hover {
    background: rgba(255, 255, 255, 0.2);
  }

  .test-btn.active {
    background: #4a9eff;
  }

  .level-meter {
    height: 6px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 3px;
    margin-top: 8px;
    overflow: hidden;
  }

  .level-fill {
    height: 100%;
    background: linear-gradient(90deg, #4cd964, #4cd964 70%, #ffcc00 85%, #ff3b30);
    border-radius: 3px;
    transition: width 0.05s;
  }

  .slider-row {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  input[type="range"] {
    flex: 1;
    -webkit-appearance: none;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 4px;
    height: 6px;
  }

  input[type="range"]::-webkit-slider-thumb {
    -webkit-appearance: none;
    width: 18px;
    height: 18px;
    background: #4a9eff;
    border-radius: 50%;
    cursor: pointer;
  }

  .slider-value {
    min-width: 32px;
    text-align: center;
    font-size: 13px;
    color: rgba(255, 255, 255, 0.6);
  }

  .camera-preview {
    margin-top: 12px;
    border-radius: 8px;
    overflow: hidden;
    background: black;
  }

  .camera-preview video {
    width: 100%;
    display: block;
  }

  .persona-grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 12px;
  }

  .persona-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    padding: 16px;
    background: rgba(255, 255, 255, 0.05);
    border: 2px solid transparent;
    border-radius: 12px;
    cursor: pointer;
    transition: all 0.2s;
  }

  .persona-card:hover {
    background: rgba(255, 255, 255, 0.1);
  }

  .persona-card.selected {
    border-color: #4a9eff;
    background: rgba(74, 158, 255, 0.15);
  }

  .persona-icon {
    font-size: 32px;
  }

  .persona-name {
    font-size: 14px;
    font-weight: 500;
    color: white;
  }

  .settings-footer {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding: 16px 20px;
    border-top: 1px solid rgba(255, 255, 255, 0.1);
  }

  .cancel-btn, .save-btn {
    padding: 10px 20px;
    border-radius: 8px;
    font-size: 14px;
    font-weight: 500;
    cursor: pointer;
    transition: background 0.2s;
  }

  .cancel-btn {
    background: rgba(255, 255, 255, 0.1);
    border: none;
    color: white;
  }

  .cancel-btn:hover {
    background: rgba(255, 255, 255, 0.2);
  }

  .save-btn {
    background: #4a9eff;
    border: none;
    color: white;
  }

  .save-btn:hover:not(:disabled) {
    background: #3a8eef;
  }

  .save-btn:disabled {
    opacity: 0.5;
    cursor: not-allowed;
  }

  .toggle-label {
    display: flex;
    justify-content: space-between;
    align-items: center;
    cursor: pointer;
  }

  .toggle-switch {
    position: relative;
    width: 48px;
    height: 26px;
    background: rgba(255, 255, 255, 0.1);
    border-radius: 13px;
    transition: background 0.2s;
  }

  .toggle-switch.active {
    background: #4a9eff;
  }

  .toggle-switch input {
    opacity: 0;
    width: 0;
    height: 0;
  }

  .toggle-slider {
    position: absolute;
    top: 3px;
    left: 3px;
    width: 20px;
    height: 20px;
    background: white;
    border-radius: 50%;
    transition: transform 0.2s;
  }

  .toggle-switch.active .toggle-slider {
    transform: translateX(22px);
  }

  .setting-hint {
    margin: 4px 0 0 0;
    font-size: 12px;
    color: rgba(255, 255, 255, 0.4);
    line-height: 1.4;
  }

  /* Brain Selection Styles */
  .brain-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
    margin-bottom: 12px;
  }

  .no-brains {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 12px;
    padding: 24px;
    background: rgba(255, 255, 255, 0.03);
    border-radius: 8px;
    color: rgba(255, 255, 255, 0.5);
  }

  .scan-btn {
    padding: 8px 16px;
    background: #4a9eff;
    border: none;
    border-radius: 6px;
    color: white;
    font-size: 13px;
    cursor: pointer;
    transition: background 0.2s;
  }

  .scan-btn:hover:not(:disabled) {
    background: #3a8eef;
  }

  .scan-btn:disabled {
    opacity: 0.6;
    cursor: not-allowed;
  }

  .brain-card {
    display: flex;
    flex-direction: column;
    gap: 4px;
    padding: 12px;
    background: rgba(255, 255, 255, 0.05);
    border: 2px solid transparent;
    border-radius: 10px;
    cursor: pointer;
    transition: all 0.2s;
    text-align: left;
    width: 100%;
  }

  .brain-card:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.08);
  }

  .brain-card.selected {
    border-color: #4a9eff;
    background: rgba(74, 158, 255, 0.1);
  }

  .brain-card.offline {
    opacity: 0.5;
  }

  .brain-card.connecting {
    border-color: #ffcc00;
    animation: pulse 1.5s infinite;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; }
    50% { opacity: 0.7; }
  }

  .brain-card:disabled {
    cursor: not-allowed;
  }

  .brain-header {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .brain-radio {
    width: 16px;
    height: 16px;
    border: 2px solid rgba(255, 255, 255, 0.3);
    border-radius: 50%;
    flex-shrink: 0;
    transition: all 0.2s;
  }

  .brain-radio.checked {
    border-color: #4a9eff;
    background: #4a9eff;
    box-shadow: inset 0 0 0 3px #1a1a2e;
  }

  .brain-name {
    font-size: 14px;
    font-weight: 600;
    color: white;
    flex: 1;
  }

  .brain-auth-badge {
    padding: 2px 6px;
    background: rgba(255, 200, 0, 0.2);
    border: 1px solid rgba(255, 200, 0, 0.4);
    border-radius: 4px;
    font-size: 10px;
    color: #ffcc00;
    text-transform: uppercase;
  }

  .brain-url {
    font-size: 12px;
    color: rgba(255, 255, 255, 0.4);
    font-family: monospace;
    margin-left: 24px;
  }

  .brain-details {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-left: 24px;
    font-size: 11px;
  }

  .brain-model {
    color: rgba(255, 255, 255, 0.5);
  }

  .brain-latency {
    color: rgba(255, 255, 255, 0.4);
  }

  .brain-status {
    padding: 2px 6px;
    background: rgba(255, 59, 48, 0.2);
    border-radius: 4px;
    color: #ff3b30;
  }

  .brain-status.online {
    background: rgba(76, 217, 100, 0.2);
    color: #4cd964;
  }

  .brain-actions {
    display: flex;
    gap: 8px;
    align-items: center;
  }

  .custom-url-input {
    display: flex;
    flex: 1;
    gap: 8px;
  }

  .custom-url-input input {
    flex: 1;
    background: rgba(255, 255, 255, 0.05);
    border: 1px solid rgba(255, 255, 255, 0.1);
    border-radius: 6px;
    padding: 8px 12px;
    color: white;
    font-size: 13px;
    outline: none;
  }

  .custom-url-input input:focus {
    border-color: #4a9eff;
  }

  .custom-url-input input::placeholder {
    color: rgba(255, 255, 255, 0.3);
  }

  .add-btn {
    padding: 8px 12px;
    background: rgba(255, 255, 255, 0.1);
    border: none;
    border-radius: 6px;
    color: white;
    font-size: 13px;
    cursor: pointer;
    transition: background 0.2s;
    white-space: nowrap;
  }

  .add-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.2);
  }

  .add-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .refresh-btn {
    width: 36px;
    height: 36px;
    display: flex;
    align-items: center;
    justify-content: center;
    background: rgba(255, 255, 255, 0.1);
    border: none;
    border-radius: 6px;
    color: rgba(255, 255, 255, 0.7);
    cursor: pointer;
    transition: all 0.2s;
  }

  .refresh-btn:hover:not(:disabled) {
    background: rgba(255, 255, 255, 0.2);
    color: white;
  }

  .refresh-btn:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }

  .refresh-btn svg.spinning {
    animation: spin 1s linear infinite;
  }

  @keyframes spin {
    from { transform: rotate(0deg); }
    to { transform: rotate(360deg); }
  }
</style>

import { writable, get } from 'svelte/store';

declare global {
  interface Window {
    runtime?: {
      EventsOn: (event: string, callback: (...args: any[]) => void) => void;
      EventsEmit: (event: string, ...args: any[]) => void;
    };
    go?: any;
  }
}

// Viseme event for lip-sync animation
export interface VisemeEvent {
  time: number;           // milliseconds from audio start
  visemeId: number;       // Oculus viseme ID (0-14)
  weight: number;         // 0-1 intensity
  duration: number;       // milliseconds
}

export interface AudioState {
  isListening: boolean;
  isSpeaking: boolean;
  isThinking: boolean;  // Cognitive processing in progress
  micEnabled: boolean;
  speakerEnabled: boolean;
  cameraEnabled: boolean;
  screenShareEnabled: boolean;
  volume: number;
  outputVolume: number;  // Speaker output volume 0-100
  vadActive: boolean;
  transcript: string;
  lastResponse: string;
  // Viseme data for lip-sync
  visemeTimeline: VisemeEvent[];
  currentAudioElement: HTMLAudioElement | null;
}

export const audioState = writable<AudioState>({
  isListening: false,
  isSpeaking: false,
  isThinking: false,
  micEnabled: false,
  speakerEnabled: true,
  cameraEnabled: false,
  screenShareEnabled: false,
  volume: 0,
  outputVolume: 100,
  vadActive: false,
  transcript: '',
  lastResponse: '',
  visemeTimeline: [],
  currentAudioElement: null,
});

// Browser Speech Recognition for STT
// @ts-ignore - SpeechRecognition may not be in TS types
const SpeechRecognition = window.SpeechRecognition || window.webkitSpeechRecognition;

// Check if Go-side STT is available
async function checkGoSTTAvailable(): Promise<{available: boolean, message: string}> {
  try {
    // @ts-ignore - Wails binding
    if (window.go?.bridge?.AudioBridge?.GetSTTStatus) {
      const status = await window.go.bridge.AudioBridge.GetSTTStatus();
      return { available: status.available, message: status.message };
    }
  } catch (e) {
    console.error('[STT] Failed to check Go STT status:', e);
  }
  return { available: false, message: 'Go STT not available' };
}

class SpeechRecognitionManager {
  private recognition: any = null;
  private isListening = false;
  private interimTranscript = '';
  private useGoSTT = false;
  private audioContext: AudioContext | null = null;
  private mediaStream: MediaStream | null = null;
  private processor: ScriptProcessorNode | null = null;
  private audioBuffer: Int16Array[] = [];

  constructor() {
    // Check if Web Speech API is available
    if (SpeechRecognition) {
      this.initWebSpeechAPI();
    } else {
      console.warn('[STT] Web Speech API not supported, will use Go-side STT');
      this.useGoSTT = true;
    }
  }

  private initWebSpeechAPI() {
    this.recognition = new SpeechRecognition();
    this.recognition.continuous = true;
    this.recognition.interimResults = true;
    this.recognition.lang = 'en-US';

    this.recognition.onstart = () => {
      console.log('[STT] Recognition started');
      this.isListening = true;
      audioState.update(s => ({ ...s, isListening: true, vadActive: true }));
    };

    this.recognition.onend = () => {
      console.log('[STT] Recognition ended');
      this.isListening = false;
      audioState.update(s => ({ ...s, isListening: false, vadActive: false }));

      // Auto-restart if mic is still enabled
      const state = get(audioState);
      if (state.micEnabled) {
        console.log('[STT] Auto-restarting recognition');
        setTimeout(() => this.start(), 100);
      }
    };

    this.recognition.onerror = (event: any) => {
      console.error('[STT] Recognition error:', event.error);
      if (event.error === 'no-speech') {
        return;
      }
      // If Web Speech API fails, try Go-side STT
      if (event.error === 'not-allowed' || event.error === 'service-not-allowed') {
        console.log('[STT] Web Speech API not allowed, switching to Go-side STT');
        this.useGoSTT = true;
      }
      audioState.update(s => ({ ...s, vadActive: false }));
    };

    this.recognition.onresult = async (event: any) => {
      let finalTranscript = '';
      this.interimTranscript = '';

      for (let i = event.resultIndex; i < event.results.length; i++) {
        const transcript = event.results[i][0].transcript;
        if (event.results[i].isFinal) {
          finalTranscript += transcript;
        } else {
          this.interimTranscript += transcript;
        }
      }

      if (this.interimTranscript) {
        audioState.update(s => ({ ...s, transcript: this.interimTranscript, vadActive: true }));
      }

      if (finalTranscript.trim()) {
        console.log('[STT] Final transcript:', finalTranscript);
        audioState.update(s => ({ ...s, transcript: finalTranscript, vadActive: false }));

        try {
          const response = await sendTextMessage(finalTranscript.trim());
          console.log('[STT] Got response:', response?.substring(0, 100));
        } catch (err) {
          console.error('[STT] Failed to send message:', err);
        }
      }
    };
  }

  async start(): Promise<void> {
    console.log('[STT] start() called, isListening:', this.isListening);
    if (this.isListening) {
      console.log('[STT] Already listening, returning');
      return;
    }

    // PREFER Go-side STT (Groq Whisper) for reliability in Wails WebView
    console.log('[STT] Checking Go STT availability...');
    const sttStatus = await checkGoSTTAvailable();
    console.log('[STT] Go STT status:', sttStatus);

    if (sttStatus.available) {
      console.log('[STT] Using Go-side STT (Groq Whisper) - preferred for reliability');
      this.useGoSTT = true;
      await this.startGoSTT();
      return;
    }

    // Fall back to Web Speech API if Go STT not available
    if (this.recognition && !this.useGoSTT) {
      try {
        console.log('[STT] Go STT not available, trying Web Speech API');
        this.recognition.start();
        return;
      } catch (err) {
        console.error('[STT] Web Speech API failed:', err);
      }
    }

    console.error('[STT] No STT method available. Please set GROQ_API_KEY for voice input.');
    audioState.update(s => ({ ...s, transcript: 'STT not available. Set GROQ_API_KEY or type your message.' }));
  }

  private async startGoSTT(): Promise<void> {
    console.log('[STT] ========================================');
    console.log('[STT] Step 1: Starting Go-side STT (audio capture)');
    console.log('[STT] ========================================');
    audioState.update(s => ({ ...s, transcript: 'Initializing microphone...' }));

    // Check STT status from Go backend
    console.log('[STT] Step 2: Checking Go STT availability...');
    const sttStatus = await checkGoSTTAvailable();
    console.log('[STT] Step 2 Result: Go STT status:', JSON.stringify(sttStatus));
    if (!sttStatus.available) {
      console.error('[STT] FAILED at Step 2: Go STT not available:', sttStatus.message);
      audioState.update(s => ({ ...s, transcript: 'STT Error: ' + sttStatus.message }));
      return;
    }

    try {
      console.log('[STT] Step 3: Checking navigator.mediaDevices...');
      audioState.update(s => ({ ...s, transcript: 'Requesting microphone access...' }));

      // Check if getUserMedia is available
      if (!navigator.mediaDevices) {
        console.error('[STT] FAILED at Step 3: navigator.mediaDevices is', navigator.mediaDevices);
        audioState.update(s => ({ ...s, transcript: 'Mic API not available', micEnabled: false }));
        return;
      }
      if (!navigator.mediaDevices.getUserMedia) {
        console.error('[STT] FAILED at Step 3: getUserMedia not available in this WebView');
        audioState.update(s => ({ ...s, transcript: 'Mic not available in WebView', micEnabled: false }));
        return;
      }
      console.log('[STT] Step 3 OK: mediaDevices.getUserMedia is available');

      // List available devices for debugging
      console.log('[STT] Step 4: Enumerating audio devices...');
      try {
        const devices = await navigator.mediaDevices.enumerateDevices();
        const audioInputs = devices.filter(d => d.kind === 'audioinput');
        console.log('[STT] Step 4 Result: Found', audioInputs.length, 'audio inputs:');
        audioInputs.forEach((d, i) => console.log(`[STT]   ${i}: "${d.label || 'unnamed'}" (${d.deviceId?.substring(0,8)}...)`));
        if (audioInputs.length === 0) {
          console.warn('[STT] WARNING: No audio input devices found!');
        }
      } catch (enumErr) {
        console.warn('[STT] Step 4 Warning: Could not enumerate devices:', enumErr);
      }

      console.log('[STT] Step 5: Calling getUserMedia...');
      this.mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        }
      });

      const tracks = this.mediaStream.getTracks();
      console.log('[STT] Step 5 OK: Microphone permission granted!');
      console.log('[STT]   MediaStream tracks:', tracks.length);
      tracks.forEach((t, i) => console.log(`[STT]   Track ${i}: ${t.kind}, enabled=${t.enabled}, muted=${t.muted}, readyState=${t.readyState}`));
      audioState.update(s => ({ ...s, transcript: 'Microphone active!' }));

      console.log('[STT] Step 6: Creating AudioContext (16kHz)...');
      this.audioContext = new AudioContext({ sampleRate: 16000 });
      console.log('[STT] Step 6 Result: AudioContext state:', this.audioContext.state, 'sampleRate:', this.audioContext.sampleRate);

      // Resume AudioContext if suspended (required by some browsers)
      if (this.audioContext.state === 'suspended') {
        console.log('[STT] AudioContext suspended, resuming...');
        await this.audioContext.resume();
        console.log('[STT] AudioContext resumed, new state:', this.audioContext.state);
      }

      console.log('[STT] Step 7: Creating MediaStreamSource...');
      const source = this.audioContext.createMediaStreamSource(this.mediaStream);
      console.log('[STT] Step 7 OK: MediaStreamSource created');

      // Use ScriptProcessor to capture audio chunks
      console.log('[STT] Step 8: Creating ScriptProcessor (4096 samples)...');
      this.processor = this.audioContext.createScriptProcessor(4096, 1, 1);
      this.audioBuffer = [];
      console.log('[STT] Step 8 OK: ScriptProcessor created');

      let speechStartTime = 0;
      let silenceStartTime = 0;
      let chunkCount = 0;
      let lastLogTime = 0;
      const VAD_THRESHOLD = 0.01;
      const SILENCE_DURATION = 1500; // 1.5 seconds of silence to end

      console.log('[STT] Step 9: Setting up onaudioprocess callback...');
      console.log('[STT]   VAD_THRESHOLD:', VAD_THRESHOLD);
      console.log('[STT]   SILENCE_DURATION:', SILENCE_DURATION, 'ms');

      this.processor.onaudioprocess = (e) => {
        chunkCount++;
        const inputData = e.inputBuffer.getChannelData(0);

        // Calculate RMS for VAD
        let sum = 0;
        for (let i = 0; i < inputData.length; i++) {
          sum += inputData[i] * inputData[i];
        }
        const rms = Math.sqrt(sum / inputData.length);

        // Update volume level for visual feedback
        audioState.update(s => ({ ...s, volume: rms * 10 })); // Scale for visibility

        // Log every 25 chunks (~6.4 seconds at 4096 samples @ 16kHz) or when speech
        const now = Date.now();
        const shouldLog = (now - lastLogTime > 2000) || (rms > VAD_THRESHOLD && speechStartTime === 0);
        if (shouldLog) {
          console.log(`[STT] Audio chunk #${chunkCount}: RMS=${rms.toFixed(4)}, isSpeech=${rms > VAD_THRESHOLD}, bufferSize=${this.audioBuffer.length}`);
          lastLogTime = now;
        }

        // Convert to Int16
        const int16Data = new Int16Array(inputData.length);
        for (let i = 0; i < inputData.length; i++) {
          int16Data[i] = Math.max(-32768, Math.min(32767, inputData[i] * 32768));
        }

        if (rms > VAD_THRESHOLD) {
          // Speech detected
          if (speechStartTime === 0) {
            speechStartTime = now;
            console.log('[STT] >>> SPEECH STARTED <<< RMS:', rms.toFixed(4));
          }
          silenceStartTime = 0;
          this.audioBuffer.push(int16Data);
          audioState.update(s => ({ ...s, vadActive: true }));
        } else if (speechStartTime > 0) {
          // Silence during speech
          if (silenceStartTime === 0) {
            silenceStartTime = now;
            console.log('[STT] Silence detected during speech, waiting', SILENCE_DURATION, 'ms...');
          }
          this.audioBuffer.push(int16Data); // Still capture during short silences

          // End of speech after silence duration
          if (now - silenceStartTime > SILENCE_DURATION) {
            console.log('[STT] >>> SPEECH ENDED <<< duration:', now - speechStartTime, 'ms, buffer chunks:', this.audioBuffer.length);
            audioState.update(s => ({ ...s, vadActive: false }));
            this.transcribeBuffer();
            speechStartTime = 0;
            silenceStartTime = 0;
          }
        }
      };

      console.log('[STT] Step 10: Connecting audio nodes...');
      source.connect(this.processor);
      this.processor.connect(this.audioContext.destination);
      console.log('[STT] Step 10 OK: Audio pipeline connected');

      this.isListening = true;
      audioState.update(s => ({ ...s, isListening: true }));
      console.log('[STT] ========================================');
      console.log('[STT] SUCCESS: Go-side STT started!');
      console.log('[STT] Speak now - watching for RMS >', VAD_THRESHOLD);
      console.log('[STT] ========================================');

    } catch (err: any) {
      console.error('[STT] Failed to start Go-side STT:', err);
      let errorMsg = 'Microphone error';
      if (err.name === 'NotAllowedError' || err.name === 'PermissionDeniedError') {
        errorMsg = 'Mic permission denied - check System Settings > Privacy > Microphone';
      } else if (err.name === 'NotFoundError' || err.name === 'DevicesNotFoundError') {
        errorMsg = 'No microphone found';
      } else if (err.name === 'NotReadableError' || err.name === 'TrackStartError') {
        errorMsg = 'Mic busy or unavailable';
      } else if (err.message) {
        errorMsg = `Mic error: ${err.message}`;
      }
      audioState.update(s => ({ ...s, isListening: false, micEnabled: false, transcript: errorMsg }));
    }
  }

  private async transcribeBuffer(): Promise<void> {
    console.log('[STT] transcribeBuffer() called, buffer chunks:', this.audioBuffer.length);

    if (this.audioBuffer.length === 0) {
      console.log('[STT] Buffer empty, skipping transcription');
      return;
    }

    // Combine all audio chunks
    const totalLength = this.audioBuffer.reduce((acc, arr) => acc + arr.length, 0);
    console.log('[STT] Combining', this.audioBuffer.length, 'chunks into', totalLength, 'samples');

    const combined = new Int16Array(totalLength);
    let offset = 0;
    for (const chunk of this.audioBuffer) {
      combined.set(chunk, offset);
      offset += chunk.length;
    }
    this.audioBuffer = [];

    // Calculate duration for logging
    const durationMs = (totalLength / 16000) * 1000;
    console.log('[STT] Audio duration:', durationMs.toFixed(0), 'ms');

    if (durationMs < 500) {
      console.log('[STT] Audio too short (<500ms), skipping');
      return;
    }

    // Convert to base64
    const bytes = new Uint8Array(combined.buffer);
    let binary = '';
    for (let i = 0; i < bytes.length; i++) {
      binary += String.fromCharCode(bytes[i]);
    }
    const audioBase64 = btoa(binary);

    console.log('[STT] Encoded to base64:', audioBase64.length, 'chars');
    console.log('[STT] Calling Go TranscribeAudio...');
    audioState.update(s => ({ ...s, transcript: 'Transcribing...' }));

    try {
      // @ts-ignore - Wails binding
      if (window.go?.bridge?.AudioBridge?.TranscribeAudio) {
        console.log('[STT] Go binding available, calling...');
        const transcript = await window.go.bridge.AudioBridge.TranscribeAudio(audioBase64);
        console.log('[STT] Go returned transcript:', transcript || '(empty)');

        if (transcript && transcript.trim()) {
          console.log('[STT] Valid transcript received:', transcript);
          audioState.update(s => ({ ...s, transcript }));

          // Send to CortexBrain
          console.log('[STT] Sending to CortexBrain...');
          const response = await sendTextMessage(transcript.trim());
          console.log('[STT] CortexBrain response:', response?.substring(0, 100) || '(empty)');
        } else {
          console.log('[STT] Empty transcript from Go, no speech detected');
          audioState.update(s => ({ ...s, transcript: 'No speech detected' }));
        }
      } else {
        console.error('[STT] ERROR: Go binding not available!');
        console.log('[STT] window.go:', typeof window.go);
        console.log('[STT] window.go?.bridge:', typeof (window as any).go?.bridge);
        audioState.update(s => ({ ...s, transcript: 'Go binding unavailable' }));
      }
    } catch (err: any) {
      console.error('[STT] Transcription failed:', err);
      console.error('[STT] Error name:', err?.name);
      console.error('[STT] Error message:', err?.message);
      audioState.update(s => ({ ...s, transcript: 'Transcription error: ' + (err?.message || 'Unknown') }));
    }
  }

  stop(): void {
    if (this.recognition && !this.useGoSTT) {
      try {
        this.recognition.stop();
      } catch (err) {
        console.error('[STT] Failed to stop recognition:', err);
      }
    }

    // Stop Go-side STT
    if (this.processor) {
      this.processor.disconnect();
      this.processor = null;
    }
    if (this.audioContext) {
      this.audioContext.close();
      this.audioContext = null;
    }
    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach(t => t.stop());
      this.mediaStream = null;
    }

    this.isListening = false;
    this.audioBuffer = [];
    audioState.update(s => ({ ...s, isListening: false, vadActive: false }));
  }

  isAvailable(): boolean {
    return !!SpeechRecognition || this.useGoSTT;
  }
}

// Global speech recognition instance
export const speechRecognitionManager = new SpeechRecognitionManager();

// Audio capture class (for volume visualization only, not STT)
class AudioCaptureManager {
  private audioContext: AudioContext | null = null;
  private mediaStream: MediaStream | null = null;
  private analyser: AnalyserNode | null = null;
  private processor: ScriptProcessorNode | null = null;
  private isCapturing = false;
  private logCounter = 0;

  async startCapture(): Promise<boolean> {
    if (this.isCapturing) return true;

    try {
      this.mediaStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          autoGainControl: true,
        }
      });

      this.audioContext = new AudioContext();
      const source = this.audioContext.createMediaStreamSource(this.mediaStream);

      // Create analyser for volume level visualization
      this.analyser = this.audioContext.createAnalyser();
      this.analyser.fftSize = 256;
      source.connect(this.analyser);

      this.isCapturing = true;
      audioState.update(s => ({ ...s, isListening: true }));
      console.log('[Audio] Capture started (volume only)');

      // Start volume monitoring
      this.monitorVolume();

      return true;
    } catch (err) {
      console.error('[Audio] Failed to start capture:', err);
      return false;
    }
  }

  private monitorVolume(): void {
    if (!this.isCapturing || !this.analyser) return;

    const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
    this.analyser.getByteFrequencyData(dataArray);

    let sum = 0;
    for (let i = 0; i < dataArray.length; i++) {
      sum += dataArray[i];
    }
    const volume = sum / dataArray.length / 255;
    audioState.update(s => ({ ...s, volume }));

    requestAnimationFrame(() => this.monitorVolume());
  }

  stopCapture(): void {
    if (!this.isCapturing) return;

    if (this.analyser) {
      this.analyser.disconnect();
      this.analyser = null;
    }
    if (this.audioContext) {
      this.audioContext.close();
      this.audioContext = null;
    }
    if (this.mediaStream) {
      this.mediaStream.getTracks().forEach(track => track.stop());
      this.mediaStream = null;
    }

    this.isCapturing = false;
    audioState.update(s => ({ ...s, isListening: false, vadActive: false }));
    console.log('[Audio] Capture stopped');
  }

  getVolumeLevel(): number {
    if (!this.analyser) return 0;

    const dataArray = new Uint8Array(this.analyser.frequencyBinCount);
    this.analyser.getByteFrequencyData(dataArray);

    let sum = 0;
    for (let i = 0; i < dataArray.length; i++) {
      sum += dataArray[i];
    }
    return sum / dataArray.length / 255;
  }
}

// Audio playback class with browser TTS fallback
class AudioPlaybackManager {
  private audioContext: AudioContext | null = null;
  private currentSource: AudioBufferSourceNode | null = null;
  private currentUtterance: SpeechSynthesisUtterance | null = null;

  async playAudio(audioBase64: string, format: string): Promise<void> {
    console.log('[Audio] playAudio() called, format:', format, 'audioLen:', audioBase64?.length);
    try {
      audioState.update(s => ({ ...s, isSpeaking: true }));
      console.log('[Audio] Set isSpeaking=true for lip sync');

      if (!this.audioContext) {
        this.audioContext = new AudioContext();
      }

      // Decode base64 to array buffer
      const binaryString = atob(audioBase64);
      const bytes = new Uint8Array(binaryString.length);
      for (let i = 0; i < binaryString.length; i++) {
        bytes[i] = binaryString.charCodeAt(i);
      }

      // Decode audio
      const audioBuffer = await this.audioContext.decodeAudioData(bytes.buffer);

      // Play
      this.currentSource = this.audioContext.createBufferSource();
      this.currentSource.buffer = audioBuffer;
      this.currentSource.connect(this.audioContext.destination);

      this.currentSource.onended = () => {
        audioState.update(s => ({ ...s, isSpeaking: false }));
      };

      this.currentSource.start();
      console.log('[Audio] Playing TTS audio');
    } catch (err) {
      console.error('[Audio] Playback failed:', err);
      audioState.update(s => ({ ...s, isSpeaking: false }));
    }
  }

  private cachedVoices: SpeechSynthesisVoice[] = [];

  speakText(text: string, voiceId?: string): void {
    if (!('speechSynthesis' in window)) {
      console.error('[Audio] Browser TTS not supported');
      return;
    }

    speechSynthesis.cancel();

    const doSpeak = () => {
      console.log('[Audio] speakText() starting, voiceId:', voiceId);
      audioState.update(s => ({ ...s, isSpeaking: true }));

      this.currentUtterance = new SpeechSynthesisUtterance(text);

      const voices = this.cachedVoices.length > 0 ? this.cachedVoices : speechSynthesis.getVoices();
      console.log('[Audio] Available voices count:', voices.length);

      const voiceMap: Record<string, string[]> = {
        'nova': ['Samantha', 'Serena', 'Fiona', 'Karen', 'Victoria'],
        'shimmer': ['Samantha', 'Zoe', 'Serena', 'Victoria', 'Karen'],
        'onyx': ['Daniel', 'Alex', 'Tom', 'Oliver', 'Fred'],
        'echo': ['Daniel', 'Tom', 'Alex', 'Oliver'],
        'alloy': ['Samantha', 'Daniel', 'Karen', 'Alex'],
        'fable': ['Daniel', 'Serena', 'Oliver', 'Fiona'],
      };

      let selectedVoice: SpeechSynthesisVoice | null = null;

      if (voiceId) {
        const directMatch = voices.find(v => v.name === voiceId || v.name.includes(voiceId));
        if (directMatch) {
          selectedVoice = directMatch;
          console.log('[Audio] Direct voice match:', directMatch.name);
        } else if (voiceMap[voiceId]) {
          for (const name of voiceMap[voiceId]) {
            const voice = voices.find(v => v.name.includes(name) && v.lang.startsWith('en'));
            if (voice) {
              selectedVoice = voice;
              console.log('[Audio] Mapped voice:', voice.name, 'for', voiceId);
              break;
            }
          }
        }
      }

      if (!selectedVoice) {
        const preferred = ['Samantha', 'Daniel', 'Serena', 'Alex', 'Karen'];
        for (const name of preferred) {
          const voice = voices.find(v => v.name.includes(name) && v.lang.startsWith('en'));
          if (voice) {
            selectedVoice = voice;
            console.log('[Audio] Fallback voice:', voice.name);
            break;
          }
        }
      }

      if (!selectedVoice) {
        selectedVoice = voices.find(v => v.lang.startsWith('en')) || null;
        console.log('[Audio] Last resort voice:', selectedVoice?.name || 'system default');
      }

      if (selectedVoice) {
        this.currentUtterance.voice = selectedVoice;
      }

      const currentState = get(audioState);
      this.currentUtterance.volume = currentState.outputVolume / 100;
      this.currentUtterance.rate = 1.0;
      this.currentUtterance.pitch = 1.0;

      this.currentUtterance.onstart = () => {
        console.log('[Audio] TTS onstart - setting isSpeaking=true');
        audioState.update(s => ({ ...s, isSpeaking: true }));
      };

      this.currentUtterance.onend = () => {
        console.log('[Audio] TTS onend - setting isSpeaking=false');
        audioState.update(s => ({ ...s, isSpeaking: false }));
      };

      this.currentUtterance.onerror = (e) => {
        console.error('[Audio] TTS error:', e);
        audioState.update(s => ({ ...s, isSpeaking: false }));
      };

      speechSynthesis.speak(this.currentUtterance);
      console.log('[Audio] Speaking:', text.substring(0, 50) + '...');
    };

    if (this.cachedVoices.length === 0) {
      this.cachedVoices = speechSynthesis.getVoices();
      if (this.cachedVoices.length === 0) {
        speechSynthesis.onvoiceschanged = () => {
          this.cachedVoices = speechSynthesis.getVoices();
          console.log('[Audio] Voices loaded:', this.cachedVoices.length);
          doSpeak();
        };
        return;
      }
    }
    doSpeak();
  }

  stopPlayback(): void {
    if (this.currentSource) {
      try {
        this.currentSource.stop();
      } catch (e) {
        // Ignore
      }
      this.currentSource = null;
    }

    // Also stop browser TTS
    if ('speechSynthesis' in window) {
      speechSynthesis.cancel();
    }

    audioState.update(s => ({ ...s, isSpeaking: false }));
  }
}

// Global instances
export const audioCaptureManager = new AudioCaptureManager();
export const audioPlaybackManager = new AudioPlaybackManager();

export function initAudioEvents(): void {
  console.log('[Audio] initAudioEvents() called');
  
  const registerEvents = () => {
    if (!window.runtime) {
      console.warn('[Audio] window.runtime not ready, retrying in 100ms...');
      setTimeout(registerEvents, 100);
      return;
    }
    
    console.log('[Audio] window.runtime available, registering events');
  
  window.runtime.EventsOn('audio:listening', (listening: boolean) => {
    console.log('[Audio] Received audio:listening:', listening);
    audioState.update(s => ({ ...s, isListening: listening }));
  });

  window.runtime.EventsOn('audio:speaking', (speaking: boolean) => {
    console.log('[Audio] Received audio:speaking:', speaking);
    audioState.update(s => ({ ...s, isSpeaking: speaking, transcript: speaking ? '🔊 SPEAKING...' : s.transcript }));
  });

  window.runtime.EventsOn('audio:transcript', (text: string) => {
    audioState.update(s => ({ ...s, transcript: text }));
  });

  window.runtime.EventsOn('cortex:response', async (text: string) => {
    console.log('[Audio] Received cortex:response:', text?.substring(0, 100));
    audioState.update(s => ({ ...s, lastResponse: text }));
  });

  window.runtime.EventsOn('audio:playback', (data: { audio: string; format: string }) => {
    if (data.audio && data.audio.length > 0) {
      audioPlaybackManager.playAudio(data.audio, data.format);
    } else {
      console.warn('[Audio] No audio data received');
    }
  });

  window.runtime.EventsOn('cortex:speak', (data: { text: string; voiceId?: string }) => {
    audioPlaybackManager.speakText(data.text, data.voiceId);
  });

  window.runtime.EventsOn('audio:stop_playback', () => {
    audioPlaybackManager.stopPlayback();
  });

  window.runtime.EventsOn('viseme:timeline', (data: { events: VisemeEvent[]; duration: number }) => {
    console.log('[Audio] Received viseme timeline:', data.events?.length, 'events, duration:', data.duration, 'ms');
    audioState.update(s => ({ ...s, visemeTimeline: data.events || [] }));
  });

  window.runtime.EventsOn('audio:thinking_start', () => {
    console.log('[Audio] Thinking started');
    audioState.update(s => ({ ...s, isThinking: true }));
  });

  window.runtime.EventsOn('audio:thinking_stop', () => {
    console.log('[Audio] Thinking stopped');
    audioState.update(s => ({ ...s, isThinking: false }));
  });

  window.runtime.EventsOn('settings:test_voice', async (data: { voiceId: string; text: string }) => {
    console.log('[Audio] Testing voice:', data.voiceId);
    if (window.go?.bridge?.AudioBridge?.SpeakText) {
      try {
        await window.go?.bridge?.SettingsBridge?.SetVoice(data.voiceId);
        await window.go.bridge.AudioBridge.SpeakText(data.text);
      } catch (err) {
        console.error('[Audio] TTS test failed:', err);
        audioPlaybackManager.speakText(data.text, data.voiceId);
      }
    } else {
      audioPlaybackManager.speakText(data.text, data.voiceId);
    }
  });

    console.log('[Audio] Event listeners registered');
  };
  
  registerEvents();
}

// Send a text message to CortexBrain
export async function sendTextMessage(text: string): Promise<string> {
  console.log('[Audio] Sending text message:', text);

  // Update transcript to show user's message
  audioState.update(s => ({ ...s, transcript: text }));

  try {
    // @ts-ignore - Wails binding
    if (window.go?.bridge?.AudioBridge?.SendMessage) {
      const response = await window.go.bridge.AudioBridge.SendMessage(text);
      console.log('[Audio] Got response:', response?.substring(0, 100));

      // Update last response
      audioState.update(s => ({ ...s, lastResponse: response }));

      // NOTE: TTS is handled by the cortex:response event handler to avoid double-speak
      // Do NOT call SpeakText here - let the event handler do it

      return response;
    } else {
      console.error('[Audio] Go bridge not available for SendMessage');
      throw new Error('Go bridge not available');
    }
  } catch (err) {
    console.error('[Audio] Failed to send message:', err);
    throw err;
  }
}

import type { TranscriptSegment, TranscriptionMode } from '@/models';
import { generateId } from '@/models';
import { getCortexClient } from './cortex';

interface WebSpeechRecognition extends EventTarget {
  continuous: boolean;
  interimResults: boolean;
  lang: string;
  maxAlternatives: number;
  start(): void;
  stop(): void;
  abort(): void;
  onerror: ((event: SpeechRecognitionErrorEvent) => void) | null;
  onresult: ((event: SpeechRecognitionEvent) => void) | null;
  onstart: (() => void) | null;
  onend: (() => void) | null;
  onspeechstart: (() => void) | null;
  onspeechend: (() => void) | null;
}

interface SpeechRecognitionErrorEvent extends Event {
  error: string;
  message: string;
}

interface SpeechRecognitionEvent extends Event {
  resultIndex: number;
  results: SpeechRecognitionResultList;
}

interface SpeechRecognitionResultList {
  length: number;
  item(index: number): SpeechRecognitionResult;
  [index: number]: SpeechRecognitionResult;
}

interface SpeechRecognitionResult {
  isFinal: boolean;
  length: number;
  item(index: number): SpeechRecognitionAlternative;
  [index: number]: SpeechRecognitionAlternative;
}

interface SpeechRecognitionAlternative {
  transcript: string;
  confidence: number;
}

type TranscriptionCallback = (segment: TranscriptSegment | null, interim: string) => void;
type ErrorCallback = (error: string) => void;

declare global {
  interface Window {
    webkitSpeechRecognition: new () => WebSpeechRecognition;
    SpeechRecognition: new () => WebSpeechRecognition;
  }
}

class TranscriptionService {
  private recognition: WebSpeechRecognition | null = null;
  private mode: TranscriptionMode = 'web_speech';
  private isListening = false;
  private currentSpeakerId = 'speaker-1';
  private currentSpeakerLabel = 'Speaker 1';
  private segmentStartTime = 0;
  private recordingStartTime = 0;
  private language = 'en-US';
  private onTranscript: TranscriptionCallback | null = null;
  private onError: ErrorCallback | null = null;
  private mediaRecorder: MediaRecorder | null = null;
  private audioChunks: Blob[] = [];
  private audioStream: MediaStream | null = null;

  isSupported(): boolean {
    return (
      typeof window !== 'undefined' &&
      ('webkitSpeechRecognition' in window || 'SpeechRecognition' in window)
    );
  }

  setMode(mode: TranscriptionMode): void {
    if (this.isListening) {
      this.stop();
    }
    this.mode = mode;
  }

  setLanguage(language: string): void {
    this.language = language;
    if (this.recognition) {
      this.recognition.lang = language;
    }
  }

  setSpeaker(speakerId: string, speakerLabel: string): void {
    this.currentSpeakerId = speakerId;
    this.currentSpeakerLabel = speakerLabel;
  }

  setCallbacks(
    onTranscript: TranscriptionCallback,
    onError: ErrorCallback
  ): void {
    this.onTranscript = onTranscript;
    this.onError = onError;
  }

  async start(): Promise<void> {
    if (this.isListening) return;

    this.recordingStartTime = Date.now();
    this.segmentStartTime = 0;

    if (this.mode === 'web_speech') {
      await this.startWebSpeech();
    } else {
      await this.startCortexSTT();
    }

    this.isListening = true;
  }

  private async startWebSpeech(): Promise<void> {
    if (!this.isSupported()) {
      throw new Error('Web Speech API is not supported in this browser');
    }

    const SpeechRecognitionClass =
      window.SpeechRecognition || window.webkitSpeechRecognition;
    this.recognition = new SpeechRecognitionClass();

    this.recognition.continuous = true;
    this.recognition.interimResults = true;
    this.recognition.lang = this.language;
    this.recognition.maxAlternatives = 1;

    this.recognition.onresult = (event: SpeechRecognitionEvent) => {
      const results = event.results;
      let interimTranscript = '';
      let finalTranscript = '';

      for (let i = event.resultIndex; i < results.length; i++) {
        const result = results[i];
        const transcript = result[0].transcript;
        const confidence = result[0].confidence;

        if (result.isFinal) {
          finalTranscript += transcript;

          if (this.onTranscript && finalTranscript.trim()) {
            const now = Date.now();
            const segment: TranscriptSegment = {
              id: generateId(),
              startTime: this.segmentStartTime,
              endTime: now - this.recordingStartTime,
              speakerId: this.currentSpeakerId,
              speakerLabel: this.currentSpeakerLabel,
              text: finalTranscript.trim(),
              confidence,
              source: 'web_speech',
              isEdited: false,
            };
            this.onTranscript(segment, '');
            this.segmentStartTime = now - this.recordingStartTime;
          }
        } else {
          interimTranscript += transcript;
        }
      }

      if (interimTranscript && this.onTranscript) {
        this.onTranscript(null, interimTranscript);
      }
    };

    this.recognition.onerror = (event: SpeechRecognitionErrorEvent) => {
      if (event.error === 'no-speech') return;
      if (event.error === 'aborted') return;

      this.onError?.(event.error || 'Speech recognition error');
    };

    this.recognition.onend = () => {
      if (this.isListening && this.recognition) {
        try {
          this.recognition.start();
        } catch {
        }
      }
    };

    this.recognition.start();
  }

  private async startCortexSTT(): Promise<void> {
    try {
      this.audioStream = await navigator.mediaDevices.getUserMedia({
        audio: {
          echoCancellation: true,
          noiseSuppression: true,
          sampleRate: 16000,
        },
      });

      this.mediaRecorder = new MediaRecorder(this.audioStream, {
        mimeType: 'audio/webm',
      });

      this.audioChunks = [];

      this.mediaRecorder.ondataavailable = (event) => {
        if (event.data.size > 0) {
          this.audioChunks.push(event.data);
        }
      };

      this.mediaRecorder.onstop = async () => {
        if (this.audioChunks.length === 0) return;

        const audioBlob = new Blob(this.audioChunks, { type: 'audio/webm' });
        this.audioChunks = [];

        try {
          const client = getCortexClient();
          const result = await client.sttTranscribe(audioBlob, {
            language: this.language,
          });

          if (result.text && this.onTranscript) {
            const segment: TranscriptSegment = {
              id: generateId(),
              startTime: this.segmentStartTime,
              endTime: result.endTime || Date.now() - this.recordingStartTime,
              speakerId: this.currentSpeakerId,
              speakerLabel: this.currentSpeakerLabel,
              text: result.text,
              confidence: result.confidence,
              source: 'cortex_stt',
              isEdited: false,
            };
            this.onTranscript(segment, '');
            this.segmentStartTime = segment.endTime;
          }
        } catch (err) {
          this.onError?.(err instanceof Error ? err.message : 'STT failed');
        }
      };

      this.mediaRecorder.start(5000);
    } catch (err) {
      throw new Error(
        `Failed to access microphone: ${err instanceof Error ? err.message : 'Unknown error'}`
      );
    }
  }

  stop(): void {
    this.isListening = false;

    if (this.recognition) {
      this.recognition.onend = null;
      this.recognition.stop();
      this.recognition = null;
    }

    if (this.mediaRecorder && this.mediaRecorder.state !== 'inactive') {
      this.mediaRecorder.stop();
    }

    if (this.audioStream) {
      this.audioStream.getTracks().forEach((track) => track.stop());
      this.audioStream = null;
    }
  }

  pause(): void {
    if (this.recognition) {
      this.recognition.stop();
    }
    if (this.mediaRecorder && this.mediaRecorder.state === 'recording') {
      this.mediaRecorder.pause();
    }
  }

  resume(): void {
    if (this.recognition) {
      this.recognition.start();
    }
    if (this.mediaRecorder && this.mediaRecorder.state === 'paused') {
      this.mediaRecorder.resume();
    }
  }

  getIsListening(): boolean {
    return this.isListening;
  }

  getMode(): TranscriptionMode {
    return this.mode;
  }
}

export const transcriptionService = new TranscriptionService();

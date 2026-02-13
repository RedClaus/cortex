import '@testing-library/jest-dom';

Object.defineProperty(window, 'matchMedia', {
  writable: true,
  value: (query: string) => ({
    matches: false,
    media: query,
    onchange: null,
    addListener: () => {},
    removeListener: () => {},
    addEventListener: () => {},
    removeEventListener: () => {},
    dispatchEvent: () => false,
  }),
});

Object.defineProperty(window, 'SpeechRecognition', {
  writable: true,
  value: undefined,
});

Object.defineProperty(window, 'webkitSpeechRecognition', {
  writable: true,
  value: undefined,
});

class MockMediaRecorder {
  static isTypeSupported = () => true;
  state = 'inactive';
  ondataavailable = null;
  onstop = null;
  start() {
    this.state = 'recording';
  }
  stop() {
    this.state = 'inactive';
  }
  pause() {
    this.state = 'paused';
  }
  resume() {
    this.state = 'recording';
  }
}

Object.defineProperty(window, 'MediaRecorder', {
  writable: true,
  value: MockMediaRecorder,
});

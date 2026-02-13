type EventCallback = (event: string, data: unknown) => void;

interface AndroidBridgeInterface {
  requestAudioFocus(): boolean;
  releaseAudioFocus(): void;
  saveFile(name: string, contentBase64: string): boolean;
  shareContent(title: string, text: string): void;
  getDeviceInfo(): string;
  showToast(message: string): void;
  vibrate(pattern: number[]): void;
  isNetworkAvailable(): boolean;
  openExternalUrl(url: string): void;
  setKeepScreenOn(keepOn: boolean): void;
}

declare global {
  interface Window {
    AndroidBridge?: AndroidBridgeInterface;
  }
}

class BridgeService {
  private eventListeners: Map<string, Set<EventCallback>> = new Map();
  private isAndroid: boolean;

  constructor() {
    this.isAndroid = typeof window !== 'undefined' && !!window.AndroidBridge;
    this.setupMessageListener();
  }

  private setupMessageListener(): void {
    if (typeof window === 'undefined') return;

    window.addEventListener('message', (event) => {
      try {
        const { type, data } = event.data;
        if (type && this.eventListeners.has(type)) {
          this.eventListeners.get(type)?.forEach((callback) => {
            callback(type, data);
          });
        }
      } catch {
      }
    });
  }

  isAndroidWebView(): boolean {
    return this.isAndroid;
  }

  requestAudioFocus(): boolean {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        return window.AndroidBridge.requestAudioFocus();
      } catch (err) {
        console.error('Failed to request audio focus:', err);
        return false;
      }
    }
    return true;
  }

  releaseAudioFocus(): void {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        window.AndroidBridge.releaseAudioFocus();
      } catch (err) {
        console.error('Failed to release audio focus:', err);
      }
    }
  }

  async saveFile(name: string, content: string): Promise<boolean> {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        const contentBase64 = btoa(unescape(encodeURIComponent(content)));
        return window.AndroidBridge.saveFile(name, contentBase64);
      } catch (err) {
        console.error('Failed to save file via bridge:', err);
        return false;
      }
    }

    try {
      const blob = new Blob([content], { type: 'text/plain' });
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = name;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      URL.revokeObjectURL(url);
      return true;
    } catch (err) {
      console.error('Failed to save file via browser:', err);
      return false;
    }
  }

  shareContent(title: string, text: string): void {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        window.AndroidBridge.shareContent(title, text);
        return;
      } catch (err) {
        console.error('Failed to share via bridge:', err);
      }
    }

    if (navigator.share) {
      navigator.share({ title, text }).catch((err) => {
        console.error('Failed to share via Web Share API:', err);
      });
    }
  }

  getDeviceInfo(): { platform: string; isAndroid: boolean } {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        const info = JSON.parse(window.AndroidBridge.getDeviceInfo());
        return { ...info, isAndroid: true };
      } catch {
      }
    }

    return {
      platform: navigator.platform,
      isAndroid: false,
    };
  }

  showToast(message: string): void {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        window.AndroidBridge.showToast(message);
        return;
      } catch {
      }
    }
  }

  vibrate(pattern: number[] = [100]): void {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        window.AndroidBridge.vibrate(pattern);
        return;
      } catch {
      }
    }

    if (navigator.vibrate) {
      navigator.vibrate(pattern);
    }
  }

  isNetworkAvailable(): boolean {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        return window.AndroidBridge.isNetworkAvailable();
      } catch {
      }
    }

    return navigator.onLine;
  }

  openExternalUrl(url: string): void {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        window.AndroidBridge.openExternalUrl(url);
        return;
      } catch {
      }
    }

    window.open(url, '_blank');
  }

  setKeepScreenOn(keepOn: boolean): void {
    if (this.isAndroid && window.AndroidBridge) {
      try {
        window.AndroidBridge.setKeepScreenOn(keepOn);
      } catch {
      }
    }
  }

  addEventListener(event: string, callback: EventCallback): () => void {
    if (!this.eventListeners.has(event)) {
      this.eventListeners.set(event, new Set());
    }
    this.eventListeners.get(event)!.add(callback);

    return () => {
      this.eventListeners.get(event)?.delete(callback);
    };
  }

  removeEventListener(event: string, callback: EventCallback): void {
    this.eventListeners.get(event)?.delete(callback);
  }
}

export const bridge = new BridgeService();

/**
 * Viseme types and utilities for lip-sync animation
 * Maps phonemes to Oculus viseme IDs and VRM blend shapes
 */

// Oculus Lipsync viseme IDs (industry standard)
export enum OculusViseme {
  sil = 0,   // silence
  PP = 1,    // p, b, m
  FF = 2,    // f, v
  TH = 3,    // th
  DD = 4,    // d, t, n
  kk = 5,    // k, g
  CH = 6,    // ch, j, sh
  SS = 7,    // s, z
  nn = 8,    // n
  RR = 9,    // r
  aa = 10,   // a
  E = 11,    // e
  ih = 12,   // i
  oh = 13,   // o
  ou = 14    // u
}

// Viseme event for timeline-based playback
export interface VisemeEvent {
  time: number;           // milliseconds from audio start
  visemeId: OculusViseme;
  weight: number;         // 0-1 intensity
  duration: number;       // milliseconds
}

// VRM blend shape weights for a single viseme
export interface VRMBlendShapeWeights {
  aa?: number;   // VRM 'aa' expression
  ih?: number;   // VRM 'ih' expression
  ou?: number;   // VRM 'ou' expression
  ee?: number;   // VRM 'ee' expression
  oh?: number;   // VRM 'oh' expression
}

// Map Oculus visemes to VRM blend shape weights
export const VISEME_TO_VRM: Record<OculusViseme, VRMBlendShapeWeights> = {
  [OculusViseme.sil]: { aa: 0, ih: 0, ou: 0, ee: 0, oh: 0 },
  [OculusViseme.PP]:  { aa: 0, ih: 0, ou: 0.3, ee: 0, oh: 0 },
  [OculusViseme.FF]:  { aa: 0, ih: 0.4, ou: 0, ee: 0, oh: 0 },
  [OculusViseme.TH]:  { aa: 0.3, ih: 0.2, ou: 0, ee: 0, oh: 0 },
  [OculusViseme.DD]:  { aa: 0.4, ih: 0, ou: 0, ee: 0, oh: 0 },
  [OculusViseme.kk]:  { aa: 0.2, ih: 0, ou: 0, ee: 0, oh: 0.3 },
  [OculusViseme.CH]:  { aa: 0, ih: 0.3, ou: 0.3, ee: 0, oh: 0.2 },
  [OculusViseme.SS]:  { aa: 0, ih: 0.5, ou: 0, ee: 0.3, oh: 0 },
  [OculusViseme.nn]:  { aa: 0.3, ih: 0, ou: 0, ee: 0, oh: 0 },
  [OculusViseme.RR]:  { aa: 0.2, ih: 0, ou: 0.2, ee: 0, oh: 0.2 },
  [OculusViseme.aa]:  { aa: 1.0, ih: 0, ou: 0, ee: 0, oh: 0 },
  [OculusViseme.E]:   { aa: 0.3, ih: 0.5, ou: 0, ee: 0.5, oh: 0 },
  [OculusViseme.ih]:  { aa: 0, ih: 0.8, ou: 0, ee: 0.6, oh: 0 },
  [OculusViseme.oh]:  { aa: 0.2, ih: 0, ou: 0.3, ee: 0, oh: 0.8 },
  [OculusViseme.ou]:  { aa: 0, ih: 0, ou: 1.0, ee: 0, oh: 0.3 }
};

// IPA phoneme to Oculus viseme mapping
export const PHONEME_TO_VISEME: Record<string, OculusViseme> = {
  // Silence
  '': OculusViseme.sil,
  ' ': OculusViseme.sil,
  
  // Bilabials (PP)
  'p': OculusViseme.PP,
  'b': OculusViseme.PP,
  'm': OculusViseme.PP,
  
  // Labiodentals (FF)
  'f': OculusViseme.FF,
  'v': OculusViseme.FF,
  
  // Dentals (TH)
  'θ': OculusViseme.TH,
  'ð': OculusViseme.TH,
  'th': OculusViseme.TH,
  
  // Alveolars (DD)
  'd': OculusViseme.DD,
  't': OculusViseme.DD,
  'n': OculusViseme.nn,
  'l': OculusViseme.DD,
  
  // Velars (kk)
  'k': OculusViseme.kk,
  'g': OculusViseme.kk,
  'ŋ': OculusViseme.kk,
  
  // Postalveolars (CH)
  'ʃ': OculusViseme.CH,
  'ʒ': OculusViseme.CH,
  'tʃ': OculusViseme.CH,
  'dʒ': OculusViseme.CH,
  'sh': OculusViseme.CH,
  'ch': OculusViseme.CH,
  'j': OculusViseme.CH,
  
  // Sibilants (SS)
  's': OculusViseme.SS,
  'z': OculusViseme.SS,
  
  // Rhotics (RR)
  'r': OculusViseme.RR,
  'ɹ': OculusViseme.RR,
  
  // Vowels
  'ɑ': OculusViseme.aa,
  'æ': OculusViseme.aa,
  'a': OculusViseme.aa,
  'ʌ': OculusViseme.aa,
  
  'ɛ': OculusViseme.E,
  'e': OculusViseme.E,
  
  'ɪ': OculusViseme.ih,
  'i': OculusViseme.ih,
  
  'ɔ': OculusViseme.oh,
  'o': OculusViseme.oh,
  'ɒ': OculusViseme.oh,
  
  'u': OculusViseme.ou,
  'ʊ': OculusViseme.ou,
  'w': OculusViseme.ou
};

// Simple English text to approximate viseme mapping
const CHAR_TO_VISEME: Record<string, OculusViseme> = {
  'a': OculusViseme.aa,
  'e': OculusViseme.E,
  'i': OculusViseme.ih,
  'o': OculusViseme.oh,
  'u': OculusViseme.ou,
  'b': OculusViseme.PP,
  'p': OculusViseme.PP,
  'm': OculusViseme.PP,
  'f': OculusViseme.FF,
  'v': OculusViseme.FF,
  'd': OculusViseme.DD,
  't': OculusViseme.DD,
  'n': OculusViseme.nn,
  'l': OculusViseme.DD,
  'k': OculusViseme.kk,
  'g': OculusViseme.kk,
  's': OculusViseme.SS,
  'z': OculusViseme.SS,
  'r': OculusViseme.RR,
  'j': OculusViseme.CH,
  'c': OculusViseme.kk,
  'h': OculusViseme.sil,
  'w': OculusViseme.ou,
  'y': OculusViseme.ih,
  'x': OculusViseme.kk,
  'q': OculusViseme.kk
};

/**
 * Parse a JSON viseme timeline into VisemeEvent array
 */
export function parseVisemeTimeline(json: string): VisemeEvent[] {
  try {
    const data = JSON.parse(json);
    if (Array.isArray(data)) {
      return data.map(item => ({
        time: item.time || item.t || 0,
        visemeId: item.visemeId ?? item.v ?? OculusViseme.sil,
        weight: item.weight ?? item.w ?? 1.0,
        duration: item.duration ?? item.d ?? 100
      }));
    }
    return [];
  } catch {
    return [];
  }
}

/**
 * Interpolate between two viseme states
 */
export function interpolateVisemes(
  from: VRMBlendShapeWeights,
  to: VRMBlendShapeWeights,
  t: number
): VRMBlendShapeWeights {
  const lerp = (a: number, b: number, t: number) => a + (b - a) * t;
  
  return {
    aa: lerp(from.aa ?? 0, to.aa ?? 0, t),
    ih: lerp(from.ih ?? 0, to.ih ?? 0, t),
    ou: lerp(from.ou ?? 0, to.ou ?? 0, t),
    ee: lerp(from.ee ?? 0, to.ee ?? 0, t),
    oh: lerp(from.oh ?? 0, to.oh ?? 0, t)
  };
}

/**
 * Get VRM blend shape weights for a viseme ID
 */
export function getVRMWeightsForViseme(visemeId: OculusViseme): VRMBlendShapeWeights {
  return VISEME_TO_VRM[visemeId] || VISEME_TO_VRM[OculusViseme.sil];
}

/**
 * Generate approximate viseme timeline from text (fallback when no TTS visemes)
 * @param text The text to convert
 * @param durationMs Total duration in milliseconds
 * @returns Array of VisemeEvents
 */
export function textToApproximateVisemes(text: string, durationMs: number): VisemeEvent[] {
  const events: VisemeEvent[] = [];
  const cleanText = text.toLowerCase().replace(/[^a-z\s]/g, '');
  const chars = cleanText.split('');
  
  if (chars.length === 0) return events;
  
  const msPerChar = durationMs / chars.length;
  let currentTime = 0;
  
  for (const char of chars) {
    const visemeId = CHAR_TO_VISEME[char] ?? OculusViseme.sil;
    
    // Don't add duplicate silence
    if (visemeId === OculusViseme.sil && events.length > 0 && 
        events[events.length - 1].visemeId === OculusViseme.sil) {
      currentTime += msPerChar;
      continue;
    }
    
    events.push({
      time: Math.round(currentTime),
      visemeId,
      weight: char === ' ' ? 0 : 0.8,
      duration: Math.round(msPerChar)
    });
    
    currentTime += msPerChar;
  }
  
  // End with silence
  events.push({
    time: Math.round(currentTime),
    visemeId: OculusViseme.sil,
    weight: 0,
    duration: 100
  });
  
  return events;
}

/**
 * Smooth a viseme timeline to reduce jitter
 */
export function smoothVisemeTimeline(events: VisemeEvent[], windowMs: number = 50): VisemeEvent[] {
  if (events.length < 3) return events;
  
  const smoothed: VisemeEvent[] = [];
  
  for (let i = 0; i < events.length; i++) {
    const current = events[i];
    
    // Skip very short visemes that cause jitter
    if (current.duration < windowMs && i > 0 && i < events.length - 1) {
      const prev = events[i - 1];
      const next = events[i + 1];
      
      // If surrounded by same viseme, skip
      if (prev.visemeId === next.visemeId) {
        continue;
      }
    }
    
    smoothed.push(current);
  }
  
  return smoothed;
}

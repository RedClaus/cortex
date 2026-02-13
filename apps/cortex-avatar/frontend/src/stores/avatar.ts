import { writable } from 'svelte/store';

export type EmotionState =
  | 'neutral'
  | 'happy'
  | 'sad'
  | 'thinking'
  | 'confused'
  | 'excited'
  | 'surprised';

// Extended from 9 to 15 visemes for better 3D lip-sync
export type MouthShape =
  // Basic shapes (original 9)
  | 'closed'  // Rest, silence
  | 'ah'      // Open vowels: AA, AE, AH, AW, AY
  | 'oh'      // Rounded vowels: AO, OW
  | 'ee'      // Spread vowels: IY, EY
  | 'fv'      // Labiodental: F, V
  | 'th'      // Dental: TH, DH
  | 'mbp'     // Bilabial: M, B, P
  | 'lnt'     // Alveolar: L, N, T, D
  | 'wq'      // Rounded: W, Q
  // Extended shapes (6 new)
  | 'oo'      // Tight round: UH, UW, OY
  | 'ih'      // Short spread: IH, EH
  | 'er'      // R-colored: ER
  | 'ch'      // Affricates: CH, JH, SH, ZH
  | 'ng'      // Velars back: NG
  | 'k';      // Velars open: K, G

export type EyeState =
  | 'open'
  | 'closed'
  | 'half'
  | 'wide'
  | 'squint';

export type LookDirection =
  | 'center'
  | 'left'
  | 'right'
  | 'up'
  | 'down';

export interface AvatarState {
  emotion: EmotionState;
  mouthShape: MouthShape;
  eyeState: EyeState;
  lookDirection: LookDirection;
  isSpeaking: boolean;
  isListening: boolean;
  isThinking: boolean;
  blinkTimer: number;
}

export const avatarState = writable<AvatarState>({
  emotion: 'neutral',
  mouthShape: 'closed',
  eyeState: 'open',
  lookDirection: 'center',
  isSpeaking: false,
  isListening: false,
  isThinking: false,
  blinkTimer: 0,
});

// Phoneme to viseme mapping for lip-sync
// Updated for extended 15-viseme set for better 3D avatar lip-sync
export const PHONEME_TO_VISEME: Record<string, MouthShape> = {
  // Silence
  'sil': 'closed',
  'sp': 'closed',

  // Open vowels (ah) - wide open jaw
  'AA': 'ah', 'AE': 'ah', 'AH': 'ah', 'AW': 'ah', 'AY': 'ah',

  // Rounded open vowels (oh) - open with lip rounding
  'AO': 'oh', 'OW': 'oh',

  // Tight rounded vowels (oo) - pursed lips
  'UH': 'oo', 'UW': 'oo', 'OY': 'oo',

  // Spread vowels (ee) - wide smile
  'IY': 'ee', 'EY': 'ee',

  // Short spread vowels (ih) - slight spread
  'IH': 'ih', 'EH': 'ih',

  // R-colored vowels (er) - slight pucker with tongue
  'ER': 'er',

  // Labial consonants (lips together)
  'M': 'mbp', 'B': 'mbp', 'P': 'mbp',

  // Labiodental (teeth on lip)
  'F': 'fv', 'V': 'fv',

  // Dental (tongue between teeth)
  'TH': 'th', 'DH': 'th',

  // Alveolar (tongue behind teeth)
  'L': 'lnt', 'N': 'lnt', 'T': 'lnt', 'D': 'lnt', 'S': 'lnt', 'Z': 'lnt',

  // Rounded consonants
  'W': 'wq', 'R': 'wq', 'Y': 'ee',

  // Affricates and fricatives (ch) - rounded with slight opening
  'CH': 'ch', 'JH': 'ch', 'SH': 'ch', 'ZH': 'ch',

  // Velar nasal (ng) - back of mouth
  'NG': 'ng',

  // Velar stops (k) - open for release
  'K': 'k', 'G': 'k',

  // Glottal
  'HH': 'ah',
};

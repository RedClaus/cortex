/**
 * Emotion types and text-based emotion detection for avatar expressions
 */

// Available emotion types
export type EmotionType = 
  | 'neutral'
  | 'happy'
  | 'sad'
  | 'angry'
  | 'surprised'
  | 'thinking'
  | 'excited'
  | 'confused';

// Emotion state with primary and optional secondary emotion
export interface EmotionState {
  primary: EmotionType;
  intensity: number;  // 0-1
  secondary?: EmotionType;
  secondaryIntensity?: number;
}

// VRM expression weights
export interface VRMExpressionWeights {
  happy?: number;
  sad?: number;
  angry?: number;
  surprised?: number;
  relaxed?: number;
  // Blendshapes
  aa?: number;
  ih?: number;
  ou?: number;
  ee?: number;
  oh?: number;
  blink?: number;
  blinkLeft?: number;
  blinkRight?: number;
  lookUp?: number;
  lookDown?: number;
  lookLeft?: number;
  lookRight?: number;
}

// Map emotion types to VRM expression weights
export const EMOTION_TO_VRM: Record<EmotionType, VRMExpressionWeights> = {
  neutral: {
    happy: 0,
    sad: 0,
    angry: 0,
    surprised: 0
  },
  happy: {
    happy: 0.7,
    aa: 0.1
  },
  sad: {
    sad: 0.6,
    lookDown: 0.2
  },
  angry: {
    angry: 0.7
  },
  surprised: {
    surprised: 0.8,
    aa: 0.2
  },
  thinking: {
    happy: 0.1,
    lookUp: 0.3
  },
  excited: {
    happy: 1.0,
    surprised: 0.3,
    aa: 0.15
  },
  confused: {
    surprised: 0.3,
    sad: 0.2,
    lookLeft: 0.2
  }
};

// Keywords for emotion detection
const EMOTION_KEYWORDS: Record<EmotionType, string[]> = {
  happy: [
    'happy', 'glad', 'great', 'awesome', 'wonderful', 'fantastic', 'excellent',
    'amazing', 'love', 'loved', 'loving', 'enjoy', 'enjoyed', 'fun', 'funny',
    'haha', 'lol', 'hehe', ':)', ':-)', 'yay', 'hooray', 'perfect', 'beautiful',
    'brilliant', 'delighted', 'pleased', 'thrilled', 'excited', 'joyful'
  ],
  sad: [
    'sad', 'sorry', 'unfortunately', 'regret', 'disappointed', 'unhappy',
    'depressed', 'down', 'upset', 'miss', 'missing', 'lost', 'grief',
    'heartbroken', 'tragic', 'terrible', ':(', ':-(', 'sigh', 'alas'
  ],
  angry: [
    'angry', 'frustrated', 'annoying', 'annoyed', 'irritated', 'furious',
    'mad', 'hate', 'hated', 'stupid', 'ridiculous', 'unacceptable',
    'outrageous', 'infuriating', 'damn', 'ugh'
  ],
  surprised: [
    'wow', 'oh', 'whoa', 'amazing', 'incredible', 'unbelievable', 'shocking',
    'surprised', 'unexpected', 'suddenly', 'really?', 'seriously?', 'no way',
    'what?!', 'omg', 'holy', 'woah'
  ],
  thinking: [
    'hmm', 'hmmm', 'let me think', 'thinking', 'consider', 'considering',
    'perhaps', 'maybe', 'possibly', 'wondering', 'pondering', 'interesting',
    'let me see', 'well...', 'actually', 'in fact'
  ],
  excited: [
    'excited', 'thrilled', 'cant wait', "can't wait", 'eager', 'pumped',
    'stoked', 'hyped', 'woohoo', 'yes!', 'finally', 'awesome!'
  ],
  confused: [
    'confused', 'confusing', 'unclear', "don't understand", 'what do you mean',
    'huh', 'wait', 'sorry?', 'pardon', 'not sure', 'uncertain', 'puzzled',
    'bewildered', 'lost'
  ],
  neutral: [] // Default, no keywords
};

/**
 * Detect emotion from text using keyword matching
 * @param text The text to analyze
 * @returns EmotionState with detected emotion and confidence
 */
export function detectEmotionFromText(text: string): EmotionState {
  const lowerText = text.toLowerCase();
  const scores: Record<EmotionType, number> = {
    neutral: 0,
    happy: 0,
    sad: 0,
    angry: 0,
    surprised: 0,
    thinking: 0,
    excited: 0,
    confused: 0
  };

  // Count keyword matches for each emotion
  for (const [emotion, keywords] of Object.entries(EMOTION_KEYWORDS)) {
    for (const keyword of keywords) {
      if (lowerText.includes(keyword)) {
        scores[emotion as EmotionType] += 1;
        
        // Bonus for exact word match (not substring)
        const regex = new RegExp(`\\b${keyword.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}\\b`, 'i');
        if (regex.test(lowerText)) {
          scores[emotion as EmotionType] += 0.5;
        }
      }
    }
  }

  // Find highest scoring emotion
  let maxScore = 0;
  let primaryEmotion: EmotionType = 'neutral';
  let secondaryEmotion: EmotionType | undefined;
  let secondaryScore = 0;

  for (const [emotion, score] of Object.entries(scores)) {
    if (score > maxScore) {
      secondaryScore = maxScore;
      secondaryEmotion = primaryEmotion;
      maxScore = score;
      primaryEmotion = emotion as EmotionType;
    } else if (score > secondaryScore) {
      secondaryScore = score;
      secondaryEmotion = emotion as EmotionType;
    }
  }

  // Calculate intensity based on keyword density
  const wordCount = text.split(/\s+/).length;
  const intensity = Math.min(1, maxScore / Math.max(3, wordCount * 0.2));

  // If no strong emotion detected, return neutral
  if (maxScore < 1) {
    return { primary: 'neutral', intensity: 0 };
  }

  const result: EmotionState = {
    primary: primaryEmotion,
    intensity: Math.max(0.3, intensity)  // Minimum intensity of 0.3 if emotion detected
  };

  // Add secondary emotion if significant
  if (secondaryEmotion && secondaryScore > 0.5 && secondaryEmotion !== 'neutral') {
    result.secondary = secondaryEmotion;
    result.secondaryIntensity = Math.min(1, secondaryScore / Math.max(3, wordCount * 0.2));
  }

  return result;
}

/**
 * Smooth transition between two emotion states
 * @param from Starting emotion state
 * @param to Target emotion state
 * @param progress Progress of transition (0-1)
 * @returns Interpolated emotion state
 */
export function smoothEmotionTransition(
  from: EmotionState,
  to: EmotionState,
  progress: number
): EmotionState {
  // Ease out cubic for natural feel
  const t = 1 - Math.pow(1 - progress, 3);
  
  return {
    primary: t < 0.5 ? from.primary : to.primary,
    intensity: from.intensity + (to.intensity - from.intensity) * t,
    secondary: t < 0.5 ? from.secondary : to.secondary,
    secondaryIntensity: from.secondaryIntensity !== undefined && to.secondaryIntensity !== undefined
      ? from.secondaryIntensity + (to.secondaryIntensity - from.secondaryIntensity) * t
      : to.secondaryIntensity
  };
}

/**
 * Get VRM expression weights for an emotion state
 * @param state The emotion state to convert
 * @returns VRM expression weights
 */
export function getVRMWeightsForEmotion(state: EmotionState): VRMExpressionWeights {
  const primaryWeights = EMOTION_TO_VRM[state.primary];
  const result: VRMExpressionWeights = {};

  // Apply primary emotion weights scaled by intensity
  for (const [key, value] of Object.entries(primaryWeights)) {
    if (value !== undefined) {
      result[key as keyof VRMExpressionWeights] = value * state.intensity;
    }
  }

  // Blend in secondary emotion if present
  if (state.secondary && state.secondaryIntensity) {
    const secondaryWeights = EMOTION_TO_VRM[state.secondary];
    for (const [key, value] of Object.entries(secondaryWeights)) {
      if (value !== undefined) {
        const currentValue = result[key as keyof VRMExpressionWeights] ?? 0;
        result[key as keyof VRMExpressionWeights] = 
          currentValue + value * state.secondaryIntensity * 0.5;  // Secondary at 50% influence
      }
    }
  }

  return result;
}

/**
 * Default neutral emotion state
 */
export const DEFAULT_EMOTION: EmotionState = {
  primary: 'neutral',
  intensity: 0
};

<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { avatarState, type MouthShape, type EyeState, type EmotionState } from '../stores/avatar';

  let animationFrame: number;
  let blinkTimer = 0;
  const BLINK_INTERVAL = 4000; // 4 seconds between blinks
  const BLINK_DURATION = 150; // 150ms blink

  // Emotion to color mapping
  const emotionColors: Record<EmotionState, string> = {
    neutral: '#4a9eff',
    happy: '#4aff4a',
    sad: '#4a4aff',
    thinking: '#ffaa4a',
    confused: '#ff4aff',
    excited: '#ff4a4a',
    surprised: '#ffff4a',
  };

  // Eye sprite mappings
  const eyeSprites: Record<EyeState, string> = {
    open: 'M 35,50 Q 50,40 65,50 Q 50,60 35,50',
    closed: 'M 35,50 Q 50,50 65,50',
    half: 'M 35,50 Q 50,45 65,50 Q 50,55 35,50',
    wide: 'M 30,50 Q 50,35 70,50 Q 50,65 30,50',
    squint: 'M 38,50 Q 50,47 62,50 Q 50,53 38,50',
  };

  // Mouth shape paths
  const mouthShapes: Record<MouthShape, string> = {
    closed: 'M 40,75 Q 50,75 60,75',
    ah: 'M 38,73 Q 50,82 62,73 Q 50,78 38,73',
    oh: 'M 42,72 Q 50,80 58,72 Q 58,78 50,80 Q 42,78 42,72',
    ee: 'M 36,75 Q 50,72 64,75 Q 50,78 36,75',
    fv: 'M 40,74 Q 50,76 60,74 L 60,76 Q 50,74 40,76 Z',
    th: 'M 40,73 Q 50,77 60,73 Q 50,80 40,73',
    mbp: 'M 42,75 Q 50,75 58,75',
    lnt: 'M 40,73 Q 50,78 60,73 Q 50,76 40,73',
    wq: 'M 44,72 Q 50,78 56,72 Q 56,78 50,80 Q 44,78 44,72',
  };

  // Animation loop
  function animate() {
    const now = Date.now();

    // Handle blinking
    if (!$avatarState.isSpeaking && !$avatarState.isThinking) {
      blinkTimer += 16; // ~60fps
      if (blinkTimer > BLINK_INTERVAL) {
        avatarState.update(s => ({ ...s, eyeState: 'closed' }));
        setTimeout(() => {
          avatarState.update(s => ({ ...s, eyeState: 'open' }));
        }, BLINK_DURATION);
        blinkTimer = 0;
      }
    }

    animationFrame = requestAnimationFrame(animate);
  }

  onMount(() => {
    animationFrame = requestAnimationFrame(animate);
  });

  onDestroy(() => {
    if (animationFrame) {
      cancelAnimationFrame(animationFrame);
    }
  });

  $: currentEyePath = eyeSprites[$avatarState.eyeState];
  $: currentMouthPath = mouthShapes[$avatarState.mouthShape];
  $: glowColor = emotionColors[$avatarState.emotion];
  $: isActive = $avatarState.isSpeaking || $avatarState.isListening || $avatarState.isThinking;
</script>

<div class="avatar-wrapper" class:active={isActive}>
  <svg viewBox="0 0 100 100" class="avatar-svg">
    <!-- Glow effect -->
    <defs>
      <filter id="glow">
        <feGaussianBlur stdDeviation="2" result="coloredBlur"/>
        <feMerge>
          <feMergeNode in="coloredBlur"/>
          <feMergeNode in="SourceGraphic"/>
        </feMerge>
      </filter>
      <radialGradient id="faceGradient" cx="50%" cy="30%" r="60%">
        <stop offset="0%" style="stop-color:#ffecd2"/>
        <stop offset="100%" style="stop-color:#fcb69f"/>
      </radialGradient>
    </defs>

    <!-- Head/Face -->
    <ellipse
      cx="50" cy="50" rx="35" ry="40"
      fill="url(#faceGradient)"
      stroke={glowColor}
      stroke-width="2"
      filter={isActive ? 'url(#glow)' : 'none'}
      class="head"
    />

    <!-- Left Eye -->
    <g transform="translate(-10, 0)">
      <path
        d={currentEyePath}
        fill="white"
        stroke="#333"
        stroke-width="1"
      />
      <circle cx="50" cy="50" r="4" fill="#333"/>
      <circle cx="52" cy="48" r="1.5" fill="white"/>
    </g>

    <!-- Right Eye -->
    <g transform="translate(10, 0)">
      <path
        d={currentEyePath}
        fill="white"
        stroke="#333"
        stroke-width="1"
      />
      <circle cx="50" cy="50" r="4" fill="#333"/>
      <circle cx="52" cy="48" r="1.5" fill="white"/>
    </g>

    <!-- Eyebrows -->
    {#if $avatarState.emotion === 'thinking'}
      <path d="M 30,40 Q 37,36 45,40" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
      <path d="M 55,40 Q 63,36 70,40" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
    {:else if $avatarState.emotion === 'surprised'}
      <path d="M 30,38 Q 37,32 45,38" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
      <path d="M 55,38 Q 63,32 70,38" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
    {:else if $avatarState.emotion === 'sad'}
      <path d="M 30,42 Q 37,38 45,40" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
      <path d="M 55,40 Q 63,38 70,42" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
    {:else}
      <path d="M 30,40 Q 37,38 45,40" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
      <path d="M 55,40 Q 63,38 70,40" fill="none" stroke="#8B4513" stroke-width="2" stroke-linecap="round"/>
    {/if}

    <!-- Nose -->
    <path d="M 50,55 L 48,62 Q 50,64 52,62 Z" fill="#e8a77c"/>

    <!-- Mouth -->
    <path
      d={currentMouthPath}
      fill={$avatarState.mouthShape === 'closed' ? 'none' : '#cc4444'}
      stroke="#aa3333"
      stroke-width="1.5"
      stroke-linecap="round"
      class="mouth"
    />

    <!-- Thinking indicator -->
    {#if $avatarState.isThinking}
      <g class="thinking-dots">
        <circle cx="75" cy="25" r="3" fill={glowColor} class="dot dot1"/>
        <circle cx="82" cy="20" r="4" fill={glowColor} class="dot dot2"/>
        <circle cx="90" cy="15" r="5" fill={glowColor} class="dot dot3"/>
      </g>
    {/if}

    <!-- Listening indicator -->
    {#if $avatarState.isListening}
      <g class="listening-waves">
        <circle cx="50" cy="95" r="5" fill="none" stroke={glowColor} stroke-width="1" class="wave wave1"/>
        <circle cx="50" cy="95" r="10" fill="none" stroke={glowColor} stroke-width="1" class="wave wave2"/>
        <circle cx="50" cy="95" r="15" fill="none" stroke={glowColor} stroke-width="1" class="wave wave3"/>
      </g>
    {/if}
  </svg>

  <!-- Status label -->
  <div class="status-label" style="color: {glowColor}">
    {#if $avatarState.isSpeaking}
      Speaking...
    {:else if $avatarState.isListening}
      Listening...
    {:else if $avatarState.isThinking}
      Thinking...
    {:else}
      Ready
    {/if}
  </div>
</div>

<style>
  .avatar-wrapper {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 10px;
  }

  .avatar-svg {
    width: 250px;
    height: 250px;
    transition: transform 0.3s ease;
  }

  .avatar-wrapper.active .avatar-svg {
    transform: scale(1.05);
  }

  .head {
    transition: stroke 0.3s ease;
  }

  .mouth {
    transition: d 0.1s ease;
  }

  .status-label {
    font-size: 14px;
    font-weight: 500;
    text-transform: uppercase;
    letter-spacing: 1px;
  }

  /* Thinking animation */
  .thinking-dots .dot {
    animation: bounce 1.4s infinite ease-in-out;
  }

  .thinking-dots .dot1 { animation-delay: 0s; }
  .thinking-dots .dot2 { animation-delay: 0.2s; }
  .thinking-dots .dot3 { animation-delay: 0.4s; }

  @keyframes bounce {
    0%, 80%, 100% { transform: translateY(0); opacity: 0.6; }
    40% { transform: translateY(-5px); opacity: 1; }
  }

  /* Listening animation */
  .listening-waves .wave {
    animation: pulse 2s infinite ease-out;
    opacity: 0;
  }

  .listening-waves .wave1 { animation-delay: 0s; }
  .listening-waves .wave2 { animation-delay: 0.5s; }
  .listening-waves .wave3 { animation-delay: 1s; }

  @keyframes pulse {
    0% { transform: scale(0.5); opacity: 0.8; }
    100% { transform: scale(1.5); opacity: 0; }
  }
</style>

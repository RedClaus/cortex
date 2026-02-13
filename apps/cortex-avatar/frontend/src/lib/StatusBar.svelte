<script lang="ts">
  import { connectionState } from '../stores/connection';
  import { audioState } from '../stores/audio';

  $: statusColor = $connectionState.isConnected ? '#4aff4a' : '#ff4a4a';
  $: statusText = $connectionState.isConnected
    ? `Connected to ${$connectionState.agentName || 'CortexBrain'}`
    : $connectionState.error || 'Disconnected';
</script>

<div class="status-bar">
  <div class="left-group">
    <div class="status-indicator" style="background: {statusColor}"></div>
    <span class="status-text">{statusText}</span>
    {#if $connectionState.isConnected && $connectionState.agentVersion}
      <span class="version">v{$connectionState.agentVersion}</span>
    {/if}
  </div>

  <div class="right-group">
    <!-- Mic status -->
    <div class="indicator-item" class:active={$audioState.micEnabled} title="Microphone">
      <svg viewBox="0 0 24 24" width="14" height="14">
        <path fill="currentColor" d="M12,2A3,3 0 0,1 15,5V11A3,3 0 0,1 12,14A3,3 0 0,1 9,11V5A3,3 0 0,1 12,2M19,11C19,14.53 16.39,17.44 13,17.93V21H11V17.93C7.61,17.44 5,14.53 5,11H7A5,5 0 0,0 12,16A5,5 0 0,0 17,11H19Z"/>
      </svg>
      {#if $audioState.vadActive}
        <span class="pulse-dot"></span>
      {/if}
    </div>

    <!-- Speaker status -->
    <div class="indicator-item" class:active={$audioState.speakerEnabled} title="Speaker">
      <svg viewBox="0 0 24 24" width="14" height="14">
        <path fill="currentColor" d="M14,3.23V5.29C16.89,6.15 19,8.83 19,12C19,15.17 16.89,17.84 14,18.7V20.77C18,19.86 21,16.28 21,12C21,7.72 18,4.14 14,3.23M16.5,12C16.5,10.23 15.5,8.71 14,7.97V16C15.5,15.29 16.5,13.76 16.5,12M3,9V15H7L12,20V4L7,9H3Z"/>
      </svg>
      {#if $audioState.isSpeaking}
        <span class="pulse-dot speaking"></span>
      {/if}
    </div>

    <!-- Camera status -->
    <div class="indicator-item" class:active={$audioState.cameraEnabled} title="Camera">
      <svg viewBox="0 0 24 24" width="14" height="14">
        <path fill="currentColor" d="M17,10.5V7A1,1 0 0,0 16,6H4A1,1 0 0,0 3,7V17A1,1 0 0,0 4,18H16A1,1 0 0,0 17,17V13.5L21,17.5V6.5L17,10.5Z"/>
      </svg>
    </div>

    <!-- Volume indicator -->
    <div class="volume-indicator" title="Volume: {$audioState.outputVolume}%">
      <div class="volume-bar" style="width: {$audioState.outputVolume}%"></div>
    </div>
  </div>
</div>

<style>
  .status-bar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 8px 16px;
    background: rgba(0, 0, 0, 0.3);
    border-radius: 0 0 12px 12px;
    font-size: 12px;
  }

  .left-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }

  .right-group {
    display: flex;
    align-items: center;
    gap: 12px;
  }

  .status-indicator {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    box-shadow: 0 0 6px currentColor;
  }

  .status-text {
    color: rgba(255, 255, 255, 0.8);
  }

  .version {
    color: rgba(255, 255, 255, 0.4);
    font-size: 10px;
  }

  .indicator-item {
    position: relative;
    display: flex;
    align-items: center;
    justify-content: center;
    color: rgba(255, 255, 255, 0.3);
    transition: color 0.2s;
  }

  .indicator-item.active {
    color: #4aff4a;
  }

  .pulse-dot {
    position: absolute;
    top: -2px;
    right: -2px;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: #4aff4a;
    animation: pulse 1s infinite;
  }

  .pulse-dot.speaking {
    background: #ff9500;
  }

  @keyframes pulse {
    0%, 100% { opacity: 1; transform: scale(1); }
    50% { opacity: 0.5; transform: scale(1.2); }
  }

  .volume-indicator {
    width: 40px;
    height: 4px;
    background: rgba(255, 255, 255, 0.2);
    border-radius: 2px;
    overflow: hidden;
  }

  .volume-bar {
    height: 100%;
    background: rgba(74, 158, 255, 0.8);
    transition: width 0.2s;
  }
</style>
